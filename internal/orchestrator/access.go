package orchestrator

import (
	"context"
	"fmt"

	"github.com/evanxdsouza/mangrove/internal/auth"
	"github.com/evanxdsouza/mangrove/internal/executor"
	"github.com/evanxdsouza/mangrove/internal/portregistry"
	"github.com/evanxdsouza/mangrove/internal/proxy"
)

// SetAccessControl updates a deployment's public/internal-only and
// password-protection settings, and -- if the deployment currently has a
// running container -- immediately re-pushes (or removes) its Caddy route
// so the change takes effect without a redeploy. Enabling password
// protection always requires a password in the same call; there is no
// "keep the old password" path, which keeps this handler from needing to
// read back a hash it can't safely re-display anyway.
func (o *Orchestrator) SetAccessControl(ctx context.Context, deploymentID int64, isPublic, passwordProtected bool, password string) error {
	if passwordProtected && password == "" {
		return fmt.Errorf("password is required to enable password protection")
	}

	var passwordHash string
	if passwordProtected {
		hash, err := auth.HashPasswordBcrypt(password)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}
		passwordHash = hash
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
		return fmt.Errorf("access control toggles apply to single-service deployments only (compose stack has %d services)", len(services))
	}
	svc := services[0]

	if err := o.Store.SetDeploymentAccessControl(ctx, dep.ID, isPublic, passwordProtected, passwordHash); err != nil {
		return fmt.Errorf("update deployment: %w", err)
	}
	if err := o.Store.UpdateServiceInternalOnly(ctx, svc.ID, !isPublic); err != nil {
		return fmt.Errorf("update service: %w", err)
	}

	if o.Proxy == nil {
		return nil
	}
	// A verified custom domain's route is independent of whether this
	// deployment even has a port-based route to push below (e.g. nothing
	// running yet) -- always give it a chance to pick up the new
	// password-protection state.
	defer o.reapplyCustomDomains(ctx, dep.ID)

	// Nothing running yet -- the port-based route's setting takes effect on
	// the next deploy instead of live.
	if svc.ContainerIDCurrent == "" {
		return nil
	}

	if !isPublic {
		if svc.HostPort != nil {
			if err := o.Proxy.DeleteRoute(ctx, *svc.HostPort); err != nil {
				return fmt.Errorf("remove proxy route: %w", err)
			}
		}
		return nil
	}

	// Going public on a deployment that was created internal-only (and so
	// never had a port allocated) needs one now -- otherwise this toggle
	// would silently do nothing until the next deploy, which defeats "flip
	// a staging build to password-protected without touching app code."
	hostPort := svc.HostPort
	if hostPort == nil {
		p, err := portregistry.AllocateForService(ctx, o.Store.DB, svc.ID, o.Config.PortRangeMin, o.Config.PortRangeMax)
		if err != nil {
			return fmt.Errorf("allocate port: %w", err)
		}
		hostPort = &p
		o.notifyAccessChanged(ctx, dep, *hostPort)
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
	opts := proxy.RouteOptions{PasswordProtected: passwordProtected, GateDeploymentID: dep.ID, GatePort: o.Config.APIPort}
	if err := o.Proxy.PutRouteMulti(ctx, *hostPort, upstreams, opts); err != nil {
		return fmt.Errorf("update proxy route: %w", err)
	}
	return nil
}

// GateUpstreams resolves where a password-protected deployment's traffic
// should actually go once the gate has let a request through: either one or
// more "host:port" reverse-proxy upstreams for a running container, or a
// static build's output directory. Exactly one of the two return values is
// non-empty on success. Mirrors pushCustomDomainRoute's own resolution
// logic in domains.go, which needs the same thing to program a custom
// domain's Caddy route.
func (o *Orchestrator) GateUpstreams(ctx context.Context, deploymentID int64) (upstreams []string, staticRoot string, err error) {
	dep, err := o.Store.GetDeployment(ctx, deploymentID)
	if err != nil {
		return nil, "", fmt.Errorf("load deployment: %w", err)
	}
	services, err := o.Store.ListServices(ctx, deploymentID)
	if err != nil {
		return nil, "", fmt.Errorf("load services: %w", err)
	}
	if len(services) != 1 {
		return nil, "", fmt.Errorf("gated deployments must be single-service (compose stack has %d services)", len(services))
	}
	svc := services[0]

	if dep.BuildStrategy == string(executor.StrategyStatic) {
		history, err := o.Store.GetCurrentDeployHistory(ctx, dep.ID)
		if err != nil {
			return nil, "", fmt.Errorf("load current deploy history: %w", err)
		}
		artifact, err := o.Store.GetArtifactForServiceAtDeployHistory(ctx, history.ID, svc.ID)
		if err != nil {
			return nil, "", fmt.Errorf("load static output path: %w", err)
		}
		if artifact.OutputPath == "" {
			return nil, "", fmt.Errorf("deployment %d has no built static output", deploymentID)
		}
		return nil, artifact.OutputPath, nil
	}

	ids := serviceContainerIDs(svc)
	upstreams = make([]string, 0, len(ids))
	for _, id := range ids {
		addr, err := o.Exec.ContainerAddr(ctx, id, svc.InternalPort)
		if err != nil {
			continue
		}
		upstreams = append(upstreams, addr)
	}
	if len(upstreams) == 0 {
		return nil, "", fmt.Errorf("deployment %d has no running container", deploymentID)
	}
	return upstreams, "", nil
}
