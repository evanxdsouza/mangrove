package orchestrator

import (
	"context"
	"fmt"

	"github.com/evanxdsouza/mangrove/internal/executor"
	"github.com/evanxdsouza/mangrove/internal/models"
	"github.com/evanxdsouza/mangrove/internal/proxy"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// RemoveInterruptedContainers cleans up the containers a deploy had already
// started when a Mangrove restart cut it off. SweepStuckDeploys marks those
// deploys failed, but their new container set (named
// "<service container>-<history id>", plus "-<n>" per extra replica) was
// left running with nothing tracking it, holding its memory until someone
// noticed it in `docker ps`. Call with the deploys ListActiveDeploys
// returned before the sweep.
//
// One case is deliberately left alone: if the service's recorded
// containers are all gone, the interrupted deploy had already swapped
// traffic over and torn the old set down, so its "orphan" is the only
// thing serving the app.
func (o *Orchestrator) RemoveInterruptedContainers(ctx context.Context, interrupted []store.ActiveDeploy) {
	for _, a := range interrupted {
		dep, err := o.Store.GetDeployment(ctx, a.DeploymentID)
		if err != nil {
			continue
		}
		if dep.BuildStrategy == string(executor.StrategyCompose) || dep.BuildStrategy == string(executor.StrategyStatic) {
			continue // no per-deploy containers to orphan
		}
		services, err := o.Store.ListServices(ctx, dep.ID)
		if err != nil {
			continue
		}
		for _, svc := range services {
			if svc.ContainerName == "" {
				continue
			}
			if recorded := serviceContainerIDs(svc); len(recorded) > 0 && !o.anyContainerRunning(ctx, recorded, svc.InternalPort) {
				o.Log.Warn("leaving interrupted deploy's container in place: the previous one is gone, so it may be serving traffic",
					"deployment_id", dep.ID, "deploy_history_id", a.HistoryID, "service", svc.Name)
				continue
			}
			base := fmt.Sprintf("%s-%d", svc.ContainerName, a.HistoryID)
			replicas := max(dep.Replicas, 1)
			for i := 0; i < replicas; i++ {
				name := base
				if i > 0 {
					name = fmt.Sprintf("%s-%d", base, i)
				}
				// Best-effort: most interrupted deploys never got as far
				// as starting a container, so "no such container" is the
				// normal outcome here.
				if err := o.Exec.Remove(ctx, name); err == nil {
					o.Log.Info("removed container left by an interrupted deploy", "container", name, "deployment_id", dep.ID)
				}
			}
		}
	}
}

func (o *Orchestrator) anyContainerRunning(ctx context.Context, ids []string, port int) bool {
	for _, id := range ids {
		if _, err := o.Exec.ContainerAddr(ctx, id, port); err == nil {
			return true
		}
	}
	return false
}

// RestoreMissingRoutes re-pushes the Caddy routes of every live deployment
// whose route Caddy no longer has. Routes exist only in Caddy's running
// config -- nothing re-creates them from the DB -- so a reboot, a
// `systemctl restart caddy`, or a Caddy package upgrade (the stock unit
// starts from the Caddyfile, without --resume) left every deployed app
// unreachable until it was redeployed by hand.
//
// Only missing routes are pushed, and deployments with a deploy in flight
// are skipped, so this can't race a blue/green swap back onto the old
// containers. Returns how many deployments it re-routed.
func (o *Orchestrator) RestoreMissingRoutes(ctx context.Context) (int, error) {
	if o.Proxy == nil {
		return 0, nil
	}
	have, err := o.Proxy.ServerNames(ctx)
	if err != nil {
		// Caddy may have come back with no http app at all.
		if baseErr := o.Proxy.EnsureBaseConfig(ctx); baseErr != nil {
			return 0, err
		}
		if have, err = o.Proxy.ServerNames(ctx); err != nil {
			return 0, err
		}
	}
	deps, err := o.Store.ListAllDeployments(ctx)
	if err != nil {
		return 0, err
	}
	active, err := o.Store.ListActiveDeploys(ctx)
	if err != nil {
		return 0, err
	}
	busy := make(map[int64]bool, len(active))
	for _, a := range active {
		busy[a.DeploymentID] = true
	}

	restored := 0
	for _, dep := range deps {
		if busy[dep.ID] || o.deployInFlight(dep.ID) {
			continue
		}
		switch dep.Status {
		case "running", "sleeping", "failed", "building":
			// "failed"/"building": a failed or in-progress redeploy leaves
			// the previous, still-healthy set serving.
		default:
			continue
		}
		services, err := o.Store.ListServices(ctx, dep.ID)
		if err != nil || len(services) != 1 {
			continue // compose stacks aren't routed through Caddy
		}
		svc := services[0]

		routeMissing := !svc.IsInternalOnly && svc.HostPort != nil && !have[proxy.PortServerName(*svc.HostPort)]
		domainsMissing := o.customDomainRoutesMissing(ctx, dep.ID, have)
		if !routeMissing && !domainsMissing {
			continue
		}
		if routeMissing {
			pushed, err := o.pushDeploymentRoute(ctx, dep, svc)
			if err != nil {
				o.Log.Warn("restore route failed", "deployment_id", dep.ID, "error", err)
				continue
			}
			if !pushed && !domainsMissing {
				continue // nothing live to route to (e.g. never deployed)
			}
		}
		o.reapplyCustomDomains(ctx, dep.ID)
		restored++
	}
	return restored, nil
}

func (o *Orchestrator) deployInFlight(deploymentID int64) bool {
	o.inflightMu.Lock()
	defer o.inflightMu.Unlock()
	_, ok := o.inflight[deploymentID]
	return ok
}

func (o *Orchestrator) customDomainRoutesMissing(ctx context.Context, deploymentID int64, have map[string]bool) bool {
	domains, err := o.Store.ListVerifiedCustomDomainsForDeployment(ctx, deploymentID)
	if err != nil {
		return false
	}
	for _, d := range domains {
		if d.RoutingMode == "port" {
			if d.Port != nil && !have[proxy.PortServerName(*d.Port)] {
				return true
			}
		} else if !have["srv_public"] {
			return true
		}
	}
	return false
}

// pushDeploymentRoute programs a deployment's own port-based route from
// what the DB says is current -- the same route Deploy/DeployStatic push.
// pushed is false when there's nothing live to route to.
func (o *Orchestrator) pushDeploymentRoute(ctx context.Context, dep models.Deployment, svc models.Service) (pushed bool, err error) {
	opts := o.routeOptionsFor(dep)
	if dep.BuildStrategy == string(executor.StrategyStatic) {
		history, err := o.Store.GetCurrentDeployHistory(ctx, dep.ID)
		if err != nil {
			return false, nil // never successfully deployed
		}
		artifact, err := o.Store.GetArtifactForServiceAtDeployHistory(ctx, history.ID, svc.ID)
		if err != nil || artifact.OutputPath == "" {
			return false, nil
		}
		return true, o.Proxy.PutFileServerRoute(ctx, *svc.HostPort, artifact.OutputPath, opts)
	}

	var upstreams []string
	for _, id := range serviceContainerIDs(svc) {
		if addr, err := o.Exec.ContainerAddr(ctx, id, svc.InternalPort); err == nil {
			upstreams = append(upstreams, addr)
		}
	}
	// A sleeping or password-protected deployment routes through the gate,
	// which needs no live upstream (it wakes the container itself).
	if len(upstreams) == 0 && !opts.PasswordProtected && !opts.Sleepable {
		return false, nil
	}
	return true, o.Proxy.PutRouteMulti(ctx, *svc.HostPort, upstreams, opts)
}
