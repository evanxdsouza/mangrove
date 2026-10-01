# Idle sleep: auto-stop and auto-wake

A deployment can opt into going to sleep after a period with no traffic
(`sleep_enabled` + `sleep_idle_minutes`, the "Idle sleep" card on a
deployment's Overview tab, `POST /api/deployments/{id}/sleep`), and waking
itself back up the moment the next visitor arrives, seeing a brief "waking
up" page in the meantime instead of connection-refused. Modeled on the
serverless-platform "scale to zero" idea, but opt-in per deployment and
simple: stop/start the same container, not a cold rebuild.

## Why this needed a different route than Stop

`StopDeployment` (the user-facing Stop button) deletes the deployment's
Caddy route entirely -- the honest behavior for "I stopped this on purpose,"
since there's nothing to show a visitor except connection-refused anyway.
Idle sleep needs the opposite: the route has to survive the container being
stopped, pointed at something that can actually serve a visitor and wake the
container back up. That's the same problem password protection already
solved -- make Mangrove itself the thing Caddy routes to, on every request,
instead of the real app directly.

## Reusing the gate, not inventing a second handler

`proxy.RouteOptions` gained a `Sleepable` flag alongside the existing
`PasswordProtected` one. Both flags route through the exact same Caddy
handler (`gateHandler` in `internal/proxy/caddy.go`): its only job is
"forward to Mangrove's own loopback API, tagged with
`X-Mangrove-Gate-Deployment: <id>`" -- that's equally what a sleep-enabled
deployment needs, so there was no reason to build a second indirection
layer. `internal/orchestrator/routeopts.go`'s `routeOptionsFor` is the one
place a deployment's `PasswordProtected`/`SleepEnabled` settings turn into
`RouteOptions`, used by every route-pushing call site (deploy, redeploy,
restart, access-control change, custom domains) so the two flags can't drift
out of sync across those five places the way five separately-maintained
copies of the same branch would.

```
visitor -> Caddy (public route) -> Mangrove's own loopback API -> real app
                                         |
                                         +-- password gate, if protected
                                         +-- sleep check, if sleep-enabled
```

`internal/api/gate.go`'s `handleGatedRequest` checks the password gate
first (if the deployment has one), then calls `proxyOrWake` -- the single
choke point every already-authorized request passes through on its way to
the real app:

- Deployment awake (`status != "sleeping"`) -> bump `last_request_at`
  (throttled to once a minute, so a busy awake deployment isn't writing to
  the DB on every request) and proxy through for real
  (`gateProxyThrough`, the same function the password gate uses).
- Deployment asleep -> serve the "waking up" page (`renderWakingPage`, a
  standalone `html/template` page like the gate's own, with a
  `<meta http-equiv="refresh">` instead of JS) and kick off a background
  wake, deduped per-deployment (`sync.Mutex` via `wakeLockFor`) so several
  visitors landing in the same moment only trigger one restart. The
  background wake runs detached from the request's context (like
  `orchestrator.WithInflightDeploy`'s own deploys) since it must finish even
  if the visitor who triggered it navigates away immediately.

On the browser's next auto-refresh (every 3 seconds), the whole request
cycle repeats: once the background wake has flipped the deployment's status
back to `running`, that reload proxies through for real. No separate
status-polling endpoint was needed -- the page reload itself re-runs the
same status check.

A password-protected **and** sleep-enabled deployment checks the password
first: triggering a wake is a real (if mild) cost, and checking the password
first means an unauthenticated visitor can't repeatedly wake someone else's
gated deployment just by hitting it.

## Putting a deployment to sleep

`internal/scheduler/sleeper.go`'s `Sleeper` ticks every minute (far more
often than `PreviewReaper`'s hourly sweep, since `sleep_idle_minutes` can be
set as low as a few minutes) and calls `Store.ListSleepCandidates` -- every
`running`, `sleep_enabled` deployment whose `last_request_at` is older than
its own `sleep_idle_minutes` (a per-row SQL comparison,
`datetime('now', '-' || sleep_idle_minutes || ' minutes')`). Each candidate
goes through `Orchestrator.SleepDeployment`: the same best-effort
per-container `Exec.Stop` loop `StopDeployment` uses, except it leaves the
Caddy route untouched and sets the deployment's status to a new `sleeping`
value (added to the `status` CHECK constraint via a full-table rebuild,
migration 0016 -- SQLite can't `ALTER` a `CHECK` in place, same as
`0011_custom_domain_port_mode.sql` and `0004_redeploy_trigger.sql` before
it) instead of `stopped`.

`WakeDeployment` is `RestartDeployment` under a name that reads correctly at
its call site: starting a sleeping deployment's containers back up and
starting a deliberately-stopped-then-manually-restarted one are the exact
same operation. A deployment's own detail page also gets a manual "Wake up"
button when its status is `sleeping` (next to the usual Restart/Stop pair),
for waking it ahead of the next real visitor without waiting.

## Enabling sleep doesn't mean "idle since whenever"

`Store.SetDeploymentSleepConfig` resets `last_request_at` to now whenever
`sleep_enabled` flips on. Without that, a deployment that's never actually
been touched by the wake handler (so `last_request_at` is still `NULL`, or
stale from before sleep was ever turned on) would either never qualify as a
candidate (a `NULL` check) or -- worse -- read as having been idle forever
and get swept on the sleeper's very next tick, seconds after someone turned
the feature on. `TouchDeploymentDeployed` (every successful deploy) also
bumps `last_request_at` for the same reason: a fresh deploy shouldn't read
as idle just because no visitor has shown up yet.

## Known limitations

- **Single-service deployments only**, matching `SetAccessControl`'s own
  constraint -- a compose stack has no one canonical container to stop/start
  this way without an orchestration layer idle sleep doesn't have yet.
- **No live route re-push while actually sleeping or never-deployed.**
  `SetSleepConfig` only re-pushes the Caddy route immediately when the
  deployment is currently `running` (mirrors `SetAccessControl`'s own
  live-reapply pattern) -- a sleeping deployment's route is already correctly
  pointed at the wake handler (that's what let it sleep), and one that's
  never been deployed has no live route to touch either way. Both pick up
  the setting next time a route gets pushed.
- **Not exposed in Simple mode or the CLI/TUI/MCP clients** yet -- Simple
  mode's app detail page explains a sleeping app's state in plain language,
  but the toggle itself is technical-mode-only for now.
