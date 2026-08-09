package orchestrator

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/evanxdsouza/mangrove/internal/models"
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
		o.teardownServices(ctx, "delete project", services)
	}

	return o.Store.DeleteProjectCascade(ctx, projectID)
}

// DeleteDeployment tears down one deployment's live resources -- running
// containers, Caddy routes, named volumes, static output directories --
// before removing its DB rows via Store.DeleteDeployment. Mirrors
// DeleteProject's teardown, just scoped to a single deployment.
func (o *Orchestrator) DeleteDeployment(ctx context.Context, deploymentID int64) error {
	services, err := o.Store.ListServices(ctx, deploymentID)
	if err != nil {
		return fmt.Errorf("load services: %w", err)
	}
	o.teardownServices(ctx, "delete deployment", services)
	return o.Store.DeleteDeployment(ctx, deploymentID)
}

// teardownServices stops/removes each service's live container, deletes
// its Caddy route, removes its named volumes, and removes any static
// build output directories. Best-effort throughout (logged, not fatal) --
// a delete request should still succeed even if e.g. a container was
// already removed out-of-band. logPrefix distinguishes callers in logs.
func (o *Orchestrator) teardownServices(ctx context.Context, logPrefix string, services []models.Service) {
	for _, svc := range services {
		if svc.ContainerIDCurrent != "" {
			if err := o.Exec.Stop(ctx, svc.ContainerIDCurrent, 10*time.Second); err != nil {
				o.Log.Warn(logPrefix+": stop container failed", "service_id", svc.ID, "error", err)
			}
			if err := o.Exec.Remove(ctx, svc.ContainerIDCurrent); err != nil {
				o.Log.Warn(logPrefix+": remove container failed", "service_id", svc.ID, "error", err)
			}
		}
		if o.Proxy != nil && svc.HostPort != nil {
			if err := o.Proxy.DeleteRoute(ctx, *svc.HostPort); err != nil {
				o.Log.Warn(logPrefix+": remove proxy route failed", "service_id", svc.ID, "port", *svc.HostPort, "error", err)
			}
		}

		// Named volumes outlive any single container by design (that's
		// the whole point -- data survives redeploys/rollbacks), so
		// they're never cleaned up by the normal deploy path. Delete
		// them explicitly here or they'd sit on disk forever, which
		// matters on a 16GB box. The container removal above must run
		// first: Docker refuses to remove a volume still in use.
		vols, err := o.Store.ListVolumesForService(ctx, svc.ID)
		if err != nil {
			o.Log.Warn(logPrefix+": list volumes failed", "service_id", svc.ID, "error", err)
			continue
		}
		for _, v := range vols {
			if err := o.Exec.RemoveVolume(ctx, v.DockerVolumeName); err != nil {
				o.Log.Warn(logPrefix+": remove volume failed", "service_id", svc.ID, "volume", v.DockerVolumeName, "error", err)
			}
		}

		// Same reasoning for a static deployment's output directories --
		// they live on disk under StaticSitesDir independent of any
		// container and would otherwise be orphaned.
		artifacts, err := o.Store.ListArtifactsForService(ctx, svc.ID)
		if err != nil {
			o.Log.Warn(logPrefix+": list artifacts failed", "service_id", svc.ID, "error", err)
			continue
		}
		for _, a := range artifacts {
			if a.OutputPath == "" {
				continue
			}
			if err := os.RemoveAll(a.OutputPath); err != nil {
				o.Log.Warn(logPrefix+": remove static output failed", "service_id", svc.ID, "path", a.OutputPath, "error", err)
			}
		}
	}
}
