package orchestrator

import (
	"context"
	"testing"

	"github.com/evanxdsouza/mangrove/internal/executor"
)

func TestStopThenRestartDeployment(t *testing.T) {
	o, st, projectID := newTestOrchestrator(t)
	ctx := context.Background()
	fake := o.Exec.(*fakeTemplateExecutor)

	result, err := o.InstallTemplate(ctx, projectID, "postgres", "mydb", nil, nil)
	if err != nil {
		t.Fatalf("InstallTemplate: %v", err)
	}
	depID := result.Deployments[0].DeploymentID

	svcs, err := st.ListServices(ctx, depID)
	if err != nil || len(svcs) != 1 {
		t.Fatalf("ListServices: %v (got %d)", err, len(svcs))
	}
	containerID := svcs[0].ContainerIDCurrent
	if containerID == "" {
		t.Fatalf("expected a running container after install")
	}

	if err := o.StopDeployment(ctx, depID); err != nil {
		t.Fatalf("StopDeployment: %v", err)
	}
	if len(fake.stoppedRefs) != 1 || fake.stoppedRefs[0] != containerID {
		t.Errorf("expected Stop() called with %q, got %v", containerID, fake.stoppedRefs)
	}
	dep, err := st.GetDeployment(ctx, depID)
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}
	if dep.Status != "stopped" {
		t.Errorf("expected deployment status 'stopped', got %q", dep.Status)
	}
	svc, err := st.GetService(ctx, svcs[0].ID)
	if err != nil {
		t.Fatalf("GetService: %v", err)
	}
	if svc.Status != "stopped" {
		t.Errorf("expected service status 'stopped', got %q", svc.Status)
	}
	// Stop must not remove the container -- only a fast restart is possible
	// if the container itself is still there.
	if len(fake.removedRefs) != 0 {
		t.Errorf("expected Stop to leave the container in place, but Remove() was called: %v", fake.removedRefs)
	}

	// Stopping again is idempotent -- `docker stop` succeeds as a no-op on
	// an already-stopped container, and StopDeployment mirrors that rather
	// than treating a repeat call as an error.
	if err := o.StopDeployment(ctx, depID); err != nil {
		t.Errorf("expected a repeat StopDeployment to be a harmless no-op, got: %v", err)
	}

	if err := o.RestartDeployment(ctx, depID); err != nil {
		t.Fatalf("RestartDeployment: %v", err)
	}
	if len(fake.restartedRefs) != 1 || fake.restartedRefs[0] != containerID {
		t.Errorf("expected Restart() called with %q, got %v", containerID, fake.restartedRefs)
	}
	dep, err = st.GetDeployment(ctx, depID)
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}
	if dep.Status != "running" {
		t.Errorf("expected deployment status 'running' after restart, got %q", dep.Status)
	}
	svc, err = st.GetService(ctx, svcs[0].ID)
	if err != nil {
		t.Fatalf("GetService: %v", err)
	}
	if svc.Status != "running" {
		t.Errorf("expected service status 'running' after restart, got %q", svc.Status)
	}
}

func TestRunServiceCommand(t *testing.T) {
	o, st, projectID := newTestOrchestrator(t)
	ctx := context.Background()
	fake := o.Exec.(*fakeTemplateExecutor)
	fake.execResult = executor.ExecResult{Output: "migrated 3 tables\n", ExitCode: 0}

	result, err := o.InstallTemplate(ctx, projectID, "postgres", "mydb", nil, nil)
	if err != nil {
		t.Fatalf("InstallTemplate: %v", err)
	}
	svcs, err := st.ListServices(ctx, result.Deployments[0].DeploymentID)
	if err != nil || len(svcs) != 1 {
		t.Fatalf("ListServices: %v (got %d)", err, len(svcs))
	}

	out, err := o.RunServiceCommand(ctx, svcs[0].ID, []string{"sh", "-c", "migrate up"})
	if err != nil {
		t.Fatalf("RunServiceCommand: %v", err)
	}
	if out.Output != "migrated 3 tables\n" || out.ExitCode != 0 {
		t.Errorf("unexpected result: %+v", out)
	}
	if len(fake.execCalls) != 1 || fake.execCalls[0].ref != svcs[0].ContainerIDCurrent {
		t.Errorf("expected Exec() called against the service's container, got %+v", fake.execCalls)
	}
}

func TestRunServiceCommandRequiresCommand(t *testing.T) {
	o, st, projectID := newTestOrchestrator(t)
	ctx := context.Background()

	result, err := o.InstallTemplate(ctx, projectID, "postgres", "mydb", nil, nil)
	if err != nil {
		t.Fatalf("InstallTemplate: %v", err)
	}
	svcs, _ := st.ListServices(ctx, result.Deployments[0].DeploymentID)

	if _, err := o.RunServiceCommand(ctx, svcs[0].ID, nil); err == nil {
		t.Errorf("expected an error for an empty command")
	}
}
