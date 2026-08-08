package orchestrator

import (
	"context"
	"fmt"
	"time"
)

// DeleteProject tears down every live resource a project's deployments
// hold -- running containers and Caddy routes -- before removing the DB
// rows. Store.DeleteProjectCascade alone only knows about SQL; without
// this step deleting a project would orphan running containers and leave
// dangling Caddy routes with no corresponding service in the DB.
// Container/proxy cleanup is best-effort (logged, not fatal) since a
// delete request should still succeed even if e.g. a container was
// already removed out-of-band.
func (o *Orchestrator) DeleteProject(ctx context.Context, projectID int64) error {
	deployments, err := o.Store.ListDeployments(ctx, projectID)
	if err != nil {
		return fmt.Errorf("load deployments: %w", err)
	}

	for _, dep := range deployments {
		services, err := o.Store.ListServices(ctx, dep.ID)
		if err != nil {
			o.Log.Warn("delete project: failed to list services", "deployment_id", dep.ID, "error", err)
			continue
		}
		for _, svc := range services {
			if svc.ContainerIDCurrent != "" {
				if err := o.Exec.Stop(ctx, svc.ContainerIDCurrent, 10*time.Second); err != nil {
					o.Log.Warn("delete project: stop container failed", "service_id", svc.ID, "error", err)
				}
				if err := o.Exec.Remove(ctx, svc.ContainerIDCurrent); err != nil {
					o.Log.Warn("delete project: remove container failed", "service_id", svc.ID, "error", err)
				}
			}
			if o.Proxy != nil && svc.HostPort != nil {
				if err := o.Proxy.DeleteRoute(ctx, *svc.HostPort); err != nil {
					o.Log.Warn("delete project: remove proxy route failed", "service_id", svc.ID, "port", *svc.HostPort, "error", err)
				}
			}
		}
	}

	return o.Store.DeleteProjectCascade(ctx, projectID)
}
