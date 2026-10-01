package orchestrator

import (
	"context"
	"fmt"
	"time"
)

// SleepDeployment stops every service's container, the same way
// StopDeployment does -- except it deliberately leaves the Caddy route in
// place and sets the deployment's status to "sleeping" instead of
// "stopped". A sleep-enabled deployment's route is always pointed at
// Mangrove's own wake handler (see routeOptionsFor/proxy.RouteOptions.
// Sleepable), which is what lets the next visitor's request wake it back up
// instead of Caddy answering connection-refused the way it would for a
// deliberately-stopped deployment.
//
// Only ever called by the idle-sleep scheduler (internal/scheduler/
// sleeper.go) against a deployment with SleepEnabled -- a user-initiated
// Stop always means StopDeployment, never this.
func (o *Orchestrator) SleepDeployment(ctx context.Context, deploymentID int64) error {
	dep, err := o.Store.GetDeployment(ctx, deploymentID)
	if err != nil {
		return fmt.Errorf("load deployment: %w", err)
	}
	services, err := o.Store.ListServices(ctx, deploymentID)
	if err != nil {
		return fmt.Errorf("load services: %w", err)
	}

	sleptAny := false
	for _, svc := range services {
		ids := serviceContainerIDs(svc)
		if len(ids) == 0 {
			continue
		}
		// Best-effort per container, same reasoning as StopDeployment: one
		// flaky/already-gone replica must not abort the whole call.
		svcStopped := false
		for _, id := range ids {
			if err := o.Exec.Stop(ctx, id, 10*time.Second); err != nil {
				o.Log.Warn("sleep deployment: stop container failed", "service_id", svc.ID, "container_id", id, "error", err)
				continue
			}
			svcStopped = true
		}
		if !svcStopped {
			continue
		}
		sleptAny = true
		if err := o.Store.UpdateServiceStatus(ctx, svc.ID, "stopped"); err != nil {
			o.Log.Warn("sleep deployment: update service status failed", "service_id", svc.ID, "error", err)
		}
	}
	if !sleptAny {
		return fmt.Errorf("deployment %d has no running container to sleep", dep.ID)
	}

	return o.Store.UpdateDeploymentStatus(ctx, dep.ID, "sleeping")
}

// WakeDeployment restarts a sleeping deployment's containers -- the
// counterpart to SleepDeployment, called by the wake handler
// (internal/api/gate.go) when a visitor's request reaches one. It's
// RestartDeployment under a name that reads correctly at that call site:
// the same "start the stopped containers back up, re-push the route" logic
// applies whether a deployment was stopped by SleepDeployment or just
// happens to have containers docker can restart in place.
func (o *Orchestrator) WakeDeployment(ctx context.Context, deploymentID int64) error {
	return o.RestartDeployment(ctx, deploymentID)
}

// SetSleepConfig updates a deployment's idle-sleep opt-in/threshold and, if
// it currently has a running container, immediately re-pushes its Caddy
// route so the change takes effect without a redeploy -- mirrors
// SetAccessControl's own live-reapply pattern in access.go. Applying the
// new route only while actually running keeps this simple: a deployment
// that's mid-sleep already has its route correctly pointed at the wake
// handler (that's what let it go to sleep in the first place, see
// SleepDeployment's doc comment), and one that's never been deployed has no
// live route to touch either way -- both just pick up the new setting
// next time Deploy/RestartDeployment/WakeDeployment pushes a route.
func (o *Orchestrator) SetSleepConfig(ctx context.Context, deploymentID int64, enabled bool, idleMinutes int) error {
	if idleMinutes < 1 {
		idleMinutes = 1
	}
	dep, err := o.Store.GetDeployment(ctx, deploymentID)
	if err != nil {
		return fmt.Errorf("load deployment: %w", err)
	}
	services, err := o.Store.ListServices(ctx, deploymentID)
	if err != nil {
		return fmt.Errorf("load services: %w", err)
	}
	if len(services) != 1 {
		return fmt.Errorf("sleep applies to single-service deployments only (compose stack has %d services)", len(services))
	}
	svc := services[0]

	if err := o.Store.SetDeploymentSleepConfig(ctx, dep.ID, enabled, idleMinutes); err != nil {
		return fmt.Errorf("update deployment: %w", err)
	}

	if o.Proxy == nil || svc.HostPort == nil || dep.Status != "running" {
		return nil
	}

	ids := serviceContainerIDs(svc)
	upstreams := make([]string, 0, len(ids))
	for _, id := range ids {
		addr, err := o.Exec.ContainerAddr(ctx, id, svc.InternalPort)
		if err != nil {
			return fmt.Errorf("resolve running container address: %w", err)
		}
		upstreams = append(upstreams, addr)
	}
	dep.SleepEnabled = enabled // reflect the value just persisted, for routeOptionsFor
	if err := o.Proxy.PutRouteMulti(ctx, *svc.HostPort, upstreams, o.routeOptionsFor(dep)); err != nil {
		return fmt.Errorf("update proxy route: %w", err)
	}
	return nil
}
