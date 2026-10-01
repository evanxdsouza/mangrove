# Self-healing: auto-restart and auto-retry

A deployment can opt into `self_heal_enabled` (the "Self-healing" card on a
deployment's Overview tab, `POST /api/deployments/{id}/self-heal`), which
gates two independent automated recovery behaviors:

1. A service whose HTTP health check fails repeatedly while the deployment
   is otherwise `running` gets its container restarted automatically.
2. A deploy that fails outright gets retried automatically, with backoff,
   up to a small cap.

Both are opt-in, off by default for every existing deployment -- consistent
with every other automated per-deployment behavior in this codebase (idle
sleep, PR previews, auto-deploy-on-push).

## Why this needed building at all

Docker's own `--restart unless-stopped` policy (every service already gets
this, see `internal/executor/docker.go`) already recovers a container that
actually *crashes* -- the process exits, Docker restarts it. It does nothing
for a container that's still running but has stopped actually working: a
deadlocked app, a connection-pool exhaustion, anything that keeps the
process alive while it stops answering requests. `internal/scheduler/
health.go`'s `HealthChecker` already polled for exactly this (an HTTP health
check against the service's configured path) -- it just never *acted* on a
failing result, only recorded it. Self-healing closes that gap.

Separately, a deploy that fails outright (a flaky build step, a transient
registry pull failure, a health-check timeout during the blue/green swap)
previously just sat as `failed` until a human noticed and clicked
"Redeploy." For a deployment nobody's actively watching, that could be a
long time.

## Health-check-triggered restart

`internal/store`'s `RunnableService` projection (what `HealthChecker`
polls) now also carries the owning deployment's `SelfHealEnabled` flag and
the service's own `ConsecutiveHealthFailures` streak, joined in by
`ListRunningServicesWithHealthCheck` so no second query is needed per
service per tick. `HealthChecker.checkOne` calls `recordFailureStreak`
after every check:

- A failure increments the streak (`Store.SetServiceHealthFailureStreak`); a
  success resets it to 0.
- Once a self-heal-enabled service's streak reaches
  `healthFailureRestartThreshold` (3) **and** its deployment is still
  `running` (not already mid-deploy -- `building`/`healthchecking` belongs
  to `Deploy()`'s own health-check gate, a completely separate mechanism
  this must not interfere with), `HealthChecker` calls
  `Orchestrator.RestartDeployment` and resets the streak to 0 regardless of
  whether the restart itself succeeded.

No separate cooldown timer was needed: each check only happens once per the
service's own `health_check_interval_s`, so the streak itself can't
accumulate toward another restart faster than
`healthFailureRestartThreshold * health_check_interval_s` after the last
one -- a simpler, self-contained rate limit than a second timestamp column
would have been.

`HealthChecker` now takes an `*orchestrator.Orchestrator` (previously a bare
`Store`/`Executor` pair) specifically so it can call `RestartDeployment` --
the same route-re-push-included restart the manual "Restart" button uses,
not a reimplementation.

## Automatic deploy retry

`internal/scheduler/healer.go`'s `Healer` ticks every minute, sweeping
`Store.ListAutoRetryCandidates`: every self-heal-enabled deployment that's
`failed`, hasn't exhausted its retry cap (`auto_retry_count < 2`), and whose
`updated_at` (bumped on every status change, so for a `failed` deployment
this is "when did it most recently fail") is older than that attempt's
backoff -- 1 minute before the first retry, 5 minutes before the second,
computed in SQL (`datetime('now', '-' || (CASE auto_retry_count ...) || '
minutes')`) rather than in application code, so the query and the schedule
can't drift apart.

For each candidate, `Healer.retry`:

1. Increments `auto_retry_count` *before* attempting the deploy -- a retry
   that itself fails must still count toward the cap, or a persistently
   broken deployment would retry forever.
2. Resolves the deployment's currently-configured source via
   `Orchestrator.BuildRedeployRequest` (the exact same source resolution --
   linked repo + decrypted PAT, or just the image ref -- the manual
   "Redeploy" button's `buildRedeployRequest` in `internal/api/
   deployments.go` uses; that function is now a one-line delegate to this
   shared orchestrator method).
3. Dispatches through `Orchestrator.DispatchDeploy` (the build-strategy
   switch -- compose/static/single-service -- pulled out of `internal/api/
   deployments.go`'s own `dispatchDeploy` so the scheduler doesn't need the
   `api` package's auditing wrapped around it; an automated retry isn't a
   user action to audit-log, just a `deploy_history` row and a log line,
   the same convention `Sleeper`/`PreviewReaper` already follow), inside
   `WithInflightDeploy` so it can't race a manual deploy of the same
   deployment.

The retry shows up in the deployment's History tab with its own
`triggered_by` value, `auto-retry` (`deploy_history`'s `CHECK` constraint
extended via a table rebuild, migration `0017` -- SQLite can't `ALTER` a
`CHECK` in place, same pattern `0004_redeploy_trigger.sql` used to add
`redeploy` itself), so it's visibly distinct from a human clicking
"Redeploy."

A successful deploy (`Store.TouchDeploymentDeployed`, called on every
successful `Deploy`/`DeployCompose`/`DeployStatic`) resets `auto_retry_count`
back to 0 -- whether that success came from a human, a webhook push, or the
healer's own retry, the failure streak it was tracking is over.

## Known limitations

- **Fixed thresholds, not configurable per deployment.** The failure count
  (3), retry cap (2), and backoff schedule (1 min, 5 min) are package
  constants, not settings -- matching `Sleeper`'s/`PreviewReaper`'s own
  hardcoded intervals. If these need to be tunable later, they're one flag
  each, not an architecture change.
- **One `self_heal_enabled` flag gates both behaviors.** A deployment can't
  opt into automatic restarts without also getting automatic deploy
  retries, or vice versa -- simpler to reason about and explain as "turn on
  self-healing," at the cost of not letting someone pick just one.
- **Not exposed in Simple mode or the CLI/TUI/MCP clients** yet, matching
  idle sleep's own current scope.
