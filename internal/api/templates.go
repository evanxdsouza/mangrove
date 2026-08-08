package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/evanxdsouza/mangrove/internal/templates"
)

type templateDeploymentSummary struct {
	SlugSuffix        string  `json:"slug_suffix"`
	NameSuffix        string  `json:"name_suffix"`
	ImageRef          string  `json:"image_ref"`
	MemoryLimitMB     int     `json:"memory_limit_mb"`
	CPULimitCores     float64 `json:"cpu_limit_cores"`
	ForceInternalOnly bool    `json:"force_internal_only"`
}

type templateSummary struct {
	Key           string                      `json:"key"`
	Name          string                      `json:"name"`
	Description   string                      `json:"description"`
	Category      string                      `json:"category"`
	TotalMemoryMB int                         `json:"total_memory_mb"`
	Deployments   []templateDeploymentSummary `json:"deployments"`
}

// listTemplates returns every built-in template for the gallery. There's
// no DB round-trip here -- see internal/templates, which loads these from
// JSON embedded in the binary at build time, not from any table.
func (s *Server) listTemplates(w http.ResponseWriter, r *http.Request) {
	all := templates.List()
	out := make([]templateSummary, 0, len(all))
	for _, t := range all {
		deps := make([]templateDeploymentSummary, 0, len(t.Deployments))
		for _, d := range t.Deployments {
			deps = append(deps, templateDeploymentSummary{
				SlugSuffix:        d.SlugSuffix,
				NameSuffix:        d.NameSuffix,
				ImageRef:          d.ImageRef,
				MemoryLimitMB:     d.MemoryLimitMB,
				CPULimitCores:     d.CPULimitCores,
				ForceInternalOnly: d.ForceInternalOnly,
			})
		}
		out = append(out, templateSummary{
			Key: t.Key, Name: t.Name, Description: t.Description, Category: t.Category,
			TotalMemoryMB: t.TotalMemoryMB(), Deployments: deps,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

type installTemplateRequest struct {
	Slug              string         `json:"slug"`
	MemoryOverridesMB map[string]int `json:"memory_overrides_mb,omitempty"`
}

// installTemplate expands a template into real deployment/service/volume/
// env rows and deploys each one, through the exact same admission control
// and port registry as a hand-created deployment -- see
// orchestrator.InstallTemplate. On partial failure (rows created for a
// dependency, which deployed fine, but the primary's Deploy() then failed)
// the result still reports what succeeded, with a non-2xx status and the
// error alongside it, rather than silently discarding partial progress.
func (s *Server) installTemplate(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseID(chi.URLParam(r, "projectID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	templateKey := chi.URLParam(r, "templateKey")
	if _, ok := templates.Get(templateKey); !ok {
		writeError(w, http.StatusNotFound, "unknown template")
		return
	}

	var req installTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Slug == "" {
		writeError(w, http.StatusBadRequest, "slug is required")
		return
	}

	result, err := s.Orchestrator.InstallTemplate(r.Context(), projectID, templateKey, req.Slug, req.MemoryOverridesMB)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"result": result,
			"error":  err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusCreated, result)
}
