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
	"github.com/evanxdsouza/mangrove/internal/orchestrator"
	"github.com/evanxdsouza/mangrove/internal/secrets"
	"github.com/evanxdsouza/mangrove/internal/store"
)

type Server struct {
	Store        *store.Store
	Orchestrator *orchestrator.Orchestrator
	Secrets      *secrets.Box
	Log          *slog.Logger
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
		r.Get("/auth/me", s.authMe)

		// Everything else is a write-capable resource endpoint and requires
		// a valid session -- there is no unauthenticated path through here.
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth(s.Store))

			r.Route("/projects", func(r chi.Router) {
				r.Get("/", s.listProjects)
				r.Post("/", s.createProject)
				r.Route("/{projectID}", func(r chi.Router) {
					r.Get("/", s.getProject)
					r.Delete("/", s.deleteProject)
					r.Get("/deployments", s.listDeployments)
					r.Post("/deployments", s.createDeployment)
				})
			})

			r.Route("/deployments/{deploymentID}", func(r chi.Router) {
				r.Get("/", s.getDeployment)
				r.Get("/services", s.listServices)
				r.Get("/history", s.listDeployHistory)
				r.Post("/deploy", s.triggerDeploy)
			})

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
			})
		})
	})

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	return r
}
