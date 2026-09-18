package orchestrator

import (
	"context"
	"errors"
	"fmt"
)

// ErrDeployInProgress is returned by BeginDeploy/WithInflightDeploy when a
// deploy is already running for the deployment. Callers map it to a 409.
var ErrDeployInProgress = errors.New("a deploy is already in progress for this deployment")

// BeginDeploy reserves a deployment for an in-flight deploy, returning a
// cancellable context to run the deploy with. It refuses (error) if a
// deploy is already running for that deployment -- the caller should treat
// that as a conflict rather than queueing a second one. The returned
// context is background-derived (not tied to any HTTP request) so a deploy
// survives its triggering request disconnecting, and is cancelled by
// CancelDeploy. Callers must defer EndDeploy once the deploy finishes.
func (o *Orchestrator) BeginDeploy(deploymentID int64) (context.Context, error) {
	o.inflightMu.Lock()
	defer o.inflightMu.Unlock()
	if o.inflight == nil {
		o.inflight = map[int64]*inflightDeploy{}
	}
	if _, ok := o.inflight[deploymentID]; ok {
		return nil, ErrDeployInProgress
	}
	ctx, cancel := context.WithCancel(context.Background())
	o.inflight[deploymentID] = &inflightDeploy{cancel: cancel, done: make(chan struct{})}
	return ctx, nil
}

// EndDeploy releases the in-flight reservation for a deployment. Safe to
// call even if BeginDeploy wasn't (e.g. a caller that only started a deploy
// via the webhook path never registers); it's a no-op then.
func (o *Orchestrator) EndDeploy(deploymentID int64) {
	o.inflightMu.Lock()
	entry, ok := o.inflight[deploymentID]
	delete(o.inflight, deploymentID)
	o.inflightMu.Unlock()
	if ok {
		close(entry.done)
	}
}

// CancelDeploy aborts the in-flight deploy (if any) for a deployment by
// cancelling its context. The running deploy pipeline observes the
// cancellation and marks its deploy_history failed. Returns an error if no
// deploy is currently in progress.
func (o *Orchestrator) CancelDeploy(deploymentID int64) error {
	o.inflightMu.Lock()
	entry, ok := o.inflight[deploymentID]
	o.inflightMu.Unlock()
	if !ok {
		return fmt.Errorf("no deploy is currently in progress for this deployment")
	}
	entry.cancel()
	return nil
}

// awaitNoInflightDeploy cancels and waits for any in-flight deploy of
// deploymentID to actually return before letting the caller proceed. A
// delete racing a still-running Deploy()/DeployCompose()/DeployStatic() is a
// real hazard, not a theoretical one: without this, a deploy that's already
// past the point of starting a new container can finish *after* delete has
// torn everything down and removed the DB rows, leaving a container (and,
// for a public service, a proxy route) nothing will ever reference or clean
// up again. Waiting on entry.done (closed by EndDeploy once the deploy
// function returns, whether it succeeded, failed, or was cancelled) means
// delete's own teardown always runs strictly after the deploy's -- no
// no-op if none is in progress.
func (o *Orchestrator) awaitNoInflightDeploy(deploymentID int64) {
	o.inflightMu.Lock()
	entry, ok := o.inflight[deploymentID]
	o.inflightMu.Unlock()
	if !ok {
		return
	}
	entry.cancel()
	<-entry.done
}

// WithInflightDeploy runs fn while reserving a deployment against
// concurrent deploys and allowing cancellation -- the shared wrapper for
// the API deploy-trigger handlers. It begins a cancellable context, runs
// fn (which performs the deploy), and always releases the reservation.
// It returns the BeginDeploy conflict error up front if a deploy is already
// running, before fn is ever called.
func (o *Orchestrator) WithInflightDeploy(deploymentID int64, fn func(ctx context.Context) error) error {
	ctx, err := o.BeginDeploy(deploymentID)
	if err != nil {
		return err
	}
	defer o.EndDeploy(deploymentID)
	return fn(ctx)
}
