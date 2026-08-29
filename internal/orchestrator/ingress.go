package orchestrator

import (
	"context"

	"github.com/evanxdsouza/mangrove/internal/models"
)

// ingressHostname returns the host a public deployment is reachable at on
// Mangrove's own base domain -- "" if ingress is disabled
// (MANGROVE_BASE_DOMAIN unset). This is the same string already shown as
// the "suggested domain" in deploy-success emails and GitHub commit
// statuses/PR comments (see deploy.go, webhook.go); pushIngressRoute is
// what makes it an actually-live URL instead of a cosmetic one.
func (o *Orchestrator) ingressHostname(dep models.Deployment) string {
	if o.Config.BaseDomain == "" {
		return ""
	}
	return dep.Slug + "." + o.Config.BaseDomain
}

// pushIngressRoute gives a public, single-service deployment a live route
// on the shared srv_public block at <slug>.<BaseDomain> -- the same
// host-matched mechanism internal/orchestrator/domains.go uses for
// user-added custom domains (and the same Caddy automatic-HTTPS
// certificate provisioning), just keyed off Mangrove's own base domain
// instead of a hostname the caller proved ownership of. No DNS
// verification is needed here: unlike a custom domain, BaseDomain's
// wildcard already resolves to this box by construction (a VPS/Nest
// install's DNS, or the DDNS updater on a home install -- see
// setup.sh). Best-effort throughout (logged, not returned) for the same
// reason reapplyCustomDomains is: a route failing to (re)apply shouldn't
// fail the deploy/restart/access-change it rode in on.
func (o *Orchestrator) pushIngressRoute(ctx context.Context, dep models.Deployment) {
	if o.Proxy == nil {
		return
	}
	hostname := o.ingressHostname(dep)
	if hostname == "" {
		return
	}
	// PutDomainRoute (unlike PutRoute/PutRouteMulti) has no
	// password-protection option -- see internal/proxy/caddy.go's
	// RouteOptions -- so publishing it here for a password-protected
	// deployment would silently bypass the password enforced on the
	// port-based route. Refuse rather than carry a new auth-bypass
	// surface: if ingress was already live before password protection got
	// turned on, tear it down instead.
	if dep.PasswordProtected {
		o.removeIngressRoute(ctx, dep)
		return
	}
	services, err := o.Store.ListServices(ctx, dep.ID)
	if err != nil {
		o.Log.Warn("push ingress route: list services failed", "deployment_id", dep.ID, "error", err)
		return
	}
	// Mirrors pushCustomDomainRoute's restriction: a compose stack has no
	// single upstream to point a host-matched route at.
	if len(services) != 1 || services[0].IsInternalOnly {
		return
	}
	svc := services[0]

	ids := serviceContainerIDs(svc)
	upstreams := make([]string, 0, len(ids))
	for _, id := range ids {
		addr, err := o.Exec.ContainerAddr(ctx, id, svc.InternalPort)
		if err != nil {
			o.Log.Warn("push ingress route: resolve container address failed", "deployment_id", dep.ID, "container_id", id, "error", err)
			continue
		}
		upstreams = append(upstreams, addr)
	}
	if len(upstreams) == 0 {
		// No running container to route to yet (e.g. a static-strategy
		// deployment, which has none) -- nothing to do.
		return
	}
	if err := o.Proxy.PutDomainRoute(ctx, hostname, upstreams); err != nil {
		o.Log.Warn("push ingress route failed", "hostname", hostname, "error", err)
	}
}

// removeIngressRoute tears down a deployment's <slug>.<BaseDomain> route, if
// any -- called everywhere a deployment's port-based route is torn down
// (stop, delete, toggled internal-only) so a deployment that's no longer
// serving traffic doesn't keep answering on its ingress hostname either.
// Deleting a route that was never programmed (ingress disabled, or the
// deployment never went public) is a harmless no-op.
func (o *Orchestrator) removeIngressRoute(ctx context.Context, dep models.Deployment) {
	if o.Proxy == nil {
		return
	}
	hostname := o.ingressHostname(dep)
	if hostname == "" {
		return
	}
	if err := o.Proxy.DeleteDomainRoute(ctx, hostname); err != nil {
		o.Log.Warn("remove ingress route failed", "hostname", hostname, "error", err)
	}
}
