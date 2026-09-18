package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/evanxdsouza/mangrove/internal/auth"
	"github.com/evanxdsouza/mangrove/internal/executor"
	"github.com/evanxdsouza/mangrove/internal/orchestrator"
	"github.com/evanxdsouza/mangrove/internal/portregistry"
	"github.com/evanxdsouza/mangrove/internal/sysinfo"
)

type resourceBudgetResponse struct {
	MemoryAllocatedMB int     `json:"memory_allocated_mb"`
	MemoryUsedMB      float64 `json:"memory_used_mb"`
	MemoryCeilingMB   int     `json:"memory_ceiling_mb"`
	RunningContainers int     `json:"running_containers"`
	DiskTotalGB       float64 `json:"disk_total_gb"`
	DiskUsedGB        float64 `json:"disk_used_gb"`
}

func (s *Server) getResourceBudget(w http.ResponseWriter, r *http.Request) {
	// s.Orchestrator is nil in some lightweight test environments (see
	// internal/api/roles_test.go) -- ComputeResourceBudget treats a nil
	// executor.Executor as "skip the live docker-stats sum," matching the
	// original behavior here.
	var exec executor.Executor
	if s.Orchestrator != nil {
		exec = s.Orchestrator.Exec
	}
	budget, err := orchestrator.ComputeResourceBudget(r.Context(), s.Store, exec, s.MemoryCeilingMB, s.DataDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resourceBudgetResponse{
		MemoryAllocatedMB: budget.MemoryAllocatedMB,
		MemoryUsedMB:      budget.MemoryUsedMB,
		MemoryCeilingMB:   budget.MemoryCeilingMB,
		RunningContainers: budget.RunningContainers,
		DiskTotalGB:       budget.DiskTotalGB,
		DiskUsedGB:        budget.DiskUsedGB,
	})
}

// getResourceHistory backs the resource page's trend view --
// scheduler.ResourceSampler records a ResourceBudget snapshot every 5
// minutes (see resource_budget.go), this just lists them. ?hours=N narrows
// the window (default 24); the sampler keeps 30 days before pruning, so
// anything longer than that returns whatever's left.
func (s *Server) getResourceHistory(w http.ResponseWriter, r *http.Request) {
	hours := 24
	if h := r.URL.Query().Get("hours"); h != "" {
		if n, err := strconv.Atoi(h); err == nil && n > 0 {
			hours = n
		}
	}
	snapshots, err := s.Store.ListResourceUsageSnapshots(r.Context(), time.Now().Add(-time.Duration(hours)*time.Hour))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snapshots)
}

func (s *Server) listPorts(w http.ResponseWriter, r *http.Request) {
	entries, err := portregistry.List(r.Context(), s.Store.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

type reservePortRequest struct {
	Port int    `json:"port"`
	Note string `json:"note"`
}

func (s *Server) reservePort(w http.ResponseWriter, r *http.Request) {
	var req reservePortRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	userID, _ := auth.UserIDFromContext(r.Context())
	if err := portregistry.ReserveManual(r.Context(), s.Store.DB, req.Port, req.Note, &userID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) releasePort(w http.ResponseWriter, r *http.Request) {
	port, err := parseID(chi.URLParam(r, "port"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid port")
		return
	}
	if err := portregistry.Release(r.Context(), s.Store.DB, int(port)); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.Store.ListActiveSessions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (s *Server) revokeSession(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "sessionID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	if err := s.Store.DeleteSessionByID(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.audit(r.Context(), "revoke_session", "session", id, nil, "")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := s.Store.ListNodes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, nodes)
}

func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) {
	notifications, err := s.Store.ListNotifications(r.Context(), 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, notifications)
}

func (s *Server) triggerPrune(w http.ResponseWriter, r *http.Request) {
	result, err := s.Orchestrator.Exec.Prune(r.Context(), executor.PruneOptions{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) getSecretsStatus(w http.ResponseWriter, r *http.Request) {
	// Deliberately returns presence/metadata only -- never the key or any
	// decrypted value, per the plan's admin-panel scope.
	writeJSON(w, http.StatusOK, map[string]any{
		"master_key_configured": s.Secrets != nil,
	})
}

// getSystemHealth returns a snapshot of the host the whole server runs on --
// CPU, memory, swap, disk, load, uptime -- as distinct from the admin
// resource budget, which only covers Mangrove's own containers. The CPU
// figure is a short live sample (deltas across ~1s of wall time), so this
// endpoint is naturally polled rather than cached.
func (s *Server) getSystemHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, sysinfo.HostHealthSample(time.Second))
}
