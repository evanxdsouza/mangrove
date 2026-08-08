package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/evanxdsouza/mangrove/internal/executor"
	"github.com/evanxdsouza/mangrove/internal/github"
	"github.com/evanxdsouza/mangrove/internal/portregistry"
	"github.com/evanxdsouza/mangrove/internal/proxy"
)

// DeployStatic runs one deploy round for a static-strategy deployment: an
// optional build command produces (or a pre-built repo already contains) a
// directory of static assets, which Caddy's file_server then serves
// directly -- there is no image, no container, and so no health-check gate
// or blue/green swap. Rollback skips the build entirely and just repoints
// Caddy at a previous deploy's output directory, mirroring how Deploy()
// reuses a prior image tag.
func (o *Orchestrator) DeployStatic(ctx context.Context, req DeployRequest) (deployHistoryID int64, err error) {
	dep, err := o.Store.GetDeployment(ctx, req.DeploymentID)
	if err != nil {
		return 0, fmt.Errorf("load deployment: %w", err)
	}
	if dep.BuildStrategy != string(executor.StrategyStatic) {
		return 0, fmt.Errorf("deployment %d is not a static deployment (strategy=%s)", dep.ID, dep.BuildStrategy)
	}

	services, err := o.Store.ListServices(ctx, dep.ID)
	if err != nil {
		return 0, fmt.Errorf("load services: %w", err)
	}
	if len(services) != 1 {
		return 0, fmt.Errorf("deployment %d has %d services; DeployStatic() handles exactly 1", dep.ID, len(services))
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

	// Admission control runs unconditionally, same as Deploy() -- a static
	// service's memory_limit_mb is always 0 (no container ever runs), so
	// this is a guaranteed no-op check, not a special case. Kept identical
	// rather than skipped so "a template/static deploy goes through the
	// same admission control as any other deployment" is literally true,
	// not just true in spirit.
	alreadyAllocated, err := o.Store.SumConfiguredMemoryMB(ctx, dep.ID)
	if err != nil {
		return fail(fmt.Errorf("check memory budget: %w", err))
	}
	if alreadyAllocated+svc.MemoryLimitMB > o.Config.DeploymentMemoryCeilingMB {
		return fail(&ErrMemoryCeilingExceeded{
			RequestedMB:        svc.MemoryLimitMB,
			AlreadyAllocatedMB: alreadyAllocated,
			CeilingMB:          o.Config.DeploymentMemoryCeilingMB,
		})
	}

	var outputPath string
	if req.RollbackToDeployHistoryID != nil {
		artifact, err := o.Store.GetArtifactForServiceAtDeployHistory(ctx, *req.RollbackToDeployHistoryID, svc.ID)
		if err != nil {
			return fail(fmt.Errorf("load rollback target artifact: %w", err))
		}
		if artifact.OutputPath == "" {
			return fail(fmt.Errorf("rollback target deploy history %d has no recorded output directory", *req.RollbackToDeployHistoryID))
		}
		outputPath = artifact.OutputPath
	} else {
		buildSpec := executor.BuildSpec{
			Strategy: executor.StrategyStatic,
			Context: executor.ContextSource{
				GitURL:    req.GitURL,
				GitRef:    req.GitRef,
				AuthToken: req.AuthToken,
			},
			RootPath:           dep.RootPath,
			StaticBuildCommand: dep.StaticBuildCommand,
			StaticOutputDir:    dep.StaticOutputDir,
			StaticOutputName:   fmt.Sprintf("%s-%d", dep.Slug, historyID),
		}

		var buildLog bytes.Buffer
		result, err := o.Exec.Build(ctx, buildSpec, &buildLog)
		if err != nil {
			return fail(fmt.Errorf("build: %w: %s", err, lastLines(buildLog.String(), 20)))
		}
		outputPath = result.OutputPath
	}

	o.Store.UpdateDeployHistoryStatus(ctx, historyID, "healthchecking", "")

	// Port allocation follows the same rule as Deploy(): only a public
	// service gets one, from the same registry. There's no HostPort on
	// RunSpec here at all -- Caddy serves the directory directly, so
	// there's no container to route to and thus no "usual public port
	// beyond what's already allocated" concern.
	registeredPort := svc.HostPort
	if !svc.IsInternalOnly && registeredPort == nil {
		p, err := portregistry.AllocateForService(ctx, o.Store.DB, svc.ID, o.Config.PortRangeMin, o.Config.PortRangeMax)
		if err != nil {
			return fail(fmt.Errorf("allocate port: %w", err))
		}
		registeredPort = &p
	}

	if o.Proxy != nil && !svc.IsInternalOnly && registeredPort != nil {
		routeOpts := proxy.RouteOptions{}
		if dep.PasswordProtected {
			hash, err := o.Store.GetDeploymentPasswordHash(ctx, dep.ID)
			if err != nil {
				o.Log.Warn("failed to load password hash for password-protected deployment; route will be unprotected", "deployment_id", dep.ID, "error", err)
			} else {
				routeOpts = proxy.RouteOptions{PasswordProtected: true, Username: basicAuthUsername, BcryptHash: hash}
			}
		}
		if err := o.Proxy.PutFileServerRoute(ctx, *registeredPort, outputPath, routeOpts); err != nil {
			return fail(fmt.Errorf("update proxy route: %w", err))
		}
	}

	if err := o.Store.CreateDeployArtifact(ctx, historyID, svc.ID, "", "", "", outputPath); err != nil {
		o.Log.Warn("failed to record deploy artifact", "error", err)
	}
	if err := o.Store.UpdateServiceRuntime(ctx, svc.ID, "", "", "running"); err != nil {
		o.Log.Warn("failed to update service runtime state", "error", err)
	}

	o.Store.UpdateDeployHistoryStatus(ctx, historyID, "success", "")
	o.Store.MarkDeployHistoryCurrent(ctx, dep.ID, historyID)
	o.Store.UpdateDeploymentStatus(ctx, dep.ID, "running")
	o.Store.TouchDeploymentDeployed(ctx, dep.ID)

	o.pruneOldStaticOutputs(ctx, svc.ID, dep.ImageRetentionCount)

	notifyPort := 0
	if registeredPort != nil {
		notifyPort = *registeredPort
	}
	o.notifyDeploySuccess(ctx, dep, notifyPort)
	o.postCommitStatus(ctx, dep, req, github.StateSuccess, "Deployed successfully")

	return historyID, nil
}

// pruneOldStaticOutputs keeps the newest `keep` output directories for a
// service and removes the rest from disk, best-effort -- the static-build
// equivalent of pruneOldImages, run immediately after each successful
// deploy (the scheduler's Pruner sweeps the same way on its own cadence,
// covering directories left behind by a deploy that never reached this
// point, e.g. a crash mid-deploy).
func (o *Orchestrator) pruneOldStaticOutputs(ctx context.Context, serviceID int64, keep int) {
	artifacts, err := o.Store.ListArtifactsForService(ctx, serviceID)
	if err != nil || len(artifacts) <= keep {
		return
	}
	for _, a := range artifacts[keep:] {
		if a.OutputPath == "" {
			continue
		}
		if err := os.RemoveAll(a.OutputPath); err != nil {
			o.Log.Warn("static output prune failed", "service_id", serviceID, "path", a.OutputPath, "error", err)
		}
	}
}
