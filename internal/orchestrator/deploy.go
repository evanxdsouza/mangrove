// Package orchestrator drives the build -> health-check-gated swap ->
// record-history sequence for a deploy. It depends only on the Executor
// interface, never on Docker specifics directly, so it works unchanged
// against any future remote executor.
package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/evanxdsouza/mangrove/internal/config"
	"github.com/evanxdsouza/mangrove/internal/executor"
	"github.com/evanxdsouza/mangrove/internal/github"
	"github.com/evanxdsouza/mangrove/internal/models"
	"github.com/evanxdsouza/mangrove/internal/notify"
	"github.com/evanxdsouza/mangrove/internal/portregistry"
	"github.com/evanxdsouza/mangrove/internal/proxy"
	"github.com/evanxdsouza/mangrove/internal/secrets"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// ErrMemoryCeilingExceeded is returned by admission control when a deploy
// would push total configured deployment memory past the configured cap.
type ErrMemoryCeilingExceeded struct {
	RequestedMB, AlreadyAllocatedMB, CeilingMB int
}

func (e *ErrMemoryCeilingExceeded) Error() string {
	return fmt.Sprintf("deploy would allocate %dMB on top of %dMB already committed, exceeding the %dMB ceiling",
		e.RequestedMB, e.AlreadyAllocatedMB, e.CeilingMB)
}

type Orchestrator struct {
	Store    *store.Store
	Exec     executor.Executor
	Compose  *executor.ComposeExecutor
	Proxy    *proxy.Client // nil is valid: routing is skipped (e.g. Caddy not installed yet in a dev environment)
	Secrets  *secrets.Box
	Notifier *notify.ResendClient // nil or !Enabled() is valid: notifications are skipped, not an error
	GHStatus *github.StatusClient // nil is valid: commit-status posting is skipped, not an error
	Config   config.Config
	Log      *slog.Logger

	// inflight tracks deploys currently running, keyed by deployment ID, so
	// a concurrent deploy is refused and an in-flight one can be cancelled
	// (see BeginDeploy/CancelDeploy). Guards against double-deploying the
	// same deployment and backs the POST .../cancel endpoint.
	inflightMu sync.Mutex
	inflight   map[int64]context.CancelFunc
}

// postCommitStatus sets a GitHub commit status for a deploy, best-effort.
// Only applies to deploys that came in with both a commit SHA and a
// decrypted PAT already resolved (i.e. webhook-triggered deploys of a
// linked repo) -- a manual deploy from a local/file:// git URL has no
// commit on github.com to annotate, so it's silently skipped rather than
// treated as an error.
func (o *Orchestrator) postCommitStatus(ctx context.Context, dep models.Deployment, req DeployRequest, state github.State, description string) {
	if o.GHStatus == nil || req.CommitSHA == "" || req.AuthToken == "" || dep.ProjectRepoID == nil {
		return
	}
	repo, err := o.Store.GetProjectRepoByID(ctx, *dep.ProjectRepoID)
	if err != nil {
		o.Log.Warn("commit status: failed to load linked repo", "deployment_id", dep.ID, "error", err)
		return
	}
	targetURL := fmt.Sprintf("https://%s.%s", dep.Slug, o.Config.BaseDomain)
	if err := o.GHStatus.PostStatus(ctx, req.AuthToken, repo.RepoOwner, repo.RepoName, req.CommitSHA, state, description, targetURL); err != nil {
		o.Log.Warn("commit status: post failed", "deployment_id", dep.ID, "state", state, "error", err)
	}
}

// notifyDeploySuccess sends the deploy-success email (if configured) and
// always records the attempt to notifications_log -- visible in the admin
// panel regardless of whether it actually sent.
func (o *Orchestrator) notifyDeploySuccess(ctx context.Context, dep models.Deployment, port int) {
	if o.Config.NotifyToEmail == "" {
		return
	}
	subject := fmt.Sprintf("%s deployed successfully", dep.Name)
	logEntry := store.Notification{
		Type: "deploy_success", DeploymentID: &dep.ID, Channel: "email",
		Recipient: o.Config.NotifyToEmail, Subject: subject,
	}

	if !o.Notifier.Enabled() {
		logEntry.Status = "failed"
		logEntry.ErrorMessage = "Resend API key not configured (MANGROVE_RESEND_API_KEY)"
		o.Store.CreateNotification(ctx, logEntry)
		return
	}

	domainSlug := fmt.Sprintf("%s.%s", dep.Slug, o.Config.BaseDomain)
	err := o.Notifier.SendDeploySuccess(ctx, o.Config.NotifyToEmail, notify.DeploySuccessParams{
		AppName:         dep.Name,
		Port:            port,
		SuggestedDomain: domainSlug,
	})
	if err != nil {
		logEntry.Status = "failed"
		logEntry.ErrorMessage = err.Error()
		o.Log.Warn("deploy-success email failed", "deployment_id", dep.ID, "error", err)
	} else {
		logEntry.Status = "sent"
	}
	o.Store.CreateNotification(ctx, logEntry)
}

// notifyAccessChanged sends the same "app is up" email as notifyDeploySuccess,
// for the case where a deployment goes public (and gets a host port
// allocated for the first time) outside of a Deploy() call -- e.g. flipping
// an internal-only deployment public via SetAccessControl. Logged under its
// own notification type so the admin panel doesn't show it as a deploy.
func (o *Orchestrator) notifyAccessChanged(ctx context.Context, dep models.Deployment, port int) {
	if o.Config.NotifyToEmail == "" {
		return
	}
	subject := fmt.Sprintf("%s is now publicly reachable", dep.Name)
	logEntry := store.Notification{
		Type: "access_control_change", DeploymentID: &dep.ID, Channel: "email",
		Recipient: o.Config.NotifyToEmail, Subject: subject,
	}

	if !o.Notifier.Enabled() {
		logEntry.Status = "failed"
		logEntry.ErrorMessage = "Resend API key not configured (MANGROVE_RESEND_API_KEY)"
		o.Store.CreateNotification(ctx, logEntry)
		return
	}

	domainSlug := fmt.Sprintf("%s.%s", dep.Slug, o.Config.BaseDomain)
	err := o.Notifier.SendDeploySuccess(ctx, o.Config.NotifyToEmail, notify.DeploySuccessParams{
		AppName:         dep.Name,
		Port:            port,
		SuggestedDomain: domainSlug,
	})
	if err != nil {
		logEntry.Status = "failed"
		logEntry.ErrorMessage = err.Error()
		o.Log.Warn("access-change email failed", "deployment_id", dep.ID, "error", err)
	} else {
		logEntry.Status = "sent"
	}
	o.Store.CreateNotification(ctx, logEntry)
}

type DeployRequest struct {
	DeploymentID  int64
	TriggeredBy   string // manual|push|api|rollback
	CommitSHA     string
	CommitMessage string
	GitURL        string // resolved from project_repos by the caller; empty for image-strategy deployments
	GitRef        string
	AuthToken     string // decrypted GitHub PAT, if the deployment is git-backed

	// RollbackToDeployHistoryID, when set, skips the build step entirely
	// and reuses the image tag already recorded for that prior deploy.
	RollbackToDeployHistoryID *int64

	// Files are inline files to bind-mount into the container read-only at
	// start (see executor.FileMount). Only meaningful for image-strategy
	// deploys and only passed by InstallTemplate -- ordinary redeploys of a
	// template-created deployment carry no files (the store has no rows for
	// them), which is fine because they're only needed at first boot (e.g.
	// a Postgres initdb.d script).
	Files []executor.FileMount
}

const healthCheckPollInterval = 2 * time.Second
const healthCheckSwapTimeout = 60 * time.Second

// Deploy runs one deploy round for a single-service deployment
// (dockerfile/nixpacks/image strategies). Compose stacks use DeployCompose.
func (o *Orchestrator) Deploy(ctx context.Context, req DeployRequest) (deployHistoryID int64, err error) {
	dep, err := o.Store.GetDeployment(ctx, req.DeploymentID)
	if err != nil {
		return 0, fmt.Errorf("load deployment: %w", err)
	}

	services, err := o.Store.ListServices(ctx, dep.ID)
	if err != nil {
		return 0, fmt.Errorf("load services: %w", err)
	}
	if len(services) != 1 {
		return 0, fmt.Errorf("deployment %d has %d services; Deploy() handles exactly 1 (use DeployCompose for multi-service stacks)", dep.ID, len(services))
	}
	svc := services[0]

	historyID, err := o.Store.CreateDeployHistory(ctx, dep.ID, req.TriggeredBy, req.CommitSHA, req.CommitMessage, req.GitRef)
	if err != nil {
		return 0, fmt.Errorf("create deploy_history: %w", err)
	}
	if req.RollbackToDeployHistoryID != nil {
		o.Store.SetRollbackOf(ctx, historyID, *req.RollbackToDeployHistoryID)
	}
	o.Store.UpdateDeploymentStatus(ctx, dep.ID, "building")
	o.Store.UpdateDeployHistoryStatus(ctx, historyID, "building", "")
	o.postCommitStatus(ctx, dep, req, github.StatePending, "Deploying via Mangrove")

	fail := func(stepErr error) (int64, error) {
		o.Store.UpdateDeployHistoryStatus(ctx, historyID, "failed", stepErr.Error())
		o.Store.UpdateDeploymentStatus(ctx, dep.ID, "failed")
		o.postCommitStatus(ctx, dep, req, github.StateFailure, stepErr.Error())
		return historyID, stepErr
	}

	// Admission control: the sum of every non-stopped deployment's
	// configured memory, plus this one, must not exceed the ceiling backing
	// mangrove-deployments.slice's MemoryMax.
	alreadyAllocated, err := o.Store.SumConfiguredMemoryMB(ctx, dep.ID)
	if err != nil {
		return fail(fmt.Errorf("check memory budget: %w", err))
	}
	if alreadyAllocated+svc.MemoryLimitMB*dep.Replicas > o.Config.DeploymentMemoryCeilingMB {
		return fail(&ErrMemoryCeilingExceeded{
			RequestedMB:        svc.MemoryLimitMB * dep.Replicas,
			AlreadyAllocatedMB: alreadyAllocated,
			CeilingMB:          o.Config.DeploymentMemoryCeilingMB,
		})
	}

	var imageTag, imageID string
	if req.RollbackToDeployHistoryID != nil {
		artifact, err := o.Store.GetArtifactForServiceAtDeployHistory(ctx, *req.RollbackToDeployHistoryID, svc.ID)
		if err != nil {
			return fail(fmt.Errorf("load rollback target artifact: %w", err))
		}
		imageTag, imageID = artifact.ImageTag, artifact.ImageID
	} else {
		imageTag = fmt.Sprintf("mangrove/%s-%s:%d", dep.Slug, svc.Name, historyID)

		strategy := executor.BuildStrategy(dep.BuildStrategy)
		buildSpec := executor.BuildSpec{
			Strategy:       strategy,
			RootPath:       dep.RootPath,
			DockerfilePath: dep.DockerfilePath,
			ImageTag:       imageTag,
			ImageRef:       dep.ImageRef,
			CacheKey:       fmt.Sprintf("%s-%s", dep.Slug, svc.Name),
		}
		if strategy != executor.StrategyImage {
			buildSpec.Context = executor.ContextSource{
				GitURL:    req.GitURL,
				GitRef:    req.GitRef,
				AuthToken: req.AuthToken,
			}
		}

		var buildLog bytes.Buffer
		result, err := o.Exec.Build(ctx, buildSpec, &buildLog)
		if err != nil {
			return fail(fmt.Errorf("build: %w: %s", err, lastLines(buildLog.String(), 20)))
		}
		imageTag, imageID = result.ImageTag, result.ImageID
		if imageTag == "" {
			imageTag = buildSpec.ImageTag
		}
	}

	o.Store.UpdateDeployHistoryStatus(ctx, historyID, "healthchecking", "")

	// Port allocation: this only reserves a port_registry row for the
	// service (for the admin panel and for Caddy, once task 9 wires it up
	// to reverse-proxy this port to whichever container is currently
	// healthy). It is deliberately NOT passed to RunSpec.HostPort: binding
	// the host port straight to the container would mean the old and new
	// containers can't both be up during the health-check gate below (they'd
	// collide on the same host port), which defeats the swap entirely. Until
	// Caddy is wired, a public service is reachable only via its internal
	// container address -- see plan §3, "services needing raw TCP host-port
	// exposure opt out explicitly", which is not the default path here.
	registeredPort := svc.HostPort
	if !svc.IsInternalOnly && registeredPort == nil {
		p, err := portregistry.AllocateForService(ctx, o.Store.DB, svc.ID, o.Config.PortRangeMin, o.Config.PortRangeMax)
		if err != nil {
			return fail(fmt.Errorf("allocate port: %w", err))
		}
		registeredPort = &p
	}

	env, err := o.resolveEnv(ctx, svc.ID)
	if err != nil {
		return fail(fmt.Errorf("resolve env vars: %w", err))
	}
	// Tell the container what port to bind to via $PORT -- the de facto
	// buildpack/nixpacks convention (and one plenty of hand-written
	// Dockerfiles follow too). Without this, a service's InternalPort is
	// pure guesswork on the caller's part (e.g. "Deploy from GitHub"'s
	// wizard always suggests 3000) that has no effect on which port the
	// app actually listens on, so the health check times out against a
	// port nothing is bound to. An explicit user-set PORT wins.
	if svc.InternalPort > 0 {
		if _, ok := env["PORT"]; !ok {
			env["PORT"] = strconv.Itoa(svc.InternalPort)
		}
	}

	vols, err := o.Store.ListVolumesForService(ctx, svc.ID)
	if err != nil {
		return fail(fmt.Errorf("load volumes: %w", err))
	}
	volumeMounts := make([]executor.VolumeMount, 0, len(vols))
	for _, v := range vols {
		volumeMounts = append(volumeMounts, executor.VolumeMount{Name: v.DockerVolumeName, MountPath: v.MountPath})
	}

	newContainerName := fmt.Sprintf("%s-%d", svc.ContainerName, historyID)
	runSpec := executor.RunSpec{
		ImageRef:      imageTag,
		ContainerName: newContainerName,
		Env:           env,
		Network:       o.Config.NetworkName,
		// NetworkAlias is svc.ContainerName itself (not newContainerName):
		// stable across every deploy of this service, unlike the
		// per-history container name, so other services (e.g. a
		// template-linked app container resolving its DB dependency) can
		// keep addressing it by the same name across swaps/rollbacks.
		NetworkAlias:  svc.ContainerName,
		InternalPort:  svc.InternalPort,
		CPULimitCores: svc.CPULimitCores,
		MemoryLimitMB: svc.MemoryLimitMB,
		RestartPolicy: svc.RestartPolicy,
		Volumes:       volumeMounts,
		Command:       svc.Command,
		Files:         req.Files,
		Replicas:      dep.Replicas,
		CgroupParent:  o.Config.CgroupParent,
	}

	runResult, err := o.Exec.Run(ctx, runSpec)
	if err != nil {
		return fail(fmt.Errorf("run new containers: %w", err))
	}

	// The full set of freshly-started containers (primary first) -- the
	// unit of swap. With replicas == 1 this is a single element.
	newContainerIDs := []string{runResult.ContainerID}
	for _, r := range runResult.Replicas {
		newContainerIDs = append(newContainerIDs, r.ContainerID)
	}

	healthy := o.waitHealthy(ctx, newContainerName, svc)
	if !healthy {
		o.teardownContainers(ctx, newContainerIDs)
		return fail(fmt.Errorf("new containers failed health check within %s; old set (if any) left running", healthCheckSwapTimeout))
	}

	// Repoint Caddy at the new container(s) BEFORE tearing down the old
	// ones -- this is the atomic part of the swap (see internal/proxy).
	// For a replicated deployment every new replica is a load-balanced
	// upstream. Only after traffic is flowing to the new set is it safe to
	// remove the old.
	upstreams := []string{runResult.ContainerAddr}
	for _, r := range runResult.Replicas {
		upstreams = append(upstreams, r.ContainerAddr)
	}
	if o.Proxy != nil && !svc.IsInternalOnly && registeredPort != nil && runResult.ContainerAddr != "" {
		routeOpts := proxy.RouteOptions{}
		if dep.PasswordProtected {
			hash, err := o.Store.GetDeploymentPasswordHash(ctx, dep.ID)
			if err != nil {
				o.Log.Warn("failed to load password hash for password-protected deployment; route will be unprotected", "deployment_id", dep.ID, "error", err)
			} else {
				routeOpts = proxy.RouteOptions{PasswordProtected: true, Username: basicAuthUsername, BcryptHash: hash}
			}
		}
		if err := o.Proxy.PutRouteMulti(ctx, *registeredPort, upstreams, routeOpts); err != nil {
			o.teardownContainers(ctx, newContainerIDs)
			return fail(fmt.Errorf("update proxy route: %w", err))
		}
	}

	// Tear down the previous deploy's replica set (primary + any replicas
	// recorded from the last successful deploy).
	oldIDs := append([]string{}, svc.ReplicaContainerIDs...)
	if svc.ContainerIDCurrent != "" {
		oldIDs = append(oldIDs, svc.ContainerIDCurrent)
	}
	o.teardownContainers(ctx, oldIDs)

	if err := o.Store.CreateDeployArtifact(ctx, historyID, svc.ID, imageTag, imageID, "", ""); err != nil {
		o.Log.Warn("failed to record deploy artifact", "error", err)
	}
	if err := o.Store.UpdateServiceRuntime(ctx, svc.ID, imageTag, runResult.ContainerID, "running"); err != nil {
		o.Log.Warn("failed to update service runtime state", "error", err)
	}
	if err := o.Store.UpdateServiceReplicas(ctx, svc.ID, newContainerIDs); err != nil {
		o.Log.Warn("failed to record replica state", "service_id", svc.ID, "error", err)
	}

	o.Store.UpdateDeployHistoryStatus(ctx, historyID, "success", "")
	o.Store.MarkDeployHistoryCurrent(ctx, dep.ID, historyID)
	o.Store.UpdateDeploymentStatus(ctx, dep.ID, "running")
	o.Store.TouchDeploymentDeployed(ctx, dep.ID)

	o.pruneOldImages(ctx, svc.ID, dep.ImageRetentionCount)

	notifyPort := 0
	if registeredPort != nil {
		notifyPort = *registeredPort
	}
	o.notifyDeploySuccess(ctx, dep, notifyPort)
	o.postCommitStatus(ctx, dep, req, github.StateSuccess, "Deployed successfully")

	return historyID, nil
}

// teardownContainers stops and removes a set of running containers
// best-effort, logging rather than failing. Used to clean up a freshly-run
// replica set when a deploy fails mid-swap, and to tear down a previous
// deploy's replica set once the new one is healthy.
func (o *Orchestrator) teardownContainers(ctx context.Context, containerIDs []string) {
	for _, id := range containerIDs {
		if id == "" {
			continue
		}
		if err := o.Exec.Stop(ctx, id, 10*time.Second); err != nil {
			o.Log.Warn("teardown: stop container failed", "container_id", id, "error", err)
		}
		if err := o.Exec.Remove(ctx, id); err != nil {
			o.Log.Warn("teardown: remove container failed", "container_id", id, "error", err)
		}
	}
}

func (o *Orchestrator) waitHealthy(ctx context.Context, containerName string, svc models.Service) bool {
	if svc.HealthCheckPath == "" {
		// No HTTP health check configured -- fall back to "container is
		// still running after a short grace period" so a crash-looping
		// image doesn't get promoted.
		time.Sleep(3 * time.Second)
		stats, err := o.Exec.Stats(ctx, containerName)
		return err == nil && stats.MemUsageMB >= 0
	}

	deadline := time.Now().Add(healthCheckSwapTimeout)
	spec := executor.HealthCheckSpec{Path: svc.HealthCheckPath, Port: svc.InternalPort, TimeoutSeconds: svc.HealthCheckTimeoutS}
	for time.Now().Before(deadline) {
		status, err := o.Exec.HealthCheck(ctx, containerName, spec)
		if err == nil && status.Healthy {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(healthCheckPollInterval):
		}
	}
	return false
}

func (o *Orchestrator) resolveEnv(ctx context.Context, serviceID int64) (map[string]string, error) {
	rows, err := o.Store.ListEnvVarRows(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	env := make(map[string]string, len(rows))
	for _, r := range rows {
		if !r.IsSecret {
			env[r.KeyName] = r.ValuePlain.String
			continue
		}
		aad := []byte("env_vars:" + strconv.FormatInt(serviceID, 10) + ":" + r.KeyName)
		plaintext, err := o.Secrets.Open(aad, r.ValueEncrypted, r.ValueNonce)
		if err != nil {
			return nil, fmt.Errorf("decrypt env var %s: %w", r.KeyName, err)
		}
		env[r.KeyName] = string(plaintext)
	}
	return env, nil
}

// pruneOldImages keeps the newest `keep` artifacts for a service and asks
// the executor to prune everything else, best-effort.
func (o *Orchestrator) pruneOldImages(ctx context.Context, serviceID int64, keep int) {
	artifacts, err := o.Store.ListArtifactsForService(ctx, serviceID)
	if err != nil || len(artifacts) <= keep {
		return
	}
	keepTags := make([]string, 0, keep)
	for i := 0; i < keep && i < len(artifacts); i++ {
		keepTags = append(keepTags, artifacts[i].ImageTag)
	}
	if _, err := o.Exec.Prune(ctx, executor.PruneOptions{KeepImageTags: keepTags}); err != nil {
		o.Log.Warn("image prune failed", "service_id", serviceID, "error", err)
	}
}

func lastLines(s string, n int) string {
	lines := []rune(s)
	if len(lines) <= 2000 {
		return s
	}
	return "..." + string(lines[len(lines)-2000:])
}
