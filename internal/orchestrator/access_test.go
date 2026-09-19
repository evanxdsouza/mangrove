package orchestrator

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/evanxdsouza/mangrove/internal/store"
)

func seedAccessDeployment(t *testing.T) (*Orchestrator, int64) {
	t.Helper()
	o, st, projectID := newTestOrchestrator(t)
	ctx := context.Background()
	dep, err := st.CreateDeployment(ctx, store.CreateDeploymentParams{
		ProjectID: projectID, Name: "web", Slug: "access-web", BuildStrategy: "dockerfile",
	})
	if err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}
	if _, err := st.CreateService(ctx, store.CreateServiceParams{
		DeploymentID: dep.ID, Name: "web", ContainerName: "mangrove-access-web-web", InternalPort: 3000,
	}); err != nil {
		t.Fatalf("CreateService: %v", err)
	}
	return o, dep.ID
}

func TestSetAccessControlPublicPathsAndKeepPassword(t *testing.T) {
	o, depID := seedAccessDeployment(t)
	ctx := context.Background()

	// Enabling protection still needs a password.
	if err := o.SetAccessControl(ctx, depID, true, true, "", []string{"/pricing"}); err == nil {
		t.Fatal("expected an error enabling protection without a password")
	}

	if err := o.SetAccessControl(ctx, depID, true, true, "s3cret", []string{" /pricing ", "/docs/*", ""}); err != nil {
		t.Fatalf("enable: %v", err)
	}
	dep, _ := o.Store.GetDeployment(ctx, depID)
	if want := []string{"/pricing", "/docs/*"}; !reflect.DeepEqual(dep.PublicPaths, want) {
		t.Errorf("PublicPaths = %v, want %v", dep.PublicPaths, want)
	}
	oldHash, _ := o.Store.GetDeploymentPasswordHash(ctx, depID)

	// Already protected: an empty password keeps the existing hash.
	if err := o.SetAccessControl(ctx, depID, true, true, "", []string{"/pricing"}); err != nil {
		t.Fatalf("edit paths only: %v", err)
	}
	newHash, _ := o.Store.GetDeploymentPasswordHash(ctx, depID)
	if newHash != oldHash {
		t.Error("password hash changed though no new password was supplied")
	}
	dep, _ = o.Store.GetDeployment(ctx, depID)
	if !reflect.DeepEqual(dep.PublicPaths, []string{"/pricing"}) {
		t.Errorf("PublicPaths = %v", dep.PublicPaths)
	}

	// Invalid patterns are rejected and change nothing.
	err := o.SetAccessControl(ctx, depID, true, true, "", []string{"/ok", "no-slash"})
	if err == nil || !strings.Contains(err.Error(), "must start with /") {
		t.Errorf("expected validation error, got %v", err)
	}

	// Turning protection off clears the list and the hash.
	if err := o.SetAccessControl(ctx, depID, true, false, "", []string{"/pricing"}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	dep, _ = o.Store.GetDeployment(ctx, depID)
	if len(dep.PublicPaths) != 0 || dep.PasswordProtected {
		t.Errorf("expected cleared state, got %+v", dep)
	}
	if err := o.SetAccessControl(ctx, depID, true, true, "", nil); err == nil {
		t.Error("re-enabling after disabling must require a new password")
	}
}
