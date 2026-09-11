package orchestrator

import (
	"context"
	"testing"

	"github.com/evanxdsouza/mangrove/internal/portregistry"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// newDomainTestDeployment installs a postgres template (fake executor, no
// real Docker/Caddy needed) and returns its single-service deployment ID --
// custom domains apply to single-service deployments only.
func newDomainTestDeployment(t *testing.T, o *Orchestrator, projectID int64) int64 {
	t.Helper()
	result, err := o.InstallTemplate(context.Background(), projectID, "postgres", "domaintest", nil, nil)
	if err != nil {
		t.Fatalf("InstallTemplate: %v", err)
	}
	return result.Deployments[0].DeploymentID
}

// TestAddCustomDomainDefaultModeRequiresVerification is a regression check:
// with CustomDomainMode left at its zero value (not "port"), AddCustomDomain
// must keep its original auto_tls/DNS-verification behavior untouched.
func TestAddCustomDomainDefaultModeRequiresVerification(t *testing.T) {
	o, _, projectID := newTestOrchestrator(t)
	ctx := context.Background()
	depID := newDomainTestDeployment(t, o, projectID)

	domain, err := o.AddCustomDomain(ctx, depID, "app.example.com")
	if err != nil {
		t.Fatalf("AddCustomDomain: %v", err)
	}
	if domain.RoutingMode != "auto_tls" {
		t.Errorf("expected routing_mode 'auto_tls', got %q", domain.RoutingMode)
	}
	if domain.Verified {
		t.Errorf("expected a new auto_tls domain to start unverified")
	}
	if domain.VerificationToken == "" {
		t.Errorf("expected a verification token to be generated")
	}
	if domain.Port != nil {
		t.Errorf("expected no port allocated in auto_tls mode, got %v", *domain.Port)
	}
}

// TestAddCustomDomainPortModeIsLiveImmediately covers the Nest-friendly
// path: MANGROVE_CUSTOM_DOMAIN_MODE=port skips DNS verification entirely
// and allocates a dedicated port from the same pool services use.
func TestAddCustomDomainPortModeIsLiveImmediately(t *testing.T) {
	o, st, projectID := newTestOrchestrator(t)
	o.Config.CustomDomainMode = "port"
	ctx := context.Background()
	depID := newDomainTestDeployment(t, o, projectID)

	domain, err := o.AddCustomDomain(ctx, depID, "app.example.com")
	if err != nil {
		t.Fatalf("AddCustomDomain: %v", err)
	}
	if domain.RoutingMode != "port" {
		t.Errorf("expected routing_mode 'port', got %q", domain.RoutingMode)
	}
	if !domain.Verified {
		t.Errorf("expected a port-mode domain to be verified=true immediately")
	}
	if domain.Port == nil {
		t.Fatalf("expected a port to be allocated")
	}
	if *domain.Port < o.Config.PortRangeMin || *domain.Port > o.Config.PortRangeMax {
		t.Errorf("allocated port %d outside configured range [%d,%d]", *domain.Port, o.Config.PortRangeMin, o.Config.PortRangeMax)
	}

	entries, err := portregistry.List(ctx, st.DB)
	if err != nil {
		t.Fatalf("portregistry.List: %v", err)
	}
	found := false
	for _, e := range entries {
		if e.Port == *domain.Port {
			found = true
			if e.AllocationType != "custom_domain_port" {
				t.Errorf("expected allocation_type 'custom_domain_port', got %q", e.AllocationType)
			}
		}
	}
	if !found {
		t.Errorf("expected port %d to be registered in port_registry", *domain.Port)
	}

	// It should also show up as "verified" for reapply-on-deploy purposes.
	verified, err := st.ListVerifiedCustomDomainsForDeployment(ctx, depID)
	if err != nil {
		t.Fatalf("ListVerifiedCustomDomainsForDeployment: %v", err)
	}
	if len(verified) != 1 || verified[0].ID != domain.ID {
		t.Errorf("expected the port-mode domain to appear in the verified list, got %+v", verified)
	}
}

// TestRemoveCustomDomainPortModeReleasesPort ensures deleting a port-mode
// domain frees its port_registry entry back to the pool, not just the DB
// row -- otherwise every add/remove cycle would leak a port.
func TestRemoveCustomDomainPortModeReleasesPort(t *testing.T) {
	o, st, projectID := newTestOrchestrator(t)
	o.Config.CustomDomainMode = "port"
	ctx := context.Background()
	depID := newDomainTestDeployment(t, o, projectID)

	domain, err := o.AddCustomDomain(ctx, depID, "app.example.com")
	if err != nil {
		t.Fatalf("AddCustomDomain: %v", err)
	}
	port := *domain.Port

	if err := o.RemoveCustomDomain(ctx, domain.ID); err != nil {
		t.Fatalf("RemoveCustomDomain: %v", err)
	}

	if _, err := st.GetCustomDomain(ctx, domain.ID); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound after removal, got %v", err)
	}

	entries, err := portregistry.List(ctx, st.DB)
	if err != nil {
		t.Fatalf("portregistry.List: %v", err)
	}
	for _, e := range entries {
		if e.Port == port {
			t.Errorf("expected port %d to be released, still found: %+v", port, e)
		}
	}
}

// TestVerifyCustomDomainPortModeIsANoOp guards against VerifyCustomDomain
// running its DNS TXT lookup against a port-mode domain, which was never
// given a real verification token to check.
func TestVerifyCustomDomainPortModeIsANoOp(t *testing.T) {
	o, _, projectID := newTestOrchestrator(t)
	o.Config.CustomDomainMode = "port"
	ctx := context.Background()
	depID := newDomainTestDeployment(t, o, projectID)

	domain, err := o.AddCustomDomain(ctx, depID, "app.example.com")
	if err != nil {
		t.Fatalf("AddCustomDomain: %v", err)
	}

	again, err := o.VerifyCustomDomain(ctx, domain.ID)
	if err != nil {
		t.Fatalf("VerifyCustomDomain on an already-verified port-mode domain should be a no-op, got: %v", err)
	}
	if !again.Verified || again.RoutingMode != "port" {
		t.Errorf("unexpected result from VerifyCustomDomain: %+v", again)
	}
}
