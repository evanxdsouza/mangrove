package orchestrator

import (
	"context"
	"testing"
)

// TestCancelDeploy verifies the in-flight deploy registry: BeginDeploy
// reserves a deployment, a second BeginDeploy is refused, CancelDeploy
// actually cancels the first deploy's context, and EndDeploy releases the
// reservation.
func TestCancelDeploy(t *testing.T) {
	o, _, _ := newTestOrchestrator(t)
	const depID = int64(42)

	ctx, err := o.BeginDeploy(depID)
	if err != nil {
		t.Fatalf("BeginDeploy: %v", err)
	}

	if _, err := o.BeginDeploy(depID); err != ErrDeployInProgress {
		t.Fatalf("expected ErrDeployInProgress on second BeginDeploy, got %v", err)
	}

	if err := o.CancelDeploy(depID); err != nil {
		t.Fatalf("CancelDeploy: %v", err)
	}
	if ctx.Err() != context.Canceled {
		t.Fatalf("expected cancelled context after CancelDeploy, got %v", ctx.Err())
	}

	o.EndDeploy(depID)
	// After EndDeploy the reservation is gone: a fresh BeginDeploy succeeds.
	if _, err := o.BeginDeploy(depID); err != nil {
		t.Fatalf("BeginDeploy after EndDeploy: %v", err)
	}
	o.EndDeploy(depID)

	// Cancelling when nothing is in flight errors.
	if err := o.CancelDeploy(depID); err == nil {
		t.Fatal("expected error cancelling a deploy that isn't in flight")
	}
}
