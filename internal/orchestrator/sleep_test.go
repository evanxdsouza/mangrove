package orchestrator

import (
	"context"
	"testing"
)

// TestSleepThenWakeDeployment guards the core distinction SleepDeployment
// exists for: unlike StopDeployment, it must leave the deployment in a
// status the wake handler (internal/api/gate.go) can recognize and recover
// from -- "sleeping", not "stopped" -- and WakeDeployment must be able to
// bring it back to "running" the same way RestartDeployment already does.
func TestSleepThenWakeDeployment(t *testing.T) {
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

	if err := o.SleepDeployment(ctx, depID); err != nil {
		t.Fatalf("SleepDeployment: %v", err)
	}
	if len(fake.stoppedRefs) != 1 || fake.stoppedRefs[0] != containerID {
		t.Errorf("expected Stop() called with %q, got %v", containerID, fake.stoppedRefs)
	}
	dep, err := st.GetDeployment(ctx, depID)
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}
	if dep.Status != "sleeping" {
		t.Errorf("expected deployment status 'sleeping' (not 'stopped'), got %q", dep.Status)
	}
	// The container must not be removed -- WakeDeployment restarts it in
	// place, the same assumption RestartDeployment already relies on.
	if len(fake.removedRefs) != 0 {
		t.Errorf("expected Sleep to leave the container in place, but Remove() was called: %v", fake.removedRefs)
	}

	if err := o.WakeDeployment(ctx, depID); err != nil {
		t.Fatalf("WakeDeployment: %v", err)
	}
	if len(fake.restartedRefs) != 1 || fake.restartedRefs[0] != containerID {
		t.Errorf("expected Restart() called with %q, got %v", containerID, fake.restartedRefs)
	}
	dep, err = st.GetDeployment(ctx, depID)
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}
	if dep.Status != "running" {
		t.Errorf("expected deployment status 'running' after wake, got %q", dep.Status)
	}
}

// TestSetSleepConfigPersistsAndResetsLastRequest guards the "enabling sleep
// shouldn't immediately qualify a deployment as idle" behavior: the store
// layer resets last_request_at to now when sleep_enabled flips on (see
// Store.SetDeploymentSleepConfig), so a deployment that's never been
// touched by the wake handler yet doesn't read as having been idle forever.
func TestSetSleepConfigPersistsAndResetsLastRequest(t *testing.T) {
	o, st, projectID := newTestOrchestrator(t)
	ctx := context.Background()

	result, err := o.InstallTemplate(ctx, projectID, "postgres", "mydb", nil, nil)
	if err != nil {
		t.Fatalf("InstallTemplate: %v", err)
	}
	depID := result.Deployments[0].DeploymentID

	if err := o.SetSleepConfig(ctx, depID, true, 15); err != nil {
		t.Fatalf("SetSleepConfig: %v", err)
	}
	dep, err := st.GetDeployment(ctx, depID)
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}
	if !dep.SleepEnabled || dep.SleepIdleMinutes != 15 {
		t.Errorf("expected sleep_enabled=true idle_minutes=15, got enabled=%v idle=%d", dep.SleepEnabled, dep.SleepIdleMinutes)
	}
	if dep.LastRequestAt == nil {
		t.Errorf("expected last_request_at to be set when enabling sleep, got nil")
	}

	// A freshly-enabled deployment must not be an immediate sleep
	// candidate -- that would undo the whole point of resetting
	// last_request_at above.
	candidates, err := st.ListSleepCandidates(ctx)
	if err != nil {
		t.Fatalf("ListSleepCandidates: %v", err)
	}
	for _, c := range candidates {
		if c.ID == depID {
			t.Errorf("freshly sleep-enabled deployment should not be an immediate sleep candidate")
		}
	}
}
