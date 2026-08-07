package orchestrator

import (
	"context"
	"fmt"

	"github.com/evanxdsouza/mangrove/internal/auth"
	"github.com/evanxdsouza/mangrove/internal/portregistry"
	"github.com/evanxdsouza/mangrove/internal/proxy"
)

// basicAuthUsername is fixed -- Mangrove's password protection is a single
// shared password per deployment (a staging-gate, not a user directory),
// matching the plan's "flip a staging build to password-protected without
// touching app code" framing.
const basicAuthUsername = "mangrove"

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

	// Nothing running yet (or Caddy isn't wired up) -- the setting takes
	// effect on the next deploy instead of live.
	if o.Proxy == nil || svc.ContainerIDCurrent == "" {
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
	}

	addr, err := o.Exec.ContainerAddr(ctx, svc.ContainerIDCurrent, svc.InternalPort)
	if err != nil {
		return fmt.Errorf("resolve running container address: %w", err)
	}
	opts := proxy.RouteOptions{PasswordProtected: passwordProtected, Username: basicAuthUsername, BcryptHash: passwordHash}
	if err := o.Proxy.PutRoute(ctx, *hostPort, addr, opts); err != nil {
		return fmt.Errorf("update proxy route: %w", err)
	}
	return nil
}
