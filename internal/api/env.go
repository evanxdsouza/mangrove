package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/evanxdsouza/mangrove/internal/auth"
)

type envVarResponse struct {
	Key      string `json:"key"`
	IsSecret bool   `json:"is_secret"`
	Value    string `json:"value,omitempty"` // omitted for secrets -- never round-tripped back out
}

func (s *Server) listEnvVars(w http.ResponseWriter, r *http.Request) {
	serviceID, err := parseID(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid service id")
		return
	}
	rows, err := s.Store.ListEnvVarRows(r.Context(), serviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]envVarResponse, 0, len(rows))
	for _, row := range rows {
		ev := envVarResponse{Key: row.KeyName, IsSecret: row.IsSecret}
		if !row.IsSecret {
			ev.Value = row.ValuePlain.String
		}
		out = append(out, ev)
	}
	writeJSON(w, http.StatusOK, out)
}

type setEnvVarRequest struct {
	Value    string `json:"value"`
	IsSecret bool   `json:"is_secret"`
}

func (s *Server) setEnvVar(w http.ResponseWriter, r *http.Request) {
	serviceID, err := parseID(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid service id")
		return
	}
	key := chi.URLParam(r, "key")
	var req setEnvVarRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if !req.IsSecret {
		if err := s.Store.CreatePlainEnvVar(r.Context(), serviceID, key, req.Value); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Setting a secret is the only place a member could otherwise stash
	// (and, since they already know it, effectively "read") a credential
	// through this endpoint -- owner-only, same tier as viewing generated
	// template credentials.
	if role, _ := auth.RoleFromContext(r.Context()); role != "owner" {
		writeError(w, http.StatusForbidden, "owner role required to set secret env vars")
		return
	}

	aad := []byte("env_vars:" + strconv.FormatInt(serviceID, 10) + ":" + key)
	ciphertext, nonce, err := s.Secrets.Seal(aad, []byte(req.Value))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encrypt: "+err.Error())
		return
	}
	if err := s.Store.CreateSecretEnvVar(r.Context(), serviceID, key, ciphertext, nonce); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteEnvVar(w http.ResponseWriter, r *http.Request) {
	serviceID, err := parseID(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid service id")
		return
	}
	key := chi.URLParam(r, "key")
	if err := s.Store.DeleteEnvVar(r.Context(), serviceID, key); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
