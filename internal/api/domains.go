package api

import (
	"net/http"

	"github.com/evanxdsouza/mangrove/internal/store"
	"github.com/go-chi/chi/v5"
)

func (s *Server) listCustomDomains(w http.ResponseWriter, r *http.Request) {
	deploymentID, err := parseID(chi.URLParam(r, "deploymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment id")
		return
	}
	domains, err := s.Store.ListCustomDomainsForDeployment(r.Context(), deploymentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, domains)
}

type addCustomDomainRequest struct {
	Hostname string `json:"hostname"`
}

func (s *Server) addCustomDomain(w http.ResponseWriter, r *http.Request) {
	deploymentID, err := parseID(chi.URLParam(r, "deploymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment id")
		return
	}
	var req addCustomDomainRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	domain, err := s.Orchestrator.AddCustomDomain(r.Context(), deploymentID, req.Hostname)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, domain)
}

func (s *Server) verifyCustomDomain(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "domainID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain id")
		return
	}
	domain, err := s.Orchestrator.VerifyCustomDomain(r.Context(), id)
	if err == store.ErrNotFound {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, domain)
}

func (s *Server) deleteCustomDomain(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "domainID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain id")
		return
	}
	if err := s.Orchestrator.RemoveCustomDomain(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
