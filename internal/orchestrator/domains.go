package orchestrator

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/evanxdsouza/mangrove/internal/executor"
	"github.com/evanxdsouza/mangrove/internal/models"
	"github.com/evanxdsouza/mangrove/internal/portregistry"
	"github.com/evanxdsouza/mangrove/internal/proxy"
	"github.com/evanxdsouza/mangrove/internal/webhook"
)

// AddCustomDomain registers hostname against a deployment.
//
// In the default "auto_tls" mode this is unverified: the caller must
// complete DNS verification (VerifyCustomDomain) before Mangrove will
// program a live Caddy route for it -- see the verification_token comment
// in 0009_custom_domains.sql.
//
// In "port" mode (config.Config.CustomDomainMode -- see its doc comment
// for why) there's no DNS check: a dedicated port is allocated immediately
// and the route goes live in this same call, matching how a deployment's
// own base port already works. A failure to push that route is returned
// directly (not swallowed as best-effort) since, unlike auto_tls's
// verify-then-wait-for-DNS flow, there's no later step that would ever
// retry it -- the caller needs to know now if e.g. nothing's running yet.
func (o *Orchestrator) AddCustomDomain(ctx context.Context, deploymentID int64, hostname string) (models.CustomDomain, error) {
	hostname = normalizeHostname(hostname)
	if err := validateHostname(hostname); err != nil {
		return models.CustomDomain{}, err
	}
	if _, err := o.Store.GetDeployment(ctx, deploymentID); err != nil {
		return models.CustomDomain{}, fmt.Errorf("load deployment: %w", err)
	}

	if o.Config.CustomDomainPortMode() {
		return o.addCustomDomainPortMode(ctx, deploymentID, hostname)
	}

	token, err := webhook.GenerateToken()
	if err != nil {
		return models.CustomDomain{}, fmt.Errorf("generate verification token: %w", err)
	}
	return o.Store.CreateCustomDomain(ctx, deploymentID, hostname, token)
}

func (o *Orchestrator) addCustomDomainPortMode(ctx context.Context, deploymentID int64, hostname string) (models.CustomDomain, error) {
	port, err := portregistry.AllocateForCustomDomain(ctx, o.Store.DB, deploymentID, o.Config.PortRangeMin, o.Config.PortRangeMax)
	if err != nil {
		return models.CustomDomain{}, fmt.Errorf("allocate port: %w", err)
	}

	domain, err := o.Store.CreateCustomDomainPortMode(ctx, deploymentID, hostname, port)
	if err != nil {
		_ = portregistry.Release(ctx, o.Store.DB, port)
		return models.CustomDomain{}, err
	}

	if err := o.pushCustomDomainRoute(ctx, domain); err != nil {
		_ = portregistry.Release(ctx, o.Store.DB, port)
		_ = o.Store.DeleteCustomDomain(ctx, domain.ID)
		return models.CustomDomain{}, fmt.Errorf("route %s to port %d: %w", hostname, port, err)
	}
	return domain, nil
}

// VerifyCustomDomain checks hostname's DNS for a TXT record proving control
// (mangrove-domain-verification=<token>), and on success marks the domain
// verified and immediately programs its Caddy route against the
// deployment's current upstream(s) -- mirroring SetAccessControl's
// go-live-immediately behavior rather than waiting for the next deploy.
func (o *Orchestrator) VerifyCustomDomain(ctx context.Context, domainID int64) (models.CustomDomain, error) {
	domain, err := o.Store.GetCustomDomain(ctx, domainID)
	if err != nil {
		return models.CustomDomain{}, fmt.Errorf("load domain: %w", err)
	}
	// Port-mode domains are verified=1 from creation (CreateCustomDomainPortMode)
	// -- there's no DNS check to run here.
	if domain.Verified {
		return domain, nil
	}

	want := "mangrove-domain-verification=" + domain.VerificationToken
	txts, err := net.LookupTXT(domain.Hostname)
	if err != nil {
		return models.CustomDomain{}, fmt.Errorf("lookup TXT record for %s: %w", domain.Hostname, err)
	}
	found := false
	for _, t := range txts {
		if t == want {
			found = true
			break
		}
	}
	if !found {
		return models.CustomDomain{}, fmt.Errorf("TXT record %q not found on %s yet -- DNS changes can take a few minutes to propagate", want, domain.Hostname)
	}

	if err := o.Store.MarkCustomDomainVerified(ctx, domain.ID); err != nil {
		return models.CustomDomain{}, fmt.Errorf("mark domain verified: %w", err)
	}
	domain.Verified = true

	if err := o.pushCustomDomainRoute(ctx, domain); err != nil {
		o.Log.Warn("verify domain: initial route push failed; will retry on next deploy/restart", "hostname", domain.Hostname, "error", err)
	}
	return domain, nil
}

// RemoveCustomDomain tears down hostname's live Caddy route (if any),
// releases its dedicated port in "port" mode, and deletes the row.
func (o *Orchestrator) RemoveCustomDomain(ctx context.Context, domainID int64) error {
	domain, err := o.Store.GetCustomDomain(ctx, domainID)
	if err != nil {
		return fmt.Errorf("load domain: %w", err)
	}
	if o.Proxy != nil && domain.Verified {
		if domain.RoutingMode == "port" && domain.Port != nil {
			if err := o.Proxy.DeleteRoute(ctx, *domain.Port); err != nil {
				return fmt.Errorf("remove proxy route: %w", err)
			}
		} else if domain.RoutingMode != "port" {
			if err := o.Proxy.DeleteDomainRoute(ctx, domain.Hostname); err != nil {
				return fmt.Errorf("remove proxy route: %w", err)
			}
		}
	}
	if domain.Port != nil {
		if err := portregistry.Release(ctx, o.Store.DB, *domain.Port); err != nil {
			o.Log.Warn("release custom domain port failed", "hostname", domain.Hostname, "port", *domain.Port, "error", err)
		}
	}
	return o.Store.DeleteCustomDomain(ctx, domainID)
}

// pushCustomDomainRoute resolves a domain's deployment's current running
// upstream(s) and pushes/replaces its Caddy route. Best-effort by design
// (callers log-and-continue on error) since a domain route failing to
// refresh shouldn't fail the deploy/restart it rode in on -- the deploy's
// own port-based route is still the source of truth and gets retried on
// the next deploy either way.
func (o *Orchestrator) pushCustomDomainRoute(ctx context.Context, domain models.CustomDomain) error {
	if o.Proxy == nil {
		return nil
	}
	dep, err := o.Store.GetDeployment(ctx, domain.DeploymentID)
	if err != nil {
		return fmt.Errorf("load deployment: %w", err)
	}
	services, err := o.Store.ListServices(ctx, domain.DeploymentID)
	if err != nil {
		return fmt.Errorf("load services: %w", err)
	}
	if len(services) != 1 {
		return fmt.Errorf("custom domains apply to single-service deployments only (compose stack has %d services)", len(services))
	}
	svc := services[0]

	opts := proxy.RouteOptions{}
	if dep.PasswordProtected {
		opts = proxy.RouteOptions{PasswordProtected: true, GateDeploymentID: dep.ID, GatePort: o.Config.APIPort}
	}

	// A static-strategy deployment never runs a container (Caddy serves
	// its build output directly -- see DeployStatic) so there's no
	// upstream to reverse-proxy to; route straight to its current build's
	// output directory instead.
	if dep.BuildStrategy == string(executor.StrategyStatic) {
		history, err := o.Store.GetCurrentDeployHistory(ctx, dep.ID)
		if err != nil {
			return fmt.Errorf("load current deploy history: %w", err)
		}
		artifact, err := o.Store.GetArtifactForServiceAtDeployHistory(ctx, history.ID, svc.ID)
		if err != nil {
			return fmt.Errorf("load static output path: %w", err)
		}
		if artifact.OutputPath == "" && !opts.PasswordProtected {
			return fmt.Errorf("deployment %d has no built static output to route %s to", domain.DeploymentID, domain.Hostname)
		}
		if domain.RoutingMode == "port" {
			if domain.Port == nil {
				return fmt.Errorf("port-mode domain %s has no allocated port", domain.Hostname)
			}
			return o.Proxy.PutFileServerRoute(ctx, *domain.Port, artifact.OutputPath, opts)
		}
		return o.Proxy.PutFileServerDomainRoute(ctx, domain.Hostname, artifact.OutputPath, opts)
	}

	ids := serviceContainerIDs(svc)
	upstreams := make([]string, 0, len(ids))
	for _, id := range ids {
		addr, err := o.Exec.ContainerAddr(ctx, id, svc.InternalPort)
		if err != nil {
			o.Log.Warn("resolve container address for custom domain failed", "hostname", domain.Hostname, "container_id", id, "error", err)
			continue
		}
		upstreams = append(upstreams, addr)
	}
	if len(upstreams) == 0 && !opts.PasswordProtected {
		return fmt.Errorf("deployment %d has no running container to route %s to", domain.DeploymentID, domain.Hostname)
	}
	if domain.RoutingMode == "port" {
		if domain.Port == nil {
			return fmt.Errorf("port-mode domain %s has no allocated port", domain.Hostname)
		}
		return o.Proxy.PutRouteMulti(ctx, *domain.Port, upstreams, opts)
	}
	return o.Proxy.PutDomainRoute(ctx, domain.Hostname, upstreams, opts)
}

// reapplyCustomDomains re-pushes every verified custom domain's Caddy route
// for a deployment -- called alongside the deployment's own port-based
// route refresh (deploy swap, restart) since a domain's upstream can move
// exactly when the port-based one does. Best-effort: logs and continues
// rather than failing the deploy/restart that triggered it.
func (o *Orchestrator) reapplyCustomDomains(ctx context.Context, deploymentID int64) {
	if o.Proxy == nil {
		return
	}
	domains, err := o.Store.ListVerifiedCustomDomainsForDeployment(ctx, deploymentID)
	if err != nil {
		o.Log.Warn("list verified custom domains failed", "deployment_id", deploymentID, "error", err)
		return
	}
	for _, d := range domains {
		if err := o.pushCustomDomainRoute(ctx, d); err != nil {
			o.Log.Warn("reapply custom domain route failed", "hostname", d.Hostname, "error", err)
		}
	}
}

func normalizeHostname(h string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(h), "."))
}

// validateHostname is a conservative sanity check, not a full RFC 1123
// parser -- it just needs to reject obviously-wrong input (empty, IPs,
// wildcards, values with a scheme/path/port still attached from a
// copy-pasted URL) before it's persisted and later handed to Caddy's host
// matcher and an ACME challenge.
func validateHostname(h string) error {
	if h == "" {
		return fmt.Errorf("hostname is required")
	}
	if strings.ContainsAny(h, "/: ") {
		return fmt.Errorf("hostname %q looks like a URL, not a bare hostname (no scheme, path, or port)", h)
	}
	if net.ParseIP(h) != nil {
		return fmt.Errorf("hostname must be a domain name, not an IP address")
	}
	if strings.HasPrefix(h, "*.") {
		return fmt.Errorf("wildcard domains are not supported")
	}
	if !strings.Contains(h, ".") {
		return fmt.Errorf("hostname %q is missing a TLD", h)
	}
	return nil
}
