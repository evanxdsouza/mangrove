// Package api implements Mangrove's HTTP API: project/deployment/service
// CRUD, deploy triggers, and (from later commits) auth, the webhook
// receiver, and the embedded dashboard SPA all share this router.
package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"

	"github.com/evanxdsouza/mangrove/internal/auth"
	ghclient "github.com/evanxdsouza/mangrove/internal/github"
	"github.com/evanxdsouza/mangrove/internal/orchestrator"
	"github.com/evanxdsouza/mangrove/internal/secrets"
	"github.com/evanxdsouza/mangrove/internal/store"
	"github.com/evanxdsouza/mangrove/internal/webui"
)

type Server struct {
	Store           *store.Store
	Orchestrator    *orchestrator.Orchestrator
	Secrets         *secrets.Box
	Log             *slog.Logger
	DataDir         string // for disk-usage stats in the admin resource budget
	MemoryCeilingMB int

	// GitHub OAuth App credentials for "Deploy from GitHub" -- both empty
	// disables the feature (startGithubOAuth/githubOAuthCallback answer
	// 501). GithubOAuth/GithubRepos are always set regardless (they're
	// stateless HTTP clients); only the credentials gate the feature.
	GithubOAuthClientID     string
	GithubOAuthClientSecret string
	GithubOAuth             *ghclient.OAuthClient
	GithubRepos             *ghclient.ReposClient
	// GithubComments posts/updates the bot comment on a PR's preview
	// deployment status -- nil is valid (skipped, not an error) for the
	// same reason orchestrator.GHStatus is: it's always constructed, but
	// posting is only ever best-effort.
	GithubComments *ghclient.CommentClient

	// PublicURL, when set, overrides request-derived scheme+host detection
	// in requestBaseURL -- see config.Config.PublicURL.
	PublicURL string
}

func (s *Server) GithubOAuthEnabled() bool {
	return s.GithubOAuthClientID != "" && s.GithubOAuthClientSecret != ""
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Minute)) // generous; build/deploy calls can run long

	// Shorthand for the three per-workspace role thresholds, each paired
	// with which resource-ID URL param resolves to a workspace (see
	// workspace_resolvers.go). A global owner bypasses all of these --
	// see auth.HasWorkspaceRole.
	viewer := func(resolve func(*http.Request) (int64, error)) func(http.Handler) http.Handler {
		return auth.RequireWorkspaceRole(s.Store, "viewer", resolve)
	}
	editor := func(resolve func(*http.Request) (int64, error)) func(http.Handler) http.Handler {
		return auth.RequireWorkspaceRole(s.Store, "editor", resolve)
	}
	admin := func(resolve func(*http.Request) (int64, error)) func(http.Handler) http.Handler {
		return auth.RequireWorkspaceRole(s.Store, "admin", resolve)
	}
	projWS := s.resolveProjectWorkspace
	depWS := s.resolveDeploymentWorkspace
	svcWS := s.resolveServiceWorkspace
	domWS := s.resolveDomainWorkspace
	histWS := s.resolveDeployHistoryWorkspace
	wsWS := s.resolveWorkspaceIDParam

	// Intercepts every request Caddy forwards for a password-protected
	// deployment (see internal/proxy/caddy.go's gateHandler) before any
	// other routing happens -- those requests carry arbitrary paths (the
	// protected app's own routes), so this can't be a normal chi route,
	// it has to run ahead of route matching. Everything else passes
	// straight through untouched. See internal/api/gate.go.
	r.Use(s.gateIntercept)

	r.Route("/api", func(r chi.Router) {
		// Auth endpoints are the one part of /api that must work without an
		// existing session -- there'd be no way to ever log in otherwise.
		// setup/login are rate-limited (5 attempts per 5 minutes per IP) so
		// they can't be brute-forced; this is on day one, not bolted on later.
		r.Group(func(r chi.Router) {
			r.Use(httprate.LimitByIP(5, 5*time.Minute))
			r.Post("/auth/setup", s.authSetup)
			r.Post("/auth/login", s.authLogin)
		})
		r.Get("/auth/status", s.authStatus)
		r.Post("/auth/logout", s.authLogout)

		// Everything else -- including /auth/me, which by design needs a
		// valid session to answer "who am I" -- requires auth. There is no
		// unauthenticated path through here.
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth(s.Store))
			r.Get("/auth/me", s.authMe)
			r.Post("/auth/change-password", s.authChangePassword)

			r.Get("/templates", s.listTemplates)

			// list/create have no single resource ID to resolve a
			// workspace from (list spans every workspace; create's
			// target workspace is in the JSON body) -- both check role
			// inline (see workspaces.go). Delete and membership
			// management require admin+ in the workspace itself.
			r.Route("/workspaces", func(r chi.Router) {
				r.Get("/", s.listWorkspaces)
				r.Post("/", s.createWorkspace)
				r.Route("/{workspaceID}", func(r chi.Router) {
					r.With(admin(wsWS)).Delete("/", s.deleteWorkspace)
					r.Route("/members", func(r chi.Router) {
						r.Use(admin(wsWS))
						r.Get("/", s.listWorkspaceMembers)
						r.Post("/", s.addWorkspaceMember)
						r.Put("/{userID}", s.setWorkspaceMemberRole)
						r.Delete("/{userID}", s.removeWorkspaceMember)
					})
				})
			})

			r.Route("/projects", func(r chi.Router) {
				// list/create check role inline (list has no single
				// resource ID; create's workspace_id is in the body).
				r.Get("/", s.listProjects)
				r.Post("/", s.createProject)
				r.Route("/{projectID}", func(r chi.Router) {
					r.With(viewer(projWS)).Get("/", s.getProject)
					r.With(admin(projWS)).Delete("/", s.deleteProject)
					// setProjectWorkspace checks admin+ in *both* the
					// source and destination workspace inline, since two
					// different workspace IDs are involved.
					r.Post("/workspace", s.setProjectWorkspace)
					r.With(viewer(projWS)).Get("/deployments", s.listDeployments)
					r.With(editor(projWS)).Post("/deployments", s.createDeployment)
					r.With(viewer(projWS)).Get("/repo", s.getProjectRepo)
					r.With(editor(projWS)).Post("/repo", s.linkProjectRepo)
					r.With(viewer(projWS)).Get("/repo/webhook-instructions", s.getProjectRepoWebhookInstructions)
					r.With(editor(projWS)).Post("/repo/resync-webhook", s.resyncProjectRepoWebhook)
					r.With(viewer(projWS)).Get("/repo/webhook-events", s.listProjectRepoWebhookEvents)
					r.With(editor(projWS)).Post("/templates/{templateKey}/install", s.installTemplate)
				})
			})

			r.Route("/deployments/{deploymentID}", func(r chi.Router) {
				r.With(viewer(depWS)).Get("/", s.getDeployment)
				r.With(admin(depWS)).Delete("/", s.deleteDeployment)
				r.With(viewer(depWS)).Get("/services", s.listServices)
				r.With(viewer(depWS)).Get("/history", s.listDeployHistory)
				r.With(editor(depWS)).Post("/deploy", s.triggerDeploy)
				r.With(editor(depWS)).Post("/redeploy", s.redeployDeployment)
				r.With(editor(depWS)).Post("/scale", s.scaleDeployment)
				r.With(editor(depWS)).Post("/cancel", s.cancelDeployment)
				r.With(editor(depWS)).Post("/stop", s.stopDeployment)
				r.With(editor(depWS)).Post("/restart", s.restartDeployment)
				r.With(editor(depWS)).Post("/repo", s.setDeploymentRepo)
				r.With(admin(depWS)).Post("/access", s.setDeploymentAccess)
				r.With(viewer(depWS)).Get("/staging", s.listStagingDeployments)
				r.With(editor(depWS)).Post("/staging", s.createStagingDeployment)
				r.With(editor(depWS)).Post("/promote", s.promoteDeployment)
				r.With(viewer(depWS)).Get("/previews", s.listPreviewDeployments)
				r.With(editor(depWS)).Post("/pr-previews", s.setPRPreviews)
				r.With(viewer(depWS)).Get("/domains", s.listCustomDomains)
				r.With(admin(depWS)).Post("/domains", s.addCustomDomain)
			})

			r.Route("/domains/{domainID}", func(r chi.Router) {
				r.With(editor(domWS)).Post("/verify", s.verifyCustomDomain)
				r.With(admin(domWS)).Delete("/", s.deleteCustomDomain)
			})

			// Unchanged by workspace roles: github_pats is org_id-scoped,
			// not workspace-scoped -- a PAT can back repos across
			// multiple workspaces via project_repos.github_pat_id, so
			// there's no single workspace to resolve it against without a
			// schema change. See docs/multi-user.md.
			r.Route("/github/pats", func(r chi.Router) {
				r.Get("/", s.listGithubPATs)
				r.Post("/", s.createGithubPAT)
				r.Delete("/{patID}", s.deleteGithubPAT)
			})

			// "Deploy from GitHub": OAuth connect (start/callback are real
			// browser navigations, not fetch calls -- see startGithubOAuth),
			// then repo listing and build-strategy detection for the picker.
			r.Route("/github/oauth", func(r chi.Router) {
				r.Get("/start", s.startGithubOAuth)
				r.Get("/callback", s.githubOAuthCallback)
			})
			r.Get("/github/repos", s.listGithubRepos)
			r.Post("/github/detect", s.detectGithubRepo)

			r.Route("/deploy-history/{historyID}", func(r chi.Router) {
				r.With(editor(histWS)).Post("/rollback", s.triggerRollback)
			})

			r.Route("/services/{serviceID}", func(r chi.Router) {
				r.With(viewer(svcWS)).Get("/", s.getService)
				r.With(viewer(svcWS)).Get("/env", s.listEnvVars)
				// editor+ gets in the door; setEnvVar itself additionally
				// requires admin+ inline for is_secret:true (env.go).
				r.With(editor(svcWS)).Put("/env/{key}", s.setEnvVar)
				r.With(editor(svcWS)).Delete("/env/{key}", s.deleteEnvVar)
				r.With(viewer(svcWS)).Get("/health", s.getServiceHealth)
				r.With(viewer(svcWS)).Get("/logs/stream", s.streamServiceLogs)
				// The headline fix: exec/terminal (arbitrary command
				// execution / interactive shell in a live container) were
				// open to any authenticated user regardless of workspace.
				r.With(editor(svcWS)).Post("/exec", s.execServiceCommand)
				r.With(editor(svcWS)).Get("/terminal", s.serviceTerminal)
			})

			r.Route("/admin", func(r chi.Router) {
				r.Get("/resource-budget", s.getResourceBudget)
				r.Get("/system-health", s.getSystemHealth)
				r.Get("/nodes", s.listNodes)
				r.Get("/notifications", s.listNotifications)
				r.Get("/secrets-status", s.getSecretsStatus)

				// Owner-only, top to bottom -- each of these is either a
				// system-wide, unscoped view/action (ports, prune) or one
				// that lets the caller act on *any* user's session,
				// including the owner's own (list/revoke) -- a member
				// revoking the owner's session is a lockout vector, not a
				// legitimate "manage my own sessions" feature, so this
				// isn't scoped down to "your own sessions only", it's
				// gated the same as user management below. See
				// docs/multi-user.md.
				r.Group(func(r chi.Router) {
					r.Use(auth.RequireOwner)
					r.Get("/ports", s.listPorts)
					r.Post("/ports", s.reservePort)
					r.Delete("/ports/{port}", s.releasePort)
					r.Get("/sessions", s.listSessions)
					r.Delete("/sessions/{sessionID}", s.revokeSession)
					r.Post("/prune", s.triggerPrune)
					r.Get("/backup", s.backup)

					// User management is owner-only, top to bottom -- a
					// member listing/inviting/removing other accounts is
					// exactly the kind of privilege escalation roles exist
					// to prevent.
					r.Get("/users", s.listUsers)
					r.Post("/users", s.createTeamUser)
					r.Delete("/users/{userID}", s.deleteTeamUser)
				})
			})

			// Storage/NAS shares: mounting a drive and creating an SMB share
			// with plaintext credentials is system-level, same bar as /admin
			// -- owner-only, top to bottom.
			r.Route("/storage", func(r chi.Router) {
				r.Use(auth.RequireOwner)
				r.Get("/drives", s.listDrives)
				r.Post("/drives/{uuid}/mount", s.mountDrive)
				r.Post("/drives/{uuid}/unmount", s.unmountDrive)
				r.Get("/shares", s.listNASShares)
				r.Post("/shares", s.createNASShare)
			})
		})
	})

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// The one deliberate exception to "no unauthenticated path" -- GitHub
	// can't hold a session cookie. Every request here is verified by HMAC
	// signature instead (see githubWebhook). Not nested under /api so it
	// reads clearly as Mangrove's single always-on public endpoint per
	// plan §5, and so the /api RequireAuth group can never accidentally
	// swallow it.
	r.Post("/webhooks/github/{token}", s.githubWebhook)

	// The other deliberate exception to "no unauthenticated path": the
	// far side of a protected deployment's "continue with your Mangrove
	// account" handoff (see internal/api/gate.go). It self-checks the
	// dashboard session cookie by hand rather than sitting behind
	// RequireAuth, since it has to work for a signed-out visitor too (it's
	// how they get *to* the login form). Rate-limited the same way
	// /api/auth/login is, since it accepts a password.
	r.Get("/gate-auth", s.gateAuthHandoff)
	r.Group(func(r chi.Router) {
		r.Use(httprate.LimitByIP(5, 5*time.Minute))
		r.Post("/gate-auth", s.gateAuthLogin)
	})

	// Dashboard SPA: mounted last so it never shadows /api, /healthz, or /webhooks.
	r.Handle("/*", webui.Handler())

	return r
}
