package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/evanxdsouza/mangrove/internal/templates"
)

type templatePromptedEnvVar struct {
	Key      string `json:"key"`
	Label    string `json:"label,omitempty"`
	Required bool   `json:"required,omitempty"`
}

type templateDeploymentSummary struct {
	SlugSuffix        string                   `json:"slug_suffix"`
	NameSuffix        string                   `json:"name_suffix"`
	BuildStrategy     string                   `json:"build_strategy"`
	ImageRef          string                   `json:"image_ref,omitempty"`
	GitURL            string                   `json:"git_url,omitempty"`
	MemoryLimitMB     int                      `json:"memory_limit_mb"`
	CPULimitCores     float64                  `json:"cpu_limit_cores"`
	ForceInternalOnly bool                     `json:"force_internal_only"`
	PromptedEnv       []templatePromptedEnvVar `json:"prompted_env,omitempty"`
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
			buildStrategy := d.BuildStrategy
			if buildStrategy == "" {
				buildStrategy = "image"
			}
			var prompted []templatePromptedEnvVar
			for _, ev := range d.Env {
				if ev.Prompt {
					prompted = append(prompted, templatePromptedEnvVar{Key: ev.Key, Label: ev.Label, Required: ev.Required})
				}
			}
			deps = append(deps, templateDeploymentSummary{
				SlugSuffix:        d.SlugSuffix,
				NameSuffix:        d.NameSuffix,
				BuildStrategy:     buildStrategy,
				ImageRef:          d.ImageRef,
				GitURL:            d.GitURL,
				MemoryLimitMB:     d.MemoryLimitMB,
				CPULimitCores:     d.CPULimitCores,
				ForceInternalOnly: d.ForceInternalOnly,
				PromptedEnv:       prompted,
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
	Slug              string                       `json:"slug"`
	MemoryOverridesMB map[string]int               `json:"memory_overrides_mb,omitempty"`
	EnvOverrides      map[string]map[string]string `json:"env_overrides,omitempty"` // slug_suffix -> env key -> value, for the template's prompted env vars
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

	result, err := s.Orchestrator.InstallTemplate(r.Context(), projectID, templateKey, req.Slug, req.MemoryOverridesMB, req.EnvOverrides)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"result": result,
			"error":  err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusCreated, result)
}
