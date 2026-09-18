package orchestrator

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
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

// TestAwaitNoInflightDeployWaitsForCompletion guards against a delete racing
// a still-running deploy: awaitNoInflightDeploy must not return until the
// deploy's own goroutine has actually finished (EndDeploy called), not just
// been asked to stop -- otherwise DeleteDeployment's teardown can run before
// the deploy is done creating/tearing down its own containers, orphaning
// whatever it creates after delete has already removed the DB rows.
func TestAwaitNoInflightDeployWaitsForCompletion(t *testing.T) {
	o, _, _ := newTestOrchestrator(t)
	const depID = int64(7)

	ctx, err := o.BeginDeploy(depID)
	if err != nil {
		t.Fatalf("BeginDeploy: %v", err)
	}

	var finishedTeardown atomic.Bool
	deployDone := make(chan struct{})
	go func() {
		defer close(deployDone)
		<-ctx.Done() // simulates the deploy pipeline observing cancellation
		time.Sleep(20 * time.Millisecond) // simulates its own cleanup work
		finishedTeardown.Store(true)
		o.EndDeploy(depID)
	}()

	o.awaitNoInflightDeploy(depID)

	if !finishedTeardown.Load() {
		t.Fatal("awaitNoInflightDeploy returned before the in-flight deploy finished its own cleanup")
	}
	<-deployDone

	// No-op when nothing is in flight.
	o.awaitNoInflightDeploy(depID)
}
