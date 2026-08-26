package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/evanxdsouza/mangrove/internal/models"
	"github.com/evanxdsouza/mangrove/internal/orchestrator"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// dispatchDeploy routes a DeployRequest to the orchestrator method that
// matches the deployment's build strategy. This is the single place that
// decision is made -- every trigger (manual deploy, redeploy, webhook
// push, staging create, promote) calls this instead of duplicating the
// switch, which is what let the webhook path drift out of sync and
// silently mis-route static-strategy deployments into Deploy() (written
// for container-based strategies) instead of DeployStatic().
func (s *Server) dispatchDeploy(ctx context.Context, dep models.Deployment, req orchestrator.DeployRequest) (int64, error) {
	switch dep.BuildStrategy {
	case "compose":
		return s.Orchestrator.DeployCompose(ctx, req)
	case "static":
		return s.Orchestrator.DeployStatic(ctx, req)
	default:
		return s.Orchestrator.Deploy(ctx, req)
	}
}

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
	Name                string `json:"name"`
	Slug                string `json:"slug"`
	BuildStrategy       string `json:"build_strategy"` // dockerfile|nixpacks|compose|image|static
	GitBranch           string `json:"git_branch"`
	ImageRef            string `json:"image_ref"`
	RootPath            string `json:"root_path"`
	DockerfilePath      string `json:"dockerfile_path"`
	ComposePath         string `json:"compose_path"`
	StaticBuildCommand  string `json:"static_build_command"` // strategy == static; optional, omit if the repo is already pre-built
	StaticOutputDir     string `json:"static_output_dir"`    // strategy == static; e.g. "dist"
	ImageRetentionCount int    `json:"image_retention_count"`
	// Replicas is how many containers of the same image run for this
	// deployment (single-service strategies only). 0/omitted means 1.
	// Compose stacks are always 1.
	Replicas int         `json:"replicas"`
	Service  serviceSpec `json:"service"` // required unless build_strategy == compose
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

	replicas := req.Replicas
	if replicas < 1 || req.BuildStrategy == "compose" || req.BuildStrategy == "static" {
		replicas = 1
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
		Replicas:            replicas,
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

func (s *Server) deleteDeployment(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "deploymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment id")
		return
	}
	if err := s.Orchestrator.DeleteDeployment(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	err = s.Orchestrator.WithInflightDeploy(deploymentID, func(ctx context.Context) error {
		var e error
		historyID, e = s.dispatchDeploy(ctx, dep, deployReq)
		return e
	})
	if errors.Is(err, orchestrator.ErrDeployInProgress) {
		writeError(w, http.StatusConflict, err.Error())
		return
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

// cancelDeployment aborts an in-flight deploy for a deployment. The running
// deploy marks its deploy_history failed with the cancellation; this just
// fires the cancellation and returns. No-op (409) if nothing is deploying.
func (s *Server) cancelDeployment(w http.ResponseWriter, r *http.Request) {
	deploymentID, err := parseID(chi.URLParam(r, "deploymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment id")
		return
	}
	if err := s.Orchestrator.CancelDeploy(deploymentID); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	err = s.Orchestrator.WithInflightDeploy(target.DeploymentID, func(ctx context.Context) error {
		var e error
		if targetDep.BuildStrategy == "static" {
			newHistoryID, e = s.Orchestrator.DeployStatic(ctx, rollbackReq)
		} else {
			newHistoryID, e = s.Orchestrator.Deploy(ctx, rollbackReq)
		}
		return e
	})
	if errors.Is(err, orchestrator.ErrDeployInProgress) {
		writeError(w, http.StatusConflict, err.Error())
		return
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

func (s *Server) stopDeployment(w http.ResponseWriter, r *http.Request) {
	deploymentID, err := parseID(chi.URLParam(r, "deploymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment id")
		return
	}
	if err := s.Orchestrator.StopDeployment(r.Context(), deploymentID); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) restartDeployment(w http.ResponseWriter, r *http.Request) {
	deploymentID, err := parseID(chi.URLParam(r, "deploymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment id")
		return
	}
	if err := s.Orchestrator.RestartDeployment(r.Context(), deploymentID); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// redeployDeployment re-runs the full build+deploy pipeline for whatever
// source a deployment is already configured with -- a fresh build from the
// linked repo's current branch tip (for git-backed deployments) or the
// same image ref (for the image strategy) -- as opposed to POST .../deploy,
// which expects the caller (CLI, or the webhook handler) to already have
// resolved and supplied git_url/auth_token itself.
func (s *Server) redeployDeployment(w http.ResponseWriter, r *http.Request) {
	deploymentID, err := parseID(chi.URLParam(r, "deploymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment id")
		return
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

	deployReq := orchestrator.DeployRequest{DeploymentID: deploymentID, TriggeredBy: "redeploy"}

	buildReq, err := s.buildRedeployRequest(r.Context(), dep)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	deployReq = buildReq

	var historyID int64
	err = s.Orchestrator.WithInflightDeploy(deploymentID, func(ctx context.Context) error {
		var e error
		historyID, e = s.dispatchDeploy(ctx, dep, deployReq)
		return e
	})
	if errors.Is(err, orchestrator.ErrDeployInProgress) {
		writeError(w, http.StatusConflict, err.Error())
		return
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

// buildRedeployRequest resolves the source a deployment is configured with
// into a DeployRequest ready to feed the deploy pipeline: for a git-backed
// deployment, the linked repo's URL/branch and decrypted PAT; for the
// image strategy, no source (a redeploy just reuses the image ref). Shared
// by redeployDeployment and scaleDeployment.
func (s *Server) buildRedeployRequest(ctx context.Context, dep models.Deployment) (orchestrator.DeployRequest, error) {
	deployReq := orchestrator.DeployRequest{DeploymentID: dep.ID, TriggeredBy: "redeploy"}

	if dep.ProjectRepoID != nil {
		repo, err := s.Store.GetProjectRepoByID(ctx, *dep.ProjectRepoID)
		if err != nil {
			return deployReq, fmt.Errorf("load linked repo: %w", err)
		}
		ciphertext, nonce, err := s.Store.GetGithubPATEncrypted(ctx, repo.GithubPATID)
		if err != nil {
			return deployReq, fmt.Errorf("load repo credentials: %w", err)
		}
		token, err := s.Secrets.Open(patAAD(repo.GithubPATID), ciphertext, nonce)
		if err != nil {
			return deployReq, fmt.Errorf("decrypt repo credentials: %w", err)
		}
		s.Store.TouchGithubPATUsed(ctx, repo.GithubPATID)

		branch := dep.GitBranch
		if branch == "" {
			branch = repo.DefaultBranch
		}
		deployReq.GitURL = fmt.Sprintf("https://github.com/%s/%s.git", repo.RepoOwner, repo.RepoName)
		deployReq.GitRef = branch
		deployReq.AuthToken = string(token)
		return deployReq, nil
	}
	if dep.BuildStrategy != "image" {
		return deployReq, fmt.Errorf("deployment has no linked repository to redeploy from; link a repo first, or use POST .../deploy with explicit git parameters")
	}
	return deployReq, nil
}

type scaleDeploymentRequest struct {
	Replicas int `json:"replicas"`
}

// scaleDeployment changes a deployment's replica count and triggers a
// redeploy so the new count takes effect -- scaling up/down is implemented
// as a blue/green swap that starts (or tears down) the requested number of
// containers. Only single-service strategies (dockerfile/nixpacks/image)
// are scalable; compose stacks and static sites stay at 1.
func (s *Server) scaleDeployment(w http.ResponseWriter, r *http.Request) {
	deploymentID, err := parseID(chi.URLParam(r, "deploymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment id")
		return
	}
	var req scaleDeploymentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Replicas < 1 || req.Replicas > 32 {
		writeError(w, http.StatusBadRequest, "replicas must be between 1 and 32")
		return
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
	if dep.BuildStrategy == "compose" || dep.BuildStrategy == "static" {
		writeError(w, http.StatusUnprocessableEntity, "scaling is only supported for single-container build strategies (dockerfile/nixpacks/image)")
		return
	}

	if err := s.Store.UpdateDeploymentReplicas(r.Context(), deploymentID, req.Replicas); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	buildReq, err := s.buildRedeployRequest(r.Context(), dep)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	var historyID int64
	err = s.Orchestrator.WithInflightDeploy(deploymentID, func(ctx context.Context) error {
		historyID, err = s.Orchestrator.Deploy(ctx, buildReq)
		return err
	})
	if errors.Is(err, orchestrator.ErrDeployInProgress) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"deploy_history_id": historyID,
			"error":             err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deploy_history_id": historyID, "replicas": req.Replicas, "status": "success"})
}

// ---- Staging environments ----
//
// A staging deployment is just another deployment row: same project, same
// build config, its own branch under the same linked repo, its own slug
// (and so its own subdomain), auto-deploying independently of production.
// promotes_to_deployment_id is the only thing that marks it as staging
// rather than a second, unrelated production deployment -- see migration
// 0007 and models.Deployment.

func (s *Server) listStagingDeployments(w http.ResponseWriter, r *http.Request) {
	deploymentID, err := parseID(chi.URLParam(r, "deploymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment id")
		return
	}
	staging, err := s.Store.ListStagingDeployments(r.Context(), deploymentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, staging)
}

// cloneDeploymentSpec parameterizes cloneDeployment for its two callers:
// createStagingDeployment (a hand-picked branch, one at a time) and
// findOrCreatePreviewDeployment (one per open pull request, driven by the
// pull_request webhook).
type cloneDeploymentSpec struct {
	Branch      string
	Environment string // "staging" or "preview"
	SlugBase    string // slug (and so subdomain) tried first; suffixed -2, -3... on collision
	Name        string
	PRNumber    *int // set only for Environment "preview"
}

// cloneDeployment clones source's build config (services, env vars) into a
// new deployment linked to the same repo on a different branch, with
// auto-deploy on. It does not itself trigger a deploy -- callers dispatch
// whatever DeployRequest fits their trigger (a fresh branch-tip deploy for
// a hand-created staging environment, or the exact commit a pull_request
// webhook delivery named for a preview).
func (s *Server) cloneDeployment(ctx context.Context, source models.Deployment, spec cloneDeploymentSpec) (models.Deployment, error) {
	sourceServices, err := s.Store.ListServices(ctx, source.ID)
	if err != nil {
		return models.Deployment{}, err
	}
	if source.BuildStrategy != "compose" && len(sourceServices) != 1 {
		return models.Deployment{}, fmt.Errorf("source deployment has %d services; expected exactly 1", len(sourceServices))
	}

	replicas := source.Replicas
	if replicas < 1 || source.BuildStrategy == "compose" || source.BuildStrategy == "static" {
		replicas = 1
	}

	// Slug (and so subdomain) must be unique per project -- try a handful
	// of suffixes rather than making the caller pick one.
	var clone models.Deployment
	for attempt := 0; attempt < 6; attempt++ {
		slug := spec.SlugBase
		if attempt > 0 {
			slug = fmt.Sprintf("%s-%d", spec.SlugBase, attempt+1)
		}
		clone, err = s.Store.CreateDeployment(ctx, store.CreateDeploymentParams{
			ProjectID:              source.ProjectID,
			Name:                   spec.Name,
			Slug:                   slug,
			BuildStrategy:          source.BuildStrategy,
			GitBranch:              spec.Branch,
			ImageRef:               source.ImageRef,
			RootPath:               source.RootPath,
			DockerfilePath:         source.DockerfilePath,
			ComposePath:            source.ComposePath,
			StaticBuildCommand:     source.StaticBuildCommand,
			StaticOutputDir:        source.StaticOutputDir,
			ImageRetentionCount:    source.ImageRetentionCount,
			Replicas:               replicas,
			Environment:            spec.Environment,
			PromotesToDeploymentID: &source.ID,
			PRNumber:               spec.PRNumber,
		})
		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return models.Deployment{}, err
		}
	}
	if err != nil {
		return models.Deployment{}, fmt.Errorf("couldn't find a free slug: %w", err)
	}

	// Clone each service (config + env vars) onto the new deployment.
	// Compose stacks discover their services from the compose file on
	// first deploy, same as a fresh compose deployment created any other
	// way -- there's nothing to pre-declare or clone.
	for _, svc := range sourceServices {
		containerName := "mangrove-" + clone.Slug + "-" + svc.Name
		newSvc, err := s.Store.CreateService(ctx, store.CreateServiceParams{
			DeploymentID:         clone.ID,
			Name:                 svc.Name,
			ContainerName:        containerName,
			InternalPort:         svc.InternalPort,
			IsInternalOnly:       svc.IsInternalOnly,
			CPULimitCores:        svc.CPULimitCores,
			MemoryLimitMB:        svc.MemoryLimitMB,
			RestartPolicy:        svc.RestartPolicy,
			HealthCheckPath:      svc.HealthCheckPath,
			HealthCheckIntervalS: svc.HealthCheckIntervalS,
			HealthCheckTimeoutS:  svc.HealthCheckTimeoutS,
			Command:              svc.Command,
			NoContainer:          source.BuildStrategy == "static",
		})
		if err != nil {
			return clone, fmt.Errorf("deployment created but service cloning failed: %w", err)
		}

		envRows, err := s.Store.ListEnvVarRows(ctx, svc.ID)
		if err != nil {
			return clone, fmt.Errorf("deployment created but env var lookup failed: %w", err)
		}
		for _, row := range envRows {
			if !row.IsSecret {
				if err := s.Store.CreatePlainEnvVar(ctx, newSvc.ID, row.KeyName, row.ValuePlain.String); err != nil {
					s.Log.Warn("clone deployment: failed to clone env var", "key", row.KeyName, "error", err)
				}
				continue
			}
			// A secret's ciphertext is bound (as AAD) to the service ID it
			// belongs to (see resolveEnv/setEnvVar) -- it must be
			// decrypted and re-sealed under the new service's ID, not
			// copied as-is.
			oldAAD := []byte("env_vars:" + strconv.FormatInt(svc.ID, 10) + ":" + row.KeyName)
			plaintext, err := s.Secrets.Open(oldAAD, row.ValueEncrypted, row.ValueNonce)
			if err != nil {
				s.Log.Warn("clone deployment: failed to decrypt secret env var for cloning", "key", row.KeyName, "error", err)
				continue
			}
			newAAD := []byte("env_vars:" + strconv.FormatInt(newSvc.ID, 10) + ":" + row.KeyName)
			ciphertext, nonce, err := s.Secrets.Seal(newAAD, plaintext)
			if err != nil {
				s.Log.Warn("clone deployment: failed to re-encrypt secret env var for cloning", "key", row.KeyName, "error", err)
				continue
			}
			if err := s.Store.CreateSecretEnvVar(ctx, newSvc.ID, row.KeyName, ciphertext, nonce); err != nil {
				s.Log.Warn("clone deployment: failed to clone secret env var", "key", row.KeyName, "error", err)
			}
		}
	}

	if err := s.Store.SetDeploymentRepo(ctx, clone.ID, *source.ProjectRepoID, spec.Branch, true); err != nil {
		return clone, fmt.Errorf("deployment created but repo link failed: %w", err)
	}
	return s.Store.GetDeployment(ctx, clone.ID)
}

type createStagingDeploymentRequest struct {
	Branch string `json:"branch"`
}

// createStagingDeployment clones a production deployment's build config
// into a new deployment tracking a different branch of the same linked
// repo, then deploys it immediately. It requires a linked repo (a staging
// environment with nothing to auto-deploy from isn't useful) but the
// source deployment being cloned may itself be a staging deployment, or a
// non-git deployment can't be staged at all -- both rejected below.
func (s *Server) createStagingDeployment(w http.ResponseWriter, r *http.Request) {
	sourceID, err := parseID(chi.URLParam(r, "deploymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment id")
		return
	}
	var req createStagingDeploymentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Branch == "" {
		writeError(w, http.StatusBadRequest, "branch is required")
		return
	}

	source, err := s.Store.GetDeployment(r.Context(), sourceID)
	if err == store.ErrNotFound {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if source.ProjectRepoID == nil {
		writeError(w, http.StatusUnprocessableEntity, "this deployment has no linked GitHub repo to stage a branch from -- connect one first")
		return
	}
	if source.PromotesToDeploymentID != nil {
		writeError(w, http.StatusUnprocessableEntity, "this deployment is itself a staging environment -- create the new one from its production deployment instead")
		return
	}

	staging, err := s.cloneDeployment(r.Context(), source, cloneDeploymentSpec{
		Branch:      req.Branch,
		Environment: "staging",
		SlugBase:    source.Slug + "-staging",
		Name:        source.Name + " (staging)",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Kick off the first deploy immediately, same as the GitHub wizard does
	// for a fresh production deployment. Non-fatal: the staging deployment
	// still exists (and will auto-deploy on its next push) even if this
	// first attempt fails.
	var deployError string
	deployReq, err := s.buildRedeployRequest(r.Context(), staging)
	if err != nil {
		deployError = err.Error()
	} else {
		err = s.Orchestrator.WithInflightDeploy(staging.ID, func(ctx context.Context) error {
			_, e := s.dispatchDeploy(ctx, staging, deployReq)
			return e
		})
		if err != nil {
			deployError = err.Error()
		}
	}

	resp := map[string]any{"deployment": staging}
	if deployError != "" {
		resp["deploy_error"] = deployError
	}
	writeJSON(w, http.StatusCreated, resp)
}

// promoteDeployment deploys a staging deployment's current commit onto its
// production deployment -- "ship exactly what's running in staging", not
// just "redeploy production from its branch's tip", which could be a
// different (later, or differently-configured) commit than whatever was
// actually verified in staging.
func (s *Server) promoteDeployment(w http.ResponseWriter, r *http.Request) {
	stagingID, err := parseID(chi.URLParam(r, "deploymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment id")
		return
	}
	staging, err := s.Store.GetDeployment(r.Context(), stagingID)
	if err == store.ErrNotFound {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if staging.PromotesToDeploymentID == nil {
		writeError(w, http.StatusUnprocessableEntity, "this deployment isn't a staging environment (no production deployment to promote into)")
		return
	}
	prod, err := s.Store.GetDeployment(r.Context(), *staging.PromotesToDeploymentID)
	if err == store.ErrNotFound {
		writeError(w, http.StatusUnprocessableEntity, "the production deployment this staging environment promotes into no longer exists")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	current, err := s.Store.GetCurrentDeployHistory(r.Context(), staging.ID)
	if err == store.ErrNotFound {
		writeError(w, http.StatusUnprocessableEntity, "staging has no successful deploy yet -- nothing to promote")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Prefer the exact commit staging is running; only a manual trigger
	// that never recorded one (e.g. a plain redeploy outside of a webhook
	// push) falls back to whatever ref staging last built from.
	promoteRef := current.CommitSHA
	if promoteRef == "" {
		promoteRef = current.GitRef
	}
	if promoteRef == "" {
		writeError(w, http.StatusUnprocessableEntity, "staging's current deploy has no recorded commit or branch to promote")
		return
	}

	deployReq, err := s.buildRedeployRequest(r.Context(), prod)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	deployReq.TriggeredBy = "promote"
	deployReq.GitRef = promoteRef
	deployReq.CommitSHA = current.CommitSHA
	deployReq.CommitMessage = current.CommitMessage

	var historyID int64
	err = s.Orchestrator.WithInflightDeploy(prod.ID, func(ctx context.Context) error {
		var e error
		historyID, e = s.dispatchDeploy(ctx, prod, deployReq)
		return e
	})
	if errors.Is(err, orchestrator.ErrDeployInProgress) {
		writeError(w, http.StatusConflict, err.Error())
		return
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

// ---- PR previews ----
//
// A preview deployment is, like staging, just another deployment row
// (environment "preview", promotes_to_deployment_id naming the production
// deployment it was cloned from) -- pr_number additionally keys it to a
// specific pull request instead of a hand-picked branch, and it's created/
// torn down automatically by the pull_request webhook (see webhook.go's
// handlePullRequestEvent) rather than by a user action. See migration 0008.

func (s *Server) listPreviewDeployments(w http.ResponseWriter, r *http.Request) {
	deploymentID, err := parseID(chi.URLParam(r, "deploymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment id")
		return
	}
	previews, err := s.Store.ListPreviewDeployments(r.Context(), deploymentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, previews)
}

type setPRPreviewsRequest struct {
	Enabled bool `json:"enabled"`
}

// setPRPreviews toggles whether this (production) deployment spins up a
// preview deployment for every open pull request against its linked repo.
func (s *Server) setPRPreviews(w http.ResponseWriter, r *http.Request) {
	deploymentID, err := parseID(chi.URLParam(r, "deploymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment id")
		return
	}
	var req setPRPreviewsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Enabled {
		dep, err := s.Store.GetDeployment(r.Context(), deploymentID)
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "deployment not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if dep.ProjectRepoID == nil {
			writeError(w, http.StatusUnprocessableEntity, "this deployment has no linked GitHub repo -- connect one first")
			return
		}
		if dep.PromotesToDeploymentID != nil {
			writeError(w, http.StatusUnprocessableEntity, "PR previews can only be enabled on a production deployment")
			return
		}
	}
	if err := s.Store.SetPRPreviewsEnabled(r.Context(), deploymentID, req.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// findOrCreatePreviewDeployment returns the preview deployment already
// tracking prNumber for source, or clones one (same machinery as
// createStagingDeployment) if this is that PR's first opened/synchronize
// delivery. Does not itself deploy -- the caller (handlePullRequestEvent)
// dispatches the exact commit the webhook payload named.
func (s *Server) findOrCreatePreviewDeployment(ctx context.Context, source models.Deployment, prNumber int, branch string) (models.Deployment, error) {
	existing, err := s.Store.GetPreviewDeployment(ctx, source.ID, prNumber)
	if err == nil {
		return existing, nil
	}
	if err != store.ErrNotFound {
		return models.Deployment{}, err
	}
	pr := prNumber
	return s.cloneDeployment(ctx, source, cloneDeploymentSpec{
		Branch:      branch,
		Environment: "preview",
		SlugBase:    fmt.Sprintf("%s-pr-%d", source.Slug, prNumber),
		Name:        fmt.Sprintf("%s (PR #%d)", source.Name, prNumber),
		PRNumber:    &pr,
	})
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

type execCommandRequest struct {
	// Command is exec-form (like Service.Command / RunSpec.Command): passed
	// straight to `docker exec`, not through a shell, unless an entry
	// itself invokes one (e.g. ["sh", "-c", "npm run migrate"]).
	Command []string `json:"command"`
}

// execServiceCommand runs a one-off command in a service's running
// container -- e.g. a database migration after a deploy. It is synchronous
// and buffers output in memory, so it's a fit for short commands, not a
// long-running or high-volume process.
func (s *Server) execServiceCommand(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid service id")
		return
	}
	var req execCommandRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if len(req.Command) == 0 {
		writeError(w, http.StatusBadRequest, "command is required")
		return
	}

	result, err := s.Orchestrator.RunServiceCommand(r.Context(), id, req.Command)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"output": result.Output, "exit_code": result.ExitCode})
}
