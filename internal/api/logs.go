package api

import (
	"bufio"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/evanxdsouza/mangrove/internal/executor"
	"github.com/evanxdsouza/mangrove/internal/store"
)

func (s *Server) getServiceHealth(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid service id")
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	checks, err := s.Store.ListRecentHealthChecks(r.Context(), id, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, checks)
}

// streamServiceLogs tails a service's live container output as
// Server-Sent Events, the "even a simple rolling log tail" differentiator
// called out in the plan -- streams as the process emits lines rather than
// buffering the whole history before responding.
func (s *Server) streamServiceLogs(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid service id")
		return
	}
	svc, err := s.Store.GetService(r.Context(), id)
	if err == store.ErrNotFound {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if svc.ContainerIDCurrent == "" {
		writeError(w, http.StatusConflict, "service has no running container")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "200"
	}
	// follow=false bounds the response to the requested tail and lets it
	// end on its own (`docker logs` without `-f` exits once it's printed
	// history) instead of streaming forever -- what apiclient.Client.Logs
	// uses for a one-shot snapshot (e.g. an MCP tool call, which is
	// request/response and can't consume an indefinite stream). The
	// dashboard's live-tailing LogViewer never sets this, so it keeps
	// defaulting to true.
	follow := true
	if v := r.URL.Query().Get("follow"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			follow = b
		}
	}
	logs, err := s.Orchestrator.Exec.Logs(r.Context(), svc.ContainerIDCurrent, executor.LogOptions{Follow: follow, Tail: tail})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "attach to logs: "+err.Error())
		return
	}
	defer logs.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	scanner := bufio.NewScanner(logs)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		select {
		case <-r.Context().Done():
			return
		default:
		}
		fmt.Fprintf(w, "data: %s\n\n", sseEscape(scanner.Text()))
		flusher.Flush()
	}
}

// sseEscape keeps a log line that happens to contain a literal newline from
// breaking the SSE "data:" framing.
func sseEscape(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, ' ')
			continue
		}
		out = append(out, s[i])
	}
	return string(out)
}
