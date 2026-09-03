package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/evanxdsouza/mangrove/internal/executor"
	"github.com/evanxdsouza/mangrove/internal/models"
	"github.com/evanxdsouza/mangrove/internal/proxy"
)

// serviceContainerIDs returns the full set of running container IDs for a
// service -- the primary (container_id_current) plus any replicas recorded
// from a replicated deploy -- deduplicated and primary-first.
func serviceContainerIDs(svc models.Service) []string {
	seen := map[string]bool{}
	var out []string
	for _, id := range append([]string{svc.ContainerIDCurrent}, svc.ReplicaContainerIDs...) {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// StopDeployment stops every service's running container without removing
// it, its volumes, or any DB rows -- unlike DeleteDeployment. Marking the
// deployment "stopped" also excludes it from admission control
// (Store.SumConfiguredMemoryMB skips stopped deployments), so stopping an
// idle deployment actually frees budget for others, not just Docker
// resources. RestartDeployment is the inverse.
func (o *Orchestrator) StopDeployment(ctx context.Context, deploymentID int64) error {
	dep, err := o.Store.GetDeployment(ctx, deploymentID)
	if err != nil {
		return fmt.Errorf("load deployment: %w", err)
	}
	services, err := o.Store.ListServices(ctx, deploymentID)
	if err != nil {
		return fmt.Errorf("load services: %w", err)
	}

	stoppedAny := false
	for _, svc := range services {
		ids := serviceContainerIDs(svc)
		if len(ids) == 0 {
			// A static site (no container ever runs) or a service that was
			// never successfully deployed -- nothing to stop.
			continue
		}
		for _, id := range ids {
			if err := o.Exec.Stop(ctx, id, 10*time.Second); err != nil {
				return fmt.Errorf("stop service %q: %w", svc.Name, err)
			}
		}
		stoppedAny = true

		// Remove the Caddy route so a stopped backend doesn't sit there
		// answering with connection-refused/502s -- the route comes back
		// via RestartDeployment. Compose services never had one in the
		// first place (see DeployCompose's doc comment), so HostPort is
		// only ever set here for single-service deployments.
		if o.Proxy != nil && svc.HostPort != nil {
			if err := o.Proxy.DeleteRoute(ctx, *svc.HostPort); err != nil {
				o.Log.Warn("stop deployment: remove proxy route failed", "service_id", svc.ID, "port", *svc.HostPort, "error", err)
			}
		}
		if err := o.Store.UpdateServiceStatus(ctx, svc.ID, "stopped"); err != nil {
			o.Log.Warn("stop deployment: update service status failed", "service_id", svc.ID, "error", err)
		}
	}
	if !stoppedAny {
		return fmt.Errorf("deployment %d has no running container to stop", dep.ID)
	}

	return o.Store.UpdateDeploymentStatus(ctx, dep.ID, "stopped")
}

// RestartDeployment restarts every service's container in place (same
// container ID, same volumes, no rebuild) -- this also starts a container
// previously stopped via StopDeployment. It is a fire-and-cycle action, not
// a health-check-gated swap like Deploy(): a slow-to-boot app is left
// running with whatever status its next health check reports, rather than
// blocking or failing this call.
func (o *Orchestrator) RestartDeployment(ctx context.Context, deploymentID int64) error {
	dep, err := o.Store.GetDeployment(ctx, deploymentID)
	if err != nil {
		return fmt.Errorf("load deployment: %w", err)
	}
	services, err := o.Store.ListServices(ctx, deploymentID)
	if err != nil {
		return fmt.Errorf("load services: %w", err)
	}

	restartedAny := false
	for _, svc := range services {
		ids := serviceContainerIDs(svc)
		if len(ids) == 0 {
			continue
		}
		for _, id := range ids {
			if err := o.Exec.Restart(ctx, id, 10*time.Second); err != nil {
				return fmt.Errorf("restart service %q: %w", svc.Name, err)
			}
		}
		restartedAny = true
		if err := o.Store.UpdateServiceStatus(ctx, svc.ID, "running"); err != nil {
			o.Log.Warn("restart deployment: update service status failed", "service_id", svc.ID, "error", err)
		}

		// Re-push the Caddy route unconditionally: StopDeployment removes it
		// entirely, and even a restart of an already-running container can
		// hand it a new internal IP. Best-effort, mirroring
		// SetAccessControl's same re-push-on-restart pattern. A replicated
		// deployment re-pushes every replica as a load-balanced upstream.
		if o.Proxy != nil && !svc.IsInternalOnly && svc.HostPort != nil && len(ids) > 0 {
			upstreams := make([]string, 0, len(ids))
			for _, id := range ids {
				addr, err := o.Exec.ContainerAddr(ctx, id, svc.InternalPort)
				if err != nil {
					o.Log.Warn("restart deployment: resolve container address failed", "service_id", svc.ID, "container_id", id, "error", err)
					continue
				}
				upstreams = append(upstreams, addr)
			}
			if len(upstreams) > 0 {
				routeOpts := proxy.RouteOptions{}
				if dep.PasswordProtected {
					hash, err := o.Store.GetDeploymentPasswordHash(ctx, dep.ID)
					if err != nil {
						o.Log.Warn("restart deployment: failed to load password hash; route will be unprotected", "deployment_id", dep.ID, "error", err)
					} else {
						routeOpts = proxy.RouteOptions{PasswordProtected: true, Username: basicAuthUsername, BcryptHash: hash}
					}
				}
				if err := o.Proxy.PutRouteMulti(ctx, *svc.HostPort, upstreams, routeOpts); err != nil {
					o.Log.Warn("restart deployment: update proxy route failed", "service_id", svc.ID, "error", err)
				} else {
					o.reapplyCustomDomains(ctx, dep.ID)
				}
			}
		}
	}
	if !restartedAny {
		return fmt.Errorf("deployment %d has no container to restart (never deployed, or a static site)", dep.ID)
	}

	return o.Store.UpdateDeploymentStatus(ctx, dep.ID, "running")
}

// RunServiceCommand executes a one-off command (e.g. a database migration)
// inside a service's currently-running container via `docker exec`. Output
// is buffered in memory and returned whole, not streamed -- fine for a
// migration command's typically-small output, but not a fit for a
// long-running or high-volume process.
func (o *Orchestrator) RunServiceCommand(ctx context.Context, serviceID int64, command []string) (executor.ExecResult, error) {
	if len(command) == 0 {
		return executor.ExecResult{}, fmt.Errorf("command is required")
	}
	svc, err := o.Store.GetService(ctx, serviceID)
	if err != nil {
		return executor.ExecResult{}, fmt.Errorf("load service: %w", err)
	}
	if svc.ContainerIDCurrent == "" {
		return executor.ExecResult{}, fmt.Errorf("service %q has no running container", svc.Name)
	}
	return o.Exec.Exec(ctx, svc.ContainerIDCurrent, command)
}

// OpenServiceTerminal opens an interactive shell session inside a service's
// currently-running container -- the live counterpart to RunServiceCommand,
// staying open for as long as the caller (the web terminal's websocket
// handler) keeps reading/writing it rather than running one command and
// returning.
func (o *Orchestrator) OpenServiceTerminal(ctx context.Context, serviceID int64) (executor.TerminalSession, error) {
	svc, err := o.Store.GetService(ctx, serviceID)
	if err != nil {
		return nil, fmt.Errorf("load service: %w", err)
	}
	if svc.ContainerIDCurrent == "" {
		return nil, fmt.Errorf("service %q has no running container", svc.Name)
	}
	return o.Exec.Terminal(ctx, svc.ContainerIDCurrent)
}
