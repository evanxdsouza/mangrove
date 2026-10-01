package orchestrator

import (
	"github.com/evanxdsouza/mangrove/internal/models"
	"github.com/evanxdsouza/mangrove/internal/proxy"
)

// routeOptionsFor is the one place that turns a deployment's
// password-protection and idle-sleep settings into proxy.RouteOptions --
// every call site that pushes a Caddy route (deploy, redeploy, restart,
// access-control change, custom domains) goes through this instead of
// constructing RouteOptions by hand, so Sleepable can never drift out of
// sync with PasswordProtected across those five places the way two
// separately-maintained copies of the same branch would.
func (o *Orchestrator) routeOptionsFor(dep models.Deployment) proxy.RouteOptions {
	// GateDeploymentID/GatePort are harmless to set even when neither flag
	// below is true -- gateHandler is only ever consulted via
	// needsIndirectRoute(), so a caller that flips PasswordProtected on
	// after calling this (access.go does, for its new not-yet-persisted
	// value) still gets a correctly-addressed route.
	return proxy.RouteOptions{
		PasswordProtected: dep.PasswordProtected,
		Sleepable:         dep.SleepEnabled,
		GateDeploymentID:  dep.ID,
		GatePort:          o.Config.APIPort,
	}
}
