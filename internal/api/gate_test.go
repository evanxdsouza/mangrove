package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/evanxdsouza/mangrove/internal/auth"
	"github.com/evanxdsouza/mangrove/internal/gateauth"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// seedGatedDeployment creates a project/deployment/service marked
// password-protected with the given plaintext password, returning the
// deployment id.
func seedGatedDeployment(t *testing.T, env *testEnv, password string) int64 {
	t.Helper()
	ctx := t.Context()

	proj, err := env.store.CreateProject(ctx, 1, "Gate Test", "gate-test", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	dep, err := env.store.CreateDeployment(ctx, store.CreateDeploymentParams{
		ProjectID: proj.ID, Name: "web", Slug: "gate-web", BuildStrategy: "dockerfile",
	})
	if err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}
	if _, err := env.store.CreateService(ctx, store.CreateServiceParams{
		DeploymentID: dep.ID, Name: "web", ContainerName: "mangrove-gate-web-web", InternalPort: 3000,
	}); err != nil {
		t.Fatalf("CreateService: %v", err)
	}
	hash, err := auth.HashPasswordBcrypt(password)
	if err != nil {
		t.Fatalf("HashPasswordBcrypt: %v", err)
	}
	if err := env.store.SetDeploymentAccessControl(ctx, dep.ID, true, true, hash); err != nil {
		t.Fatalf("SetDeploymentAccessControl: %v", err)
	}
	return dep.ID
}

func gatedRequest(method, path string, deploymentID int64, body *strings.Reader) *http.Request {
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, path, body)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r.Host = "myapp.example.com"
	r.Header.Set(gateauth.HeaderDeploymentID, idStr(deploymentID))
	return r
}

func TestGateInterceptPassesThroughWithoutHeader(t *testing.T) {
	env := newTestEnv(t)
	called := false
	h := env.server.gateIntercept(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if !called {
		t.Fatal("expected the next handler to run when no gate header is present")
	}
}

func TestGateShowsProtectedPageWithoutCookie(t *testing.T) {
	env := newTestEnv(t)
	depID := seedGatedDeployment(t, env, "correct horse battery staple")

	h := env.server.gateIntercept(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach the real app without a valid gate cookie")
	}))
	req := gatedRequest(http.MethodGet, "/dashboard?x=1", depID, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "This deployment is protected") {
		t.Errorf("body missing gate heading: %s", body)
	}
	if !strings.Contains(body, "myapp.example.com") {
		t.Errorf("body missing hostname: %s", body)
	}
	// No PublicURL configured in this test env, so the account option
	// should be omitted entirely rather than link somewhere broken.
	if strings.Contains(body, "Continue with Mangrove account") {
		t.Errorf("account button should be omitted when PublicURL is unset")
	}
}

func TestGatePasswordWrongThenRight(t *testing.T) {
	env := newTestEnv(t)
	depID := seedGatedDeployment(t, env, "sesame")
	h := env.server.gateIntercept(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	// Wrong password: re-renders the gate page with an error, no cookie set.
	form := url.Values{"password": {"nope"}, "returnTo": {"/dashboard"}}
	req := gatedRequest(http.MethodPost, "/__mangrove_gate__/password", depID, strings.NewReader(form.Encode()))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "Incorrect password") {
		t.Errorf("expected an incorrect-password error, got: %s", w.Body.String())
	}
	if w.Result().Cookies() != nil && len(w.Result().Cookies()) > 0 {
		t.Errorf("expected no cookie set on a failed password attempt")
	}

	// Right password: redirects to returnTo and sets the gate cookie.
	form = url.Values{"password": {"sesame"}, "returnTo": {"/dashboard"}}
	req = gatedRequest(http.MethodPost, "/__mangrove_gate__/password", depID, strings.NewReader(form.Encode()))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/dashboard" {
		t.Errorf("Location = %q, want /dashboard", loc)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != gateauth.CookieName {
		t.Fatalf("expected a single %s cookie, got %v", gateauth.CookieName, cookies)
	}

	// Replaying that cookie on a normal request gets past the gate page --
	// GateUpstreams then fails (no real container in this test), which is
	// the expected boundary: it proves the auth check passed and handling
	// moved on to resolving the real backend, without needing Docker here.
	req = gatedRequest(http.MethodGet, "/dashboard", depID, nil)
	req.AddCookie(cookies[0])
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if strings.Contains(w.Body.String(), "This deployment is protected") {
		t.Errorf("a valid cookie should not show the gate page again, got: %s", w.Body.String())
	}
	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502 (no running container to resolve to)", w.Code)
	}
}

func TestGateAuthHandoffAndCallback(t *testing.T) {
	env := newTestEnv(t)
	env.server.PublicURL = "https://dashboard.example.com"
	depID := seedGatedDeployment(t, env, "sesame")

	adminHash, err := auth.HashPassword("adminpassword123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	adminID, err := env.store.CreateUser(t.Context(), "admin@example.com", adminHash, "owner")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	_ = adminID

	gateHandler := env.server.gateIntercept(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	// Step 1: the gate page's account button, minted on the deployment's
	// own domain.
	req := gatedRequest(http.MethodGet, "/private-page", depID, nil)
	w := httptest.NewRecorder()
	gateHandler.ServeHTTP(w, req)
	body := w.Body.String()
	start := strings.Index(body, `href="`+env.server.PublicURL+"/gate-auth?token=")
	if start == -1 {
		t.Fatalf("expected an account handoff link in the gate page, got: %s", body)
	}
	linkStart := start + len(`href="`)
	linkEnd := strings.Index(body[linkStart:], `"`)
	accountURL := body[linkStart : linkStart+linkEnd]
	u, err := url.Parse(accountURL)
	if err != nil {
		t.Fatalf("parse account url: %v", err)
	}
	handoffToken := u.Query().Get("token")

	// Step 2: GET /gate-auth with no dashboard session -> a login form,
	// not an immediate redirect.
	req2 := httptest.NewRequest(http.MethodGet, "/gate-auth?token="+url.QueryEscape(handoffToken), nil)
	w2 := httptest.NewRecorder()
	env.server.gateAuthHandoff(w2, req2)
	if !strings.Contains(w2.Body.String(), "Sign in to continue") {
		t.Fatalf("expected a login form for a signed-out visitor, got: %s", w2.Body.String())
	}

	// Step 3: POST /gate-auth with the admin's credentials -> redirected to
	// the deployment's own callback URL, carrying a fresh callback token.
	form := url.Values{"token": {handoffToken}, "email": {"admin@example.com"}, "password": {"adminpassword123"}}
	req3 := httptest.NewRequest(http.MethodPost, "/gate-auth", strings.NewReader(form.Encode()))
	req3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w3 := httptest.NewRecorder()
	env.server.gateAuthLogin(w3, req3)
	if w3.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302, body: %s", w3.Code, w3.Body.String())
	}
	callbackURL := w3.Header().Get("Location")
	if !strings.HasPrefix(callbackURL, "https://myapp.example.com/__mangrove_gate__/callback?token=") {
		t.Fatalf("Location = %q, want a callback URL on the deployment's own domain", callbackURL)
	}

	// Step 4: redeeming that callback token on the deployment's own domain
	// sets the gate cookie and redirects to the original page.
	cbURL, err := url.Parse(callbackURL)
	if err != nil {
		t.Fatalf("parse callback url: %v", err)
	}
	req4 := gatedRequest(http.MethodGet, cbURL.Path+"?"+cbURL.RawQuery, depID, nil)
	w4 := httptest.NewRecorder()
	gateHandler.ServeHTTP(w4, req4)
	if w4.Code != http.StatusFound {
		t.Fatalf("callback status = %d, want 302, body: %s", w4.Code, w4.Body.String())
	}
	if loc := w4.Header().Get("Location"); loc != "/private-page" {
		t.Errorf("callback Location = %q, want /private-page", loc)
	}
	cookies := w4.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != gateauth.CookieName {
		t.Fatalf("expected the gate cookie to be set by the callback, got %v", cookies)
	}
}
