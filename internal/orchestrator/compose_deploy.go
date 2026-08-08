package orchestrator

import (
	"bytes"
	"context"
	"fmt"

	"github.com/evanxdsouza/mangrove/internal/executor"
	"github.com/evanxdsouza/mangrove/internal/github"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// DeployCompose brings up a multi-service stack via `docker compose`. This
// is the pragmatic exception the plan calls out: Compose's own dependency
// ordering and networking semantics aren't reimplemented against the
// single-container Executor interface, so these deployments don't get the
// health-check-gated zero-downtime swap Deploy() provides for single
// services -- `docker compose up -d --build` recreates changed containers
// in place, with whatever brief interruption that implies. Per-service
// CPU/memory caps must be set in the compose file itself (e.g. under
// `deploy.resources.limits`); Mangrove does not inject RunSpec-style caps
// into a compose stack in v1.
func (o *Orchestrator) DeployCompose(ctx context.Context, req DeployRequest) (deployHistoryID int64, err error) {
	dep, err := o.Store.GetDeployment(ctx, req.DeploymentID)
	if err != nil {
		return 0, fmt.Errorf("load deployment: %w", err)
	}
	if dep.BuildStrategy != string(executor.StrategyCompose) {
		return 0, fmt.Errorf("deployment %d is not a compose deployment (strategy=%s)", dep.ID, dep.BuildStrategy)
	}

	historyID, err := o.Store.CreateDeployHistory(ctx, dep.ID, req.TriggeredBy, req.CommitSHA, req.CommitMessage, req.GitRef)
	if err != nil {
		return 0, fmt.Errorf("create deploy_history: %w", err)
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

	spec := executor.ComposeSpec{
		Context:     executor.ContextSource{GitURL: req.GitURL, GitRef: req.GitRef, AuthToken: req.AuthToken},
		RootPath:    dep.RootPath,
		ComposePath: dep.ComposePath,
		ProjectName: fmt.Sprintf("mangrove-%s", dep.Slug),
	}

	var logs bytes.Buffer
	results, err := o.Compose.Up(ctx, spec, &logs)
	if err != nil {
		return fail(fmt.Errorf("compose up: %w: %s", err, lastLines(logs.String(), 20)))
	}

	for _, r := range results {
		svcID, err := o.upsertComposeService(ctx, dep.ID, r)
		if err != nil {
			o.Log.Warn("failed to reconcile compose service", "service", r.ServiceName, "error", err)
			continue
		}
		status := "running"
		if r.State != "running" {
			status = "stopped"
		}
		o.Store.UpdateServiceRuntime(ctx, svcID, spec.ProjectName, r.ContainerID, status)
	}

	o.Store.UpdateDeployHistoryStatus(ctx, historyID, "success", "")
	o.Store.MarkDeployHistoryCurrent(ctx, dep.ID, historyID)
	o.Store.UpdateDeploymentStatus(ctx, dep.ID, "running")
	o.Store.TouchDeploymentDeployed(ctx, dep.ID)

	o.notifyDeploySuccess(ctx, dep, 0) // compose stacks don't have a single "the" port
	o.postCommitStatus(ctx, dep, req, github.StateSuccess, "Deployed successfully")

	return historyID, nil
}

// upsertComposeService finds the services row matching this compose
// service by name, creating it on first deploy -- compose.yml is the
// source of truth for what services exist, not a row a user pre-declares.
func (o *Orchestrator) upsertComposeService(ctx context.Context, deploymentID int64, r executor.ComposeServiceResult) (int64, error) {
	existing, err := o.Store.ListServices(ctx, deploymentID)
	if err != nil {
		return 0, err
	}
	for _, s := range existing {
		if s.Name == r.ServiceName {
			return s.ID, nil
		}
	}

	internalPort, hostPort := 0, 0
	if len(r.Publishers) > 0 {
		internalPort = r.Publishers[0].TargetPort
		hostPort = r.Publishers[0].PublishedPort
	}

	created, err := o.Store.CreateService(ctx, store.CreateServiceParams{
		DeploymentID:   deploymentID,
		Name:           r.ServiceName,
		ContainerName:  r.ContainerName,
		InternalPort:   internalPort,
		IsInternalOnly: hostPort == 0,
	})
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}
