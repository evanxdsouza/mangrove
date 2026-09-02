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

## Lifecycle actions short of a full deploy

`internal/orchestrator/lifecycle.go` covers three actions that operate on a
deployment's already-running container(s) instead of building anything:

- **Stop** (`POST /api/deployments/{id}/stop`) runs `docker stop` on every
  service's current container -- it is not removed, so a later Restart is
  fast -- deletes its Caddy route (if any), and marks the deployment
  `stopped`. That status matters beyond display: `Store.SumConfiguredMemoryMB`
  excludes stopped deployments from the admission-control sum `Deploy`
  checks against `MANGROVE_DEPLOYMENT_MEMORY_CEILING_MB`, so stopping an
  idle deployment actually frees budget for others.
- **Restart** (`POST /api/deployments/{id}/restart`) runs `docker restart`
  on the same container ID (also how a stopped deployment is started back
  up -- `docker restart` on an exited container just starts it), then
  re-pushes the Caddy route the same way `SetAccessControl` does after an
  access-control change: re-resolve the container's internal address via
  `Exec.ContainerAddr` and `PutRoute` again, since a restart can hand the
  container a new internal IP even though its ID is unchanged. Unlike
  `Deploy`, this isn't health-check-gated -- it's a direct cycle of the one
  container that's there, not a blue/green swap between two.
- **Redeploy** (`POST /api/deployments/{id}/redeploy`) re-runs the full
  `Deploy`/`DeployCompose`/`DeployStatic` pipeline against whatever source
  the deployment is already configured with, resolving `git_url` and a
  decrypted PAT from its linked `project_repos` row exactly like a webhook
  push does (`internal/api/webhook.go`'s `handlePushEvent`) rather than
  requiring the caller to supply them, which plain `POST .../deploy` still
  does. An image-strategy deployment needs no repo at all; any other
  strategy without a linked repo is rejected with a 422 rather than
  attempting a `git clone ""`.

A one-off command against a service's running container (e.g. a database
migration) is `POST /api/services/{id}/exec`, executed via
`executor.Executor.Exec` (`docker exec`). It runs synchronously and buffers
output in memory -- a fit for a short migration command, not a long-running
or high-volume process.

For anything more than a one-off command there's a full interactive web
terminal: `GET /api/services/{id}/terminal` upgrades to a websocket
(`internal/api/terminal.go`), backed by `executor.Executor.Terminal`
(`internal/executor/docker.go`). Unlike `Exec`, this runs `docker exec -it`
attached to a real host-side pty via
[creack/pty](https://github.com/creack/pty) rather than plain pipes -- that's
what lets window resizes actually reach the container's shell: the docker
CLI only forwards `SIGWINCH` to the exec session when its own stdin/stdout
look like a terminal, and a pty (unlike a pipe) does. The websocket protocol
is deliberately not JSON-wrapped: binary frames carry raw terminal bytes in
both directions (keystrokes in, shell output back), and the one text frame
the client ever sends is a `{"type":"resize"}` control message -- arbitrary
shell output isn't guaranteed to be valid UTF-8, so it can't safely share a
JSON-text channel the way the control message does. The frontend
(`web/src/components/Terminal.tsx`) renders it with
[xterm.js](https://xtermjs.org/), fit to its container via the addon-fit
add-on. The shell itself prefers bash when present, falling back to sh --
resolved with a `command -v bash && exec bash || exec sh` guard rather than
a bare `exec bash || exec sh`, since POSIX sh (what `/bin/sh` actually is on
most base images) exits the whole script the instant a bare `exec` names a
command that isn't found, rather than letting `||` catch it.

## GitHub auto-deploy and staging environments

`internal/api/webhook.go`'s `githubWebhook` is Mangrove's one always-on
public endpoint besides the dashboard itself: `POST /webhooks/github/{token}`,
outside auth (GitHub can't log in), verifying `X-Hub-Signature-256` with a
constant-time HMAC compare before any other side effect, and de-duplicating
by `X-GitHub-Delivery` so a GitHub retry doesn't double-deploy
(`webhook_events.delivery_id` is unique). A `push` event resolves the
branch, looks up every `deployments` row matching
`(project_repo_id, git_branch, auto_deploy_on_push=1)` -- a `project_repos`
link can and does fan out to more than one deployment, which is exactly
what a staging environment is (see below) -- decrypts the linked GitHub
PAT once, and fires one deploy per matching deployment in the background.

Every deploy trigger (manual `POST .../deploy`, `POST .../redeploy`, a
webhook push, staging creation, and promote) ends by calling
`Server.dispatchDeploy`, the single place that picks
`Deploy`/`DeployCompose`/`DeployStatic` based on `build_strategy`. This
used to be a switch duplicated at each call site, and the webhook path's
copy drifted out of sync -- it only special-cased `compose`, so a
static-strategy deployment fell into `Deploy()` (written for
container-based strategies, which tries to run a container from an image a
static build never produces) and failed every time GitHub pushed to it.
Centralizing the switch is what makes that class of bug structurally
harder to reintroduce. The webhook path also now wraps its dispatch in
`Orchestrator.WithInflightDeploy`, the same guard the HTTP trigger
endpoints already used, so a rapid double-push can't start two concurrent
deploys of the same deployment.

Webhook health is visible in the dashboard rather than silently failing:
`project_repos.webhook_registered` records whether GitHub-side
auto-registration actually succeeded (only possible for an OAuth-sourced
credential -- a hand-pasted PAT's scopes aren't known), `GET
.../repo/webhook-instructions` re-shows the callback URL and secret at any
time (they're encrypted at rest, not just returned once), `POST
.../repo/resync-webhook` re-attempts registration, and `GET
.../repo/webhook-events` lists recent deliveries -- whether GitHub reached
Mangrove at all, whether the signature checked out, and whether anything
matched an auto-deploying deployment.

### Staging environments

A staging environment is deliberately *not* new infrastructure: it's just
another `deployments` row. `environment` (`production`|`staging`) and
`promotes_to_deployment_id` (migration `0007`) are the only additions.
`POST /api/deployments/{id}/staging` clones a production deployment's
build config, services, and env vars (a secret env var's ciphertext is
re-sealed under the new service's ID, since its AAD binds to the service
it belongs to -- see `resolveEnv`) onto a new deployment tracking a
caller-chosen branch of the *same* linked repo, sets
`auto_deploy_on_push=1` on it, and deploys it immediately. From there it
behaves exactly like any other deployment -- its own slug/subdomain, its
own history, and the webhook fan-out above auto-deploys it independently
of production whenever that branch gets pushed to.

`POST /api/deployments/{id}/promote` (only valid on a staging deployment)
deploys production with the *exact commit* staging is currently running --
not just production's branch tip, which could be a different commit than
whatever was actually verified in staging. This needed one executor fix:
`git clone --branch <ref>` only resolves a branch or tag name on the
remote, never a raw commit SHA, even though GitHub's smart HTTP protocol
happily serves any reachable commit by SHA. `internal/executor/gitfetch.go`
detects a SHA-shaped `GitRef` and switches to `git init` + `git fetch
<sha>` + `git checkout FETCH_HEAD` instead of `clone --branch` for that
case only. If staging's current deploy has no recorded commit (e.g. it was
last redeployed manually rather than by a push, which doesn't resolve or
record a commit SHA today), promote falls back to whatever `git_ref` was
last recorded.

### PR previews

Built on the same "just another `deployments` row" idea as staging, but
created and torn down automatically instead of by hand.
`pr_previews_enabled` (migration `0008`) opts a production deployment in;
`pr_number` and `github_pr_comment_id` (same migration) mark a deployment
as a preview and record which pull request and bot comment it belongs to.
`CreateWebhook` registers `pull_request` alongside `push`, so
`githubWebhook` gains a second event case: `handlePullRequestEvent`
(`internal/api/webhook.go`) fans out to every `pr_previews_enabled`
production deployment on the repo, same shape as the push handler.

`opened`/`reopened`/`synchronize` call `findOrCreatePreviewDeployment`,
which looks up an existing preview by `(promotes_to_deployment_id,
pr_number)` (a unique index enforces at most one) or clones a new one via
`cloneDeployment` -- the same clone-services-and-env-vars logic
`createStagingDeployment` uses, refactored out so the two don't drift
against each other the way the dispatch switch once did. The clone is then
deployed with the *exact* commit the webhook payload named
(`pull_request.head.sha`), not a branch-tip fetch, so a comment posted
after the deploy finishes always describes the commit that's actually
running. `closed` deletes the preview deployment via
`Orchestrator.DeleteDeployment` (containers, proxy route, port, the row
itself) if one exists -- a PR closed without ever pushing after opt-in has
none.

The PR gets one bot comment, edited in place on every update rather than
reposted (`internal/github/comment.go`'s `CommentClient.UpsertComment`,
POST to create then PATCH to edit using the id `github_pr_comment_id`
records) -- "building" before the deploy, then the preview URL or the
error once `Deploy` (which blocks until build + health check settle)
returns. Best-effort throughout, same convention as the existing
commit-status posting: a GitHub API hiccup here never fails the underlying
deploy or teardown.

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
