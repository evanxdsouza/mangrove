package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/evanxdsouza/mangrove/internal/orchestrator"
	"github.com/evanxdsouza/mangrove/internal/store"
)

func (s *Server) listDeployments(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseID(chi.URLParam(r, "projectID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	deployments, err := s.Store.ListDeployments(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, deployments)
}

type serviceSpec struct {
	Name                 string  `json:"name"`
	InternalPort         int     `json:"internal_port"`
	IsInternalOnly       bool    `json:"is_internal_only"`
	CPULimitCores        float64 `json:"cpu_limit_cores"`
	MemoryLimitMB        int     `json:"memory_limit_mb"`
	HealthCheckPath      string  `json:"health_check_path"`
	HealthCheckIntervalS int     `json:"health_check_interval_s"`
	HealthCheckTimeoutS  int     `json:"health_check_timeout_s"`
}

type createDeploymentRequest struct {
	Name                string      `json:"name"`
	Slug                string      `json:"slug"`
	BuildStrategy       string      `json:"build_strategy"` // dockerfile|nixpacks|compose|image|static
	GitBranch           string      `json:"git_branch"`
	ImageRef            string      `json:"image_ref"`
	RootPath            string      `json:"root_path"`
	DockerfilePath      string      `json:"dockerfile_path"`
	ComposePath         string      `json:"compose_path"`
	StaticBuildCommand  string      `json:"static_build_command"` // strategy == static; optional, omit if the repo is already pre-built
	StaticOutputDir     string      `json:"static_output_dir"`    // strategy == static; e.g. "dist"
	ImageRetentionCount int         `json:"image_retention_count"`
	Service             serviceSpec `json:"service"` // required unless build_strategy == compose
}

func (s *Server) createDeployment(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseID(chi.URLParam(r, "projectID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	var req createDeploymentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Name == "" || req.Slug == "" || req.BuildStrategy == "" {
		writeError(w, http.StatusBadRequest, "name, slug, and build_strategy are required")
		return
	}

	dep, err := s.Store.CreateDeployment(r.Context(), store.CreateDeploymentParams{
		ProjectID:           projectID,
		Name:                req.Name,
		Slug:                req.Slug,
		BuildStrategy:       req.BuildStrategy,
		GitBranch:           req.GitBranch,
		ImageRef:            req.ImageRef,
		RootPath:            req.RootPath,
		DockerfilePath:      req.DockerfilePath,
		ComposePath:         req.ComposePath,
		StaticBuildCommand:  req.StaticBuildCommand,
		StaticOutputDir:     req.StaticOutputDir,
		ImageRetentionCount: req.ImageRetentionCount,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Compose deployments discover their services from the compose file on
	// first deploy; every other strategy needs exactly one pre-declared
	// service for Deploy()'s single-container swap logic.
	if req.BuildStrategy != "compose" {
		if req.Service.Name == "" {
			s.Store.DeleteDeployment(r.Context(), dep.ID)
			writeError(w, http.StatusBadRequest, "service is required for non-compose build strategies")
			return
		}
		containerName := "mangrove-" + dep.Slug + "-" + req.Service.Name
		_, err := s.Store.CreateService(r.Context(), store.CreateServiceParams{
			DeploymentID:         dep.ID,
			Name:                 req.Service.Name,
			ContainerName:        containerName,
			InternalPort:         req.Service.InternalPort,
			IsInternalOnly:       req.Service.IsInternalOnly,
			CPULimitCores:        req.Service.CPULimitCores,
			MemoryLimitMB:        req.Service.MemoryLimitMB,
			HealthCheckPath:      req.Service.HealthCheckPath,
			HealthCheckIntervalS: req.Service.HealthCheckIntervalS,
			HealthCheckTimeoutS:  req.Service.HealthCheckTimeoutS,
			NoContainer:          req.BuildStrategy == "static",
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "deployment created but service creation failed: "+err.Error())
			return
		}
	}

	writeJSON(w, http.StatusCreated, dep)
}

func (s *Server) getDeployment(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "deploymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment id")
		return
	}
	dep, err := s.Store.GetDeployment(r.Context(), id)
	if err == store.ErrNotFound {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dep)
}

func (s *Server) listServices(w http.ResponseWriter, r *http.Request) {
	deploymentID, err := parseID(chi.URLParam(r, "deploymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment id")
		return
	}
	services, err := s.Store.ListServices(r.Context(), deploymentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, services)
}

func (s *Server) listDeployHistory(w http.ResponseWriter, r *http.Request) {
	deploymentID, err := parseID(chi.URLParam(r, "deploymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment id")
		return
	}
	history, err := s.Store.ListDeployHistory(r.Context(), deploymentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, history)
}

type triggerDeployRequest struct {
	GitURL        string `json:"git_url"`
	GitRef        string `json:"git_ref"`
	CommitSHA     string `json:"commit_sha"`
	CommitMessage string `json:"commit_message"`
	AuthToken     string `json:"auth_token"` // decrypted PAT; caller (CLI or webhook handler) resolves this
}

func (s *Server) triggerDeploy(w http.ResponseWriter, r *http.Request) {
	deploymentID, err := parseID(chi.URLParam(r, "deploymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment id")
		return
	}
	var req triggerDeployRequest
	if r.ContentLength != 0 {
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
	}

	dep, err := s.Store.GetDeployment(r.Context(), deploymentID)
	if err == store.ErrNotFound {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	deployReq := orchestrator.DeployRequest{
		DeploymentID:  deploymentID,
		TriggeredBy:   "manual",
		GitURL:        req.GitURL,
		GitRef:        req.GitRef,
		CommitSHA:     req.CommitSHA,
		CommitMessage: req.CommitMessage,
		AuthToken:     req.AuthToken,
	}

	var historyID int64
	switch dep.BuildStrategy {
	case "compose":
		historyID, err = s.Orchestrator.DeployCompose(r.Context(), deployReq)
	case "static":
		historyID, err = s.Orchestrator.DeployStatic(r.Context(), deployReq)
	default:
		historyID, err = s.Orchestrator.Deploy(r.Context(), deployReq)
	}
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"deploy_history_id": historyID,
			"error":             err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deploy_history_id": historyID, "status": "success"})
}

func (s *Server) triggerRollback(w http.ResponseWriter, r *http.Request) {
	historyID, err := parseID(chi.URLParam(r, "historyID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deploy history id")
		return
	}
	target, err := s.Store.GetDeployHistory(r.Context(), historyID)
	if err == store.ErrNotFound {
		writeError(w, http.StatusNotFound, "deploy history entry not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	targetDep, err := s.Store.GetDeployment(r.Context(), target.DeploymentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	rollbackReq := orchestrator.DeployRequest{
		DeploymentID:              target.DeploymentID,
		TriggeredBy:               "rollback",
		RollbackToDeployHistoryID: &historyID,
	}
	var newHistoryID int64
	if targetDep.BuildStrategy == "static" {
		newHistoryID, err = s.Orchestrator.DeployStatic(r.Context(), rollbackReq)
	} else {
		newHistoryID, err = s.Orchestrator.Deploy(r.Context(), rollbackReq)
	}
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"deploy_history_id": newHistoryID,
			"error":             err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deploy_history_id": newHistoryID, "status": "success"})
}

func (s *Server) getService(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, svc)
}
