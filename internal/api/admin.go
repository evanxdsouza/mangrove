package api

import (
	"net/http"
	"syscall"

	"github.com/go-chi/chi/v5"

	"github.com/evanxdsouza/mangrove/internal/auth"
	"github.com/evanxdsouza/mangrove/internal/executor"
	"github.com/evanxdsouza/mangrove/internal/portregistry"
)

type resourceBudgetResponse struct {
	MemoryAllocatedMB int     `json:"memory_allocated_mb"`
	MemoryCeilingMB   int     `json:"memory_ceiling_mb"`
	RunningContainers int     `json:"running_containers"`
	DiskTotalGB       float64 `json:"disk_total_gb"`
	DiskUsedGB        float64 `json:"disk_used_gb"`
}

func (s *Server) getResourceBudget(w http.ResponseWriter, r *http.Request) {
	allocated, err := s.Store.SumConfiguredMemoryMBAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	running, err := s.Store.CountRunningContainers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := resourceBudgetResponse{
		MemoryAllocatedMB: allocated,
		MemoryCeilingMB:   s.MemoryCeilingMB,
		RunningContainers: running,
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(s.DataDir, &stat); err == nil {
		blockSize := float64(stat.Bsize)
		totalBytes := float64(stat.Blocks) * blockSize
		freeBytes := float64(stat.Bfree) * blockSize
		const gb = 1024 * 1024 * 1024
		resp.DiskTotalGB = totalBytes / gb
		resp.DiskUsedGB = (totalBytes - freeBytes) / gb
	}

	writeJSON(w, http.StatusOK, resp)
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
