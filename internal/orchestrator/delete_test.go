package orchestrator

import (
	"context"
	"testing"

	"github.com/evanxdsouza/mangrove/internal/store"
)

// TestDeleteDeploymentTearsDownAndCascades asserts DeleteDeployment both
// tells the executor to stop/remove the live container and its volume, and
// actually removes the deployment's rows from the DB -- a store-only
// cascade with no orchestrator teardown would leave the container running;
// an orchestrator teardown with no store cascade would leave the row.
func TestDeleteDeploymentTearsDownAndCascades(t *testing.T) {
	o, st, projectID := newTestOrchestrator(t)
	ctx := context.Background()

	result, err := o.InstallTemplate(ctx, projectID, "postgres", "mydb", nil)
	if err != nil {
		t.Fatalf("InstallTemplate: %v", err)
	}
	depID := result.Deployments[0].DeploymentID

	svcs, err := st.ListServices(ctx, depID)
	if err != nil || len(svcs) != 1 {
		t.Fatalf("ListServices: %v (got %d)", err, len(svcs))
	}
	svc := svcs[0]
	vols, err := st.ListVolumesForService(ctx, svc.ID)
	if err != nil || len(vols) != 1 {
		t.Fatalf("ListVolumesForService: %v (got %d)", err, len(vols))
	}

	if err := o.DeleteDeployment(ctx, depID); err != nil {
		t.Fatalf("DeleteDeployment: %v", err)
	}

	fake := o.Exec.(*fakeTemplateExecutor)
	if len(fake.removedRefs) != 1 || fake.removedRefs[0] != svc.ContainerIDCurrent {
		t.Errorf("expected container %q removed, got %v", svc.ContainerIDCurrent, fake.removedRefs)
	}
	if len(fake.removedVolumes) != 1 || fake.removedVolumes[0] != vols[0].DockerVolumeName {
		t.Errorf("expected volume %q removed, got %v", vols[0].DockerVolumeName, fake.removedVolumes)
	}
	if len(fake.removedImages) != 1 || fake.removedImages[0] != svc.ImageTagCurrent {
		t.Errorf("expected image %q removed, got %v", svc.ImageTagCurrent, fake.removedImages)
	}

	if _, err := st.GetDeployment(ctx, depID); err != store.ErrNotFound {
		t.Errorf("expected deployment row gone, got err=%v", err)
	}
	if remaining, err := st.ListServices(ctx, depID); err != nil || len(remaining) != 0 {
		t.Errorf("expected no services left, got %d (err=%v)", len(remaining), err)
	}
	if remaining, err := st.ListVolumesForService(ctx, svc.ID); err != nil || len(remaining) != 0 {
		t.Errorf("expected no volume rows left, got %d (err=%v)", len(remaining), err)
	}
}

// TestDeleteDeploymentLeavesSiblingDeploymentsIntact guards against an
// overly-broad WHERE clause (e.g. accidentally scoping by project_id
// instead of deployment_id) wiping out unrelated deployments in the same
// project.
func TestDeleteDeploymentLeavesSiblingDeploymentsIntact(t *testing.T) {
	o, st, projectID := newTestOrchestrator(t)
	ctx := context.Background()

	result, err := o.InstallTemplate(ctx, projectID, "wordpress", "blog", nil)
	if err != nil {
		t.Fatalf("InstallTemplate: %v", err)
	}
	if len(result.Deployments) != 2 {
		t.Fatalf("expected 2 deployments, got %d", len(result.Deployments))
	}
	mysqlDepID := result.Deployments[0].DeploymentID
	wpDepID := result.Deployments[1].DeploymentID

	if err := o.DeleteDeployment(ctx, mysqlDepID); err != nil {
		t.Fatalf("DeleteDeployment: %v", err)
	}

	if _, err := st.GetDeployment(ctx, mysqlDepID); err != store.ErrNotFound {
		t.Errorf("expected mysql deployment gone, got err=%v", err)
	}
	if _, err := st.GetDeployment(ctx, wpDepID); err != nil {
		t.Errorf("expected wordpress deployment to survive, got err=%v", err)
	}
}

// TestDeleteProjectTearsDownEveryDeployment asserts the project-level
// delete tears down every service across every deployment in the project,
// not just the first one.
func TestDeleteProjectTearsDownEveryDeployment(t *testing.T) {
	o, st, projectID := newTestOrchestrator(t)
	ctx := context.Background()

	result, err := o.InstallTemplate(ctx, projectID, "wordpress", "blog", nil)
	if err != nil {
		t.Fatalf("InstallTemplate: %v", err)
	}

	if err := o.DeleteProject(ctx, projectID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}

	fake := o.Exec.(*fakeTemplateExecutor)
	if len(fake.removedRefs) != 2 {
		t.Errorf("expected 2 containers removed (one per deployment), got %d: %v", len(fake.removedRefs), fake.removedRefs)
	}
	for _, d := range result.Deployments {
		if _, err := st.GetDeployment(ctx, d.DeploymentID); err != store.ErrNotFound {
			t.Errorf("expected deployment %d gone, got err=%v", d.DeploymentID, err)
		}
	}
	if _, err := st.GetProject(ctx, projectID); err != store.ErrNotFound {
		t.Errorf("expected project gone, got err=%v", err)
	}
}
