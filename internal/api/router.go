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

			r.Route("/workspaces", func(r chi.Router) {
				r.Get("/", s.listWorkspaces)
				r.Post("/", s.createWorkspace)
				r.Route("/{workspaceID}", func(r chi.Router) {
					r.Delete("/", s.deleteWorkspace)
				})
			})

			r.Route("/projects", func(r chi.Router) {
				r.Get("/", s.listProjects)
				r.Post("/", s.createProject)
				r.Route("/{projectID}", func(r chi.Router) {
					r.Get("/", s.getProject)
					r.With(auth.RequireOwner).Delete("/", s.deleteProject)
					r.Post("/workspace", s.setProjectWorkspace)
					r.Get("/deployments", s.listDeployments)
					r.Post("/deployments", s.createDeployment)
					r.Get("/repo", s.getProjectRepo)
					r.Post("/repo", s.linkProjectRepo)
					r.Post("/templates/{templateKey}/install", s.installTemplate)
				})
			})

			r.Route("/deployments/{deploymentID}", func(r chi.Router) {
				r.Get("/", s.getDeployment)
				r.With(auth.RequireOwner).Delete("/", s.deleteDeployment)
				r.Get("/services", s.listServices)
				r.Get("/history", s.listDeployHistory)
				r.Post("/deploy", s.triggerDeploy)
				r.Post("/redeploy", s.redeployDeployment)
				r.Post("/scale", s.scaleDeployment)
				r.Post("/cancel", s.cancelDeployment)
				r.Post("/stop", s.stopDeployment)
				r.Post("/restart", s.restartDeployment)
				r.Post("/repo", s.setDeploymentRepo)
				r.With(auth.RequireOwner).Post("/access", s.setDeploymentAccess)
				r.With(auth.RequireOwner).Post("/build-config", s.updateDeploymentBuildConfig)
			})

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
				r.Post("/rollback", s.triggerRollback)
			})

			r.Route("/services/{serviceID}", func(r chi.Router) {
				r.Get("/", s.getService)
				r.Get("/env", s.listEnvVars)
				r.Put("/env/{key}", s.setEnvVar)
				r.Delete("/env/{key}", s.deleteEnvVar)
				r.Get("/health", s.getServiceHealth)
				r.Get("/logs/stream", s.streamServiceLogs)
				r.Post("/exec", s.execServiceCommand)
			})

			r.Route("/admin", func(r chi.Router) {
				r.Get("/resource-budget", s.getResourceBudget)
				r.Get("/system-health", s.getSystemHealth)
				r.Get("/ports", s.listPorts)
				r.Post("/ports", s.reservePort)
				r.Delete("/ports/{port}", s.releasePort)
				r.Get("/sessions", s.listSessions)
				r.Delete("/sessions/{sessionID}", s.revokeSession)
				r.Get("/nodes", s.listNodes)
				r.Get("/notifications", s.listNotifications)
				r.Post("/prune", s.triggerPrune)
				r.Get("/secrets-status", s.getSecretsStatus)

				// User management is owner-only, top to bottom -- a member
				// listing/inviting/removing other accounts is exactly the
				// kind of privilege escalation roles exist to prevent.
				r.Group(func(r chi.Router) {
					r.Use(auth.RequireOwner)
					r.Get("/users", s.listUsers)
					r.Post("/users", s.createTeamUser)
					r.Delete("/users/{userID}", s.deleteTeamUser)
				})
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

	// Dashboard SPA: mounted last so it never shadows /api, /healthz, or /webhooks.
	r.Handle("/*", webui.Handler())

	return r
}
