# Architecture

## Request path

```
Browser (React SPA, web/)
  |
  v
chi router (internal/api/router.go)
  |-- /api/*        session-cookie auth (internal/auth), then handlers
  |-- /healthz       unauthenticated liveness check
  |-- /webhooks/github/{token}   unauthenticated, HMAC-verified instead
  |-- /*             embedded SPA (internal/webui, go:embed of web/dist)
  |
  v
internal/api handlers (thin: parse request, call orchestrator/store, encode response)
  |
  v
internal/orchestrator (internal/orchestrator/*.go) -- all the actual logic
  |-- internal/executor    build + run containers (Docker CLI, shelled out to)
  |-- internal/proxy       drive Caddy's admin API for routing
  |-- internal/store       SQLite reads/writes (source of truth)
  |-- internal/portregistry allocate/release the port a public deployment gets
```

SQLite is the single source of truth (WAL mode, `PRAGMA foreign_keys=ON`
for cascade deletes). There is no separate cache or job queue: Docker and
Caddy's actual running state can, in principle, drift from what's in the
DB (e.g. someone `docker rm`s a container by hand), but every write path
goes DB-row-first, so a restart re-reads the DB as ground truth rather
than reconciling against live container state.

## Why chi, why no cgo SQLite, why a hand-rolled frontend router

- **chi**: a thin, idiomatic net/http router with per-route middleware
  chaining (`r.With(middleware)`) -- no reflection-based magic, easy to
  read top-to-bottom in `router.go` to see the whole API surface.
- **modernc.org/sqlite** (no cgo): keeps the Go binary a static,
  single-file artifact with no libsqlite3 dependency to install on the
  target box -- matters for the "small VPS" deployment story in
  [deployment.md](deployment.md), where the binary is just `scp`'d over.
- **Hand-rolled frontend router/state** (`web/src/router.tsx`,
  `userContext.tsx`, `uiMode.tsx`): the dashboard only has a handful of
  views. Pulling in react-router-dom or a state library would be a real
  dependency (and, in react-router-dom's current major version, an
  audit-noise CVE in an RSC code path this SPA never exercises) for
  something a `useState`/`useContext` pair covers in under 40 lines.

## The blue/green deploy flow

`Orchestrator.Deploy` (`internal/orchestrator/deploy.go`) is the core of
the whole system. For a single-service deployment, in order:

1. **Admission control**: sum every other deployment's configured
   `memory_limit_mb` (`Store.SumConfiguredMemoryMB`); if adding this one
   would exceed `MANGROVE_DEPLOYMENT_MEMORY_CEILING_MB`, fail before
   touching Docker at all. This mirrors (and is meant to stay comfortably
   under) `mangrove-deployments.slice`'s hard `MemoryMax` -- see
   [deployment.md](deployment.md).
2. **Build**: run the configured build strategy (`dockerfile`, `nixpacks`,
   `compose`, `image`, or `static`) via `internal/executor`, tagging the
   result `mangrove/<slug>-<service>:<deploy_history_id>`. An `image`
   strategy skips building entirely and uses the given ref directly.
3. **Port allocation** (public deployments only): reserve a host port from
   `internal/portregistry`'s range *before* running the new container --
   but deliberately **not** bound to the container yet (see next step).
4. **Run the new container** alongside the still-live old one, under a new
   per-deploy container name (`<service>-<deploy_history_id>`), on the
   internal Docker network only. Old and new containers coexist during
   this window -- this is the "blue/green" part: nothing public points at
   the new one yet, so a failed health check has zero user-facing impact.
5. **Health-check gate**: poll the new container's configured health path.
   If it never turns healthy within the timeout, stop/remove the new
   container and return failure -- **the old container is left running
   untouched**. A bad deploy never takes down a working one.
6. **Atomic swap**: once healthy, `internal/proxy` PUTs Caddy's route for
   the allocated port to the new container's internal address in one
   request. Caddy applies it atomically -- there is no window where the
   port accepts a connection but nothing answers.
7. **Tear down the old container** only *after* the swap above succeeds --
   ordering matters: repoint traffic first, then remove the thing that was
   serving it.
8. Record the deploy artifact, mark this `deploy_history` row current,
   prune old images past the deployment's configured retention count, and
   (if configured) send a notification email / post a GitHub commit
   status.

A **rollback** (`POST /api/deploy-history/{id}/rollback`) re-runs this
same flow, skipping the build step and reusing the previously-built
artifact's image tag/ID instead -- so rollback goes through the identical
health-check-gated blue/green swap as a forward deploy, not a separate
"just point at the old image" shortcut.

Multi-service deployments (compose strategy, or template-installed stacks
like WordPress+MySQL) go through `DeployCompose`
(`internal/orchestrator/compose_deploy.go`) instead, which is compose-native
rather than doing a per-service blue/green swap.

## Access control at the proxy layer

A public deployment's optional password protection
(`is_public`/`password_protected` on the `deployments` row) is enforced by
Caddy itself, via HTTP Basic Auth configured in the same route PUT that
points at the container (`internal/proxy/caddy.go`'s `RouteOptions`) --
independent of whatever auth the deployed app has, or doesn't have, on its
own.

## Roles

Session validation (`internal/auth`) loads the user's role in the same
query as the session lookup, so role-gating costs no extra DB round trip
per request. See [multi-user.md](multi-user.md) for what each role can do.
