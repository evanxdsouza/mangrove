// gate.go implements the "protected deployment" gate: the styled page a
// visitor to a password-protected deployment sees in place of the real
// app, offering either "continue with your Mangrove account" (a
// cross-domain sign-in handoff) or the deployment's own password. See
// docs/protected-deployments.md for the full flow and internal/gateauth
// for the signed tokens that hold it together, and
// internal/proxy/caddy.go's gateHandler for how traffic ends up here in
// the first place.
//
// The same loopback handler also serves a sleep-enabled deployment's
// idle-wake flow (independent of password protection, and composable with
// it -- see handleGatedRequest below): proxyOrWake is the single point
// every already-authorized request passes through, and it's where a
// sleeping deployment gets intercepted with the "waking up" page instead of
// being proxied through for real. See internal/orchestrator/sleep.go and
// internal/scheduler/sleeper.go for the other halves.
package api

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/httprate"
	"golang.org/x/crypto/bcrypt"

	"github.com/evanxdsouza/mangrove/internal/auth"
	"github.com/evanxdsouza/mangrove/internal/gateauth"
	"github.com/evanxdsouza/mangrove/internal/gatepaths"
	"github.com/evanxdsouza/mangrove/internal/models"
)

const gateCallbackPath = "/__mangrove_gate__/callback"
const gatePasswordPath = "/__mangrove_gate__/password"

// gatePasswordLimiter rate-limits password guesses the same way
// /api/auth/login is limited (see router.go) -- this endpoint never goes
// through chi's normal route middleware (gateIntercept handles it directly,
// before routing), so it's applied by hand at the call site instead.
var gatePasswordLimiter = httprate.LimitByIP(10, 5*time.Minute)

// gateIntercept is mounted globally, ahead of every other route. Requests
// carrying the gate header are handled entirely here and never reach
// /api, /healthz, or the dashboard SPA -- everything else passes straight
// through untouched.
func (s *Server) gateIntercept(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idStr := r.Header.Get(gateauth.HeaderDeploymentID)
		if idStr == "" {
			next.ServeHTTP(w, r)
			return
		}
		// This header is only ever set by Caddy's gateHandler, which
		// forcibly overwrites any client-supplied value of the same name --
		// and Mangrove's own API port is loopback-only, unreachable except
		// through that route -- so it can be trusted here.
		deploymentID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "bad gate request", http.StatusBadRequest)
			return
		}
		s.handleGatedRequest(w, r, deploymentID)
	})
}

func (s *Server) handleGatedRequest(w http.ResponseWriter, r *http.Request, deploymentID int64) {
	switch {
	case r.URL.Path == gatePasswordPath && r.Method == http.MethodPost:
		gatePasswordLimiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s.gateHandlePassword(w, r, deploymentID)
		})).ServeHTTP(w, r)
	case r.URL.Path == gateCallbackPath:
		s.gateHandleCallback(w, r, deploymentID)
	default:
		dep, err := s.Store.GetDeployment(r.Context(), deploymentID)
		if err != nil {
			http.Error(w, "this deployment is temporarily unavailable", http.StatusBadGateway)
			return
		}
		if dep.PasswordProtected {
			cookie, err := r.Cookie(gateauth.CookieName)
			authed := err == nil && gateauth.NewSigner(s.Secrets).VerifyCookie(cookie.Value, deploymentID)
			// A path the owner marked public skips the gate -- checked only
			// after the cookie, so a signed-in visitor is unaffected.
			if !authed && !gatepaths.Match(dep.PublicPaths, r.URL.Path) {
				raw := r.URL.Path
				if r.URL.RawQuery != "" {
					raw += "?" + r.URL.RawQuery
				}
				s.gateRenderPage(w, r, deploymentID, sanitizeReturnTo(raw), "")
				return
			}
		}
		// Past the password gate (or there wasn't one) -- a sleep-enabled
		// deployment still gets a chance to intercept with its own "waking
		// up" page before any real proxying happens. A password-protected +
		// sleep-enabled deployment deliberately checks the password first:
		// triggering a wake is itself a (mild) cost, and this keeps an
		// unauthenticated visitor from being able to repeatedly wake someone
		// else's gated deployment.
		s.proxyOrWake(w, r, dep)
	}
}

// sleepWakeLocks serializes concurrent wake attempts for the same sleeping
// deployment -- several visitors can land here in the same moment, and only
// one of them should actually trigger Orchestrator.WakeDeployment; the rest
// just get the same "waking up" page.
var sleepWakeLocks sync.Map // map[int64]*sync.Mutex

func wakeLockFor(deploymentID int64) *sync.Mutex {
	v, _ := sleepWakeLocks.LoadOrStore(deploymentID, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// proxyOrWake is the single choke point every already-authorized request to
// a gated route passes through on its way to the real app. A sleep-enabled
// deployment gets its traffic bumped here (throttled, so a busy awake
// deployment isn't hammering the DB on every request) and, while actually
// asleep, gets intercepted with the waking-up page instead of proxied --
// triggering a background wake the first time, not on every repeated
// request/reload while it's still booting.
func (s *Server) proxyOrWake(w http.ResponseWriter, r *http.Request, dep models.Deployment) {
	if dep.SleepEnabled {
		s.touchDeploymentTraffic(dep.ID)
		if dep.Status == "sleeping" {
			s.triggerWake(dep.ID)
			s.renderWakingPage(w, r, dep)
			return
		}
	}
	s.gateProxyThrough(w, r, dep.ID)
}

// lastTouchWrites throttles TouchDeploymentLastRequest so a busy awake
// deployment doesn't write to the DB on every single request -- just often
// enough (once a minute) that the sleeper's idle check stays accurate. Not
// persisted: a restart resets it, which only costs one extra write per
// previously-touched deployment, not a correctness problem.
var lastTouchWrites sync.Map // map[int64]time.Time

func (s *Server) touchDeploymentTraffic(deploymentID int64) {
	now := time.Now()
	if v, ok := lastTouchWrites.Load(deploymentID); ok {
		if now.Sub(v.(time.Time)) < time.Minute {
			return
		}
	}
	lastTouchWrites.Store(deploymentID, now)
	if err := s.Store.TouchDeploymentLastRequest(context.Background(), deploymentID); err != nil {
		s.Log.Warn("wake: touch last request failed", "deployment_id", deploymentID, "error", err)
	}
}

// triggerWake kicks off a background wake for a sleeping deployment, deduped
// so concurrent visitors don't all call WakeDeployment at once. Detached
// from the request context (like orchestrator.WithInflightDeploy's own
// deploys) since the wake must finish even if the visitor who triggered it
// navigates away or their connection drops.
func (s *Server) triggerWake(deploymentID int64) {
	lock := wakeLockFor(deploymentID)
	if !lock.TryLock() {
		return // already waking
	}
	go func() {
		defer lock.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := s.Orchestrator.WakeDeployment(ctx, deploymentID); err != nil {
			s.Log.Warn("wake: restart deployment failed", "deployment_id", deploymentID, "error", err)
		}
	}()
}

// sanitizeReturnTo defends against an open redirect within the gate's own
// forms/cookies: only a plain same-origin path (optionally with a query
// string) is ever accepted, never an absolute URL or a protocol-relative
// one -- the origin for any cross-domain hop is always computed by this
// server from trusted data (see gateRenderPage/gateAuthHandoff), never
// taken from client input.
func sanitizeReturnTo(v string) string {
	if v == "" || !strings.HasPrefix(v, "/") || strings.HasPrefix(v, "//") || strings.Contains(v, "://") {
		return "/"
	}
	return v
}

func setGateCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     gateauth.CookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int((24 * time.Hour).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Secure is intentionally not forced here, mirroring startSession
		// in auth.go -- Caddy terminates TLS in front of this box, and
		// hard-requiring it on the cookie would break loopback/dev setups.
	})
}

// gateRenderPage renders the "this deployment is protected" page.
// r.Host/r.URL are trustworthy here as the basis for the account-handoff
// return destination: this handler only ever runs for requests Caddy has
// forwarded with the gate header for *this specific deployment's own
// configured route* -- an external client cannot get Caddy to set that
// header for a host of their choosing. Contrast gateAuthHandoff below,
// which receives its return destination from a public, client-suppliable
// query parameter and must not trust it directly.
func (s *Server) gateRenderPage(w http.ResponseWriter, r *http.Request, deploymentID int64, returnTo, errMsg string) {
	if returnTo == "" {
		returnTo = "/"
	}
	data := gatePageData{
		Hostname: r.Host,
		ReturnTo: returnTo,
		ErrMsg:   errMsg,
	}
	if s.PublicURL != "" {
		token, err := gateauth.NewSigner(s.Secrets).IssueHandoff(deploymentID, "https://"+r.Host+returnTo)
		if err != nil {
			s.Log.Error("gate: issue handoff token failed", "deployment_id", deploymentID, "error", err)
		} else {
			data.AccountURL = s.PublicURL + "/gate-auth?token=" + url.QueryEscape(token)
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := gateTpl.Execute(w, data); err != nil {
		s.Log.Error("gate: render page failed", "error", err)
	}
}

// renderWakingPage serves the "this deployment is waking up" page in place
// of the real app, with a meta-refresh so the visitor's browser reloads the
// same URL every few seconds. On each reload, proxyOrWake re-checks the
// deployment's current status fresh from the DB -- once triggerWake's
// background WakeDeployment finishes, status flips to "running" and the
// next reload proxies through for real, no separate status-polling endpoint
// needed.
func (s *Server) renderWakingPage(w http.ResponseWriter, r *http.Request, dep models.Deployment) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Retry-After", "3")
	if err := sleepTpl.Execute(w, sleepPageData{Hostname: r.Host, Name: dep.Name}); err != nil {
		s.Log.Error("wake: render waking page failed", "error", err)
	}
}

func (s *Server) gateHandlePassword(w http.ResponseWriter, r *http.Request, deploymentID int64) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	returnTo := sanitizeReturnTo(r.FormValue("returnTo"))
	password := r.FormValue("password")

	hash, err := s.Store.GetDeploymentPasswordHash(r.Context(), deploymentID)
	if err != nil || hash == "" || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		s.gateRenderPage(w, r, deploymentID, returnTo, "Incorrect password.")
		return
	}

	token, err := gateauth.NewSigner(s.Secrets).IssueCookie(deploymentID)
	if err != nil {
		s.Log.Error("gate: issue cookie failed", "deployment_id", deploymentID, "error", err)
		s.gateRenderPage(w, r, deploymentID, returnTo, "Something went wrong, please try again.")
		return
	}
	setGateCookie(w, token)
	http.Redirect(w, r, returnTo, http.StatusFound)
}

func (s *Server) gateHandleCallback(w http.ResponseWriter, r *http.Request, deploymentID int64) {
	invalid := func() {
		s.gateRenderPage(w, r, deploymentID, "/", "This sign-in link is invalid or has expired. Please try again.")
	}

	returnURL, err := gateauth.NewSigner(s.Secrets).OpenCallback(r.URL.Query().Get("token"), deploymentID)
	if err != nil {
		invalid()
		return
	}
	u, err := url.Parse(returnURL)
	if err != nil {
		invalid()
		return
	}

	token, err := gateauth.NewSigner(s.Secrets).IssueCookie(deploymentID)
	if err != nil {
		s.Log.Error("gate: issue cookie failed", "deployment_id", deploymentID, "error", err)
		invalid()
		return
	}
	setGateCookie(w, token)
	http.Redirect(w, r, u.RequestURI(), http.StatusFound)
}

// gateProxyThrough forwards an already-let-through request on to the real
// app -- either a running container or, for a static-strategy deployment,
// its build output directory. Resolved fresh on every request (no
// caching): fine for what this is, a low-traffic staging gate, not meant
// for a high-throughput production path -- see docs/protected-deployments.md.
func (s *Server) gateProxyThrough(w http.ResponseWriter, r *http.Request, deploymentID int64) {
	upstreams, staticRoot, err := s.Orchestrator.GateUpstreams(r.Context(), deploymentID)
	if err != nil {
		s.Log.Warn("gate: resolve upstream failed", "deployment_id", deploymentID, "error", err)
		http.Error(w, "this deployment is temporarily unavailable", http.StatusBadGateway)
		return
	}

	if staticRoot != "" {
		http.FileServer(http.Dir(staticRoot)).ServeHTTP(w, r)
		return
	}

	upstream := upstreams[0]
	if len(upstreams) > 1 {
		// A request-scoped pick across replicas, not sticky -- good enough
		// load spreading for a gated route without needing real state.
		upstream = upstreams[rand.Intn(len(upstreams))]
	}
	target := &url.URL{Scheme: "http", Host: upstream}
	proxy := httputil.NewSingleHostReverseProxy(target)
	baseDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		baseDirector(req)
		// Nest's edge terminates HTTPS for visitors but forwards plain HTTP
		// with no indication of the original scheme -- mirrors the same
		// header Caddy itself sets on an ungated route (see caddy.go's
		// PutRouteMulti) so backend apps don't misbuild absolute URLs.
		req.Header.Set("X-Forwarded-Proto", "https")
	}
	proxy.ServeHTTP(w, r)
}

// gateAuthHandoff is the first stop on the dashboard's own domain for
// "continue with your Mangrove account": it only trusts what's inside the
// signed handoff token, never a raw query parameter, since (unlike
// gateRenderPage above) this is a public, unauthenticated GET that anyone
// can link to directly.
func (s *Server) gateAuthHandoff(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	deploymentID, returnURL, err := gateauth.NewSigner(s.Secrets).OpenHandoff(token)
	if err != nil {
		s.renderGateAuthError(w)
		return
	}

	if cookie, err := r.Cookie(auth.SessionCookieName); err == nil {
		if _, _, err := auth.ValidateSession(r.Context(), s.Store, cookie.Value); err == nil {
			s.completeGateHandoff(w, r, deploymentID, returnURL)
			return
		}
	}
	s.renderGateAuthLogin(w, token, "")
}

func (s *Server) gateAuthLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	token := r.FormValue("token")
	deploymentID, returnURL, err := gateauth.NewSigner(s.Secrets).OpenHandoff(token)
	if err != nil {
		s.renderGateAuthError(w)
		return
	}

	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	user, err := s.Store.GetUserByEmail(r.Context(), email)
	if err != nil {
		// Same response as a wrong password -- don't leak which part was
		// wrong, mirroring authLogin in auth.go.
		s.renderGateAuthLogin(w, token, "Invalid email or password.")
		return
	}
	if ok, err := auth.VerifyPassword(r.FormValue("password"), user.PasswordHash); err != nil || !ok {
		s.renderGateAuthLogin(w, token, "Invalid email or password.")
		return
	}

	s.Store.TouchUserLogin(r.Context(), user.ID)
	s.startSession(w, r, user.ID)
	s.completeGateHandoff(w, r, deploymentID, returnURL)
}

// completeGateHandoff mints the second, deployment-scoped token and sends
// the visitor's browser back to the protected deployment's own domain to
// redeem it. The scheme+host it redirects to comes only from returnURL,
// which itself only ever came from inside a signed token this server
// issued (gateRenderPage) -- never from unvalidated client input.
func (s *Server) completeGateHandoff(w http.ResponseWriter, r *http.Request, deploymentID int64, returnURL string) {
	u, err := url.Parse(returnURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		s.renderGateAuthError(w)
		return
	}
	cbToken, err := gateauth.NewSigner(s.Secrets).IssueCallback(deploymentID, returnURL)
	if err != nil {
		s.Log.Error("gate: issue callback token failed", "deployment_id", deploymentID, "error", err)
		s.renderGateAuthError(w)
		return
	}
	callbackURL := fmt.Sprintf("%s://%s%s?token=%s", u.Scheme, u.Host, gateCallbackPath, url.QueryEscape(cbToken))
	http.Redirect(w, r, callbackURL, http.StatusFound)
}

func (s *Server) renderGateAuthError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	if err := gateAuthTpl.Execute(w, gateAuthPageData{IsError: true, ErrMsg: "This link is invalid or has expired. Go back to the deployment and try again."}); err != nil {
		s.Log.Error("gate: render gate-auth error page failed", "error", err)
	}
}

func (s *Server) renderGateAuthLogin(w http.ResponseWriter, token, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := gateAuthTpl.Execute(w, gateAuthPageData{Token: token, ErrMsg: errMsg}); err != nil {
		s.Log.Error("gate: render gate-auth login page failed", "error", err)
	}
}
