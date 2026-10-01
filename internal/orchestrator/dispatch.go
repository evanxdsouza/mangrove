package orchestrator

import (
	"context"

	"github.com/evanxdsouza/mangrove/internal/models"
)

// DispatchDeploy routes a deploy request to the right pipeline for a
// deployment's build strategy -- the same switch internal/api/deployments.go's
// dispatchDeploy uses for every API-triggered deploy, pulled out here so
// internal/scheduler/healer.go's automatic retries go through the identical
// routing without the api package's auditing wrapped around it (an
// automated retry isn't a user action to audit-log, just a deploy_history
// row and a log line -- see healer.go).
func (o *Orchestrator) DispatchDeploy(ctx context.Context, dep models.Deployment, req DeployRequest) (int64, error) {
	switch dep.BuildStrategy {
	case "compose":
		return o.DeployCompose(ctx, req)
	case "static":
		return o.DeployStatic(ctx, req)
	default:
		return o.Deploy(ctx, req)
	}
}
