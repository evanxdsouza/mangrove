# Repo map

A navigational entry point for this repo — what lives where, how the pieces
connect, how to build/run/test each of them, and (as of the date below) what's
actually verified working vs. broken. The deep "why" for any of this lives in
the other `docs/*.md` files, linked inline; this file is the index and the
map, not a replacement for them. Re-verify the "Verified status" section
before trusting it — it's a snapshot, not a guarantee.

**If you're an agent working in this repo**: start here before exploring
from scratch, and update this file (directory table, commands, "where to
look" pointers, or the verified-status snapshot) as part of any change that
makes part of it stale — see [CLAUDE.md](../CLAUDE.md).

Last verified: 2026-09-18, updated same-day after an edge-case stability pass, then backup/restore + admin RBAC, then real per-workspace roles (admin/editor/viewer), then an audit log + PR-preview auto-cleanup + resource-usage history -- see the "Verified status" section below.

## What this is

Mangrove: a self-hosted PaaS control plane. Point it at a repo/Dockerfile/
compose file/image, it builds, runs, health-checks, and reverse-proxies it
(blue/green, auto-rollback), with GitHub auto-deploy, staging environments,
PR previews, custom domains, and a one-click template library. See the
[README](../README.md) for the full pitch and
[docs/architecture.md](architecture.md) for the request path and deploy flow.

Single Go module ("mono" in the name): one control-plane binary embeds the
web dashboard, plus three more clients (CLI, TUI, MCP server) that all talk
to the same HTTP API. No separate frontend repo, no microservices.

## Directory-by-directory

| Path | What it is | Depth doc |
|---|---|---|
| `cmd/mangrove/` | The control plane binary — `main.go` wires config, DB, router, scheduler, and starts the HTTP server. Everything else in `internal/` is a library this imports. | [architecture.md](architecture.md) |
| `cmd/mangrovectl/` | Scriptable CLI, predates `internal/apiclient`, has its own hand-rolled HTTP client (`map[string]any`, not typed). Also the `backup` command (streams `GET /api/admin/backup` to a local file). | [clients.md](clients.md), [backup.md](backup.md) |
| `cmd/mangrove-tui/` | Full-screen terminal dashboard (bubbletea). Shares `internal/apiclient`. | [clients.md](clients.md) |
| `cmd/mangrove-mcp/` | MCP server exposing a curated operations subset as tools for an LLM agent. Shares `internal/apiclient`. Two transports: stdio (local clients, e.g. Claude Code) and http (`http.go` -- remote clients like claude.ai's web app, URL-token auth, no OAuth server). | [clients.md](clients.md) |
| `cmd/mangrove-mountd/` | Privileged helper for the storage/NAS feature — mounts/unmounts removable drives so `cmd/mangrove` itself never needs mount capabilities. Separate binary, separate systemd unit, talks to the main process only over a local Unix socket. | [storage.md](storage.md) |
| `internal/mountd/` | The mountd wire protocol (`protocol.go`), the privileged server logic `cmd/mangrove-mountd` runs (`server.go` — device enumeration via `lsblk`, mount/unmount, the root-disk exclusion safety check), and the client `internal/orchestrator` uses to talk to it (`client.go`). | [storage.md](storage.md) |
| `internal/api/` | HTTP handlers (thin) + `router.go` (the whole API surface, chi). Auth middleware applied per-route here. | [architecture.md](architecture.md), [multi-user.md](multi-user.md) |
| `internal/orchestrator/` | The actual logic: `deploy.go` (blue/green flow), `compose_deploy.go`, `deploy_static.go`, `lifecycle.go` (stop/restart/redeploy/scale), `templates.go` (install), `domains.go`, `delete.go`, `cancel.go`. This is where to look first for "how does X actually work." | [architecture.md](architecture.md) |
| `internal/executor/` | Shells out to Docker CLI: build (`docker.go`, `compose.go`), git fetch (`gitfetch.go`), build-strategy detection (`detect.go` — dockerfile/nixpacks/compose/static). | [architecture.md](architecture.md) |
| `internal/proxy/` | Drives Caddy's admin API (`caddy.go`) — route PUT/DELETE, no hand-written Caddyfile. | [architecture.md](architecture.md), [deployment.md](deployment.md) |
| `internal/store/` | SQLite reads/writes — the source of truth. `store.go` is the bulk of it. | [db/migrations](../internal/db/migrations) |
| `internal/db/` | `db.go` (connection, WAL, migration runner) + `migrations/*.sql` (numbered, forward-only). | — |
| `internal/portregistry/` | Allocates/releases host ports for public deployments from `MANGROVE_PORT_RANGE_MIN/_MAX`. | [architecture.md](architecture.md) |
| `internal/auth/` | Password hashing (bcrypt), session cookie issuing/validation, global-role middleware (`RequireAuth`/`RequireOwner`), and per-workspace role middleware (`workspace.go`'s `RequireWorkspaceRole`/`HasWorkspaceRole` — admin/editor/viewer, backed by `workspace_members`). | [multi-user.md](multi-user.md) |
| `internal/gateauth/` | Signed, tamper-evident tokens (sealed under the same master key as `internal/secrets`) backing a password-protected deployment's gate cookie and its cross-domain "continue with your Mangrove account" handoff -- no new DB table. | [protected-deployments.md](protected-deployments.md) |
| `internal/github/` | GitHub OAuth, repo listing, commit-status posting, PR comment upsert. | [architecture.md](architecture.md)#github-auto-deploy |
| `internal/webhook/` | `githubWebhook` HTTP handler's supporting logic — HMAC verify, delivery dedup. (Handler itself is `internal/api/webhook.go`.) | [architecture.md](architecture.md)#github-auto-deploy |
| `internal/templates/` | `templates.go` (loader + `validate()`, panics at `init()` on a bad template) + `data/*.json` (the templates themselves, embedded via `go:embed`). | [templates.md](templates.md) |
| `internal/scheduler/` | Background jobs: `health.go` (deployment health polling), `prune.go` (old image cleanup), `ddns.go` (DuckDNS updater, every 5 min), `preview_reaper.go` (tears down a PR preview with no push in `MANGROVE_PREVIEW_MAX_AGE_HOURS`, backstop for a missed "PR closed" webhook), `resource_sampler.go` (records a resource-usage snapshot every 5 min, pruned to a 30-day rolling window). | [deployment.md](deployment.md)#home-server--ddns |
| `internal/secrets/` | Encryption at rest for secret env vars / PATs (AAD bound to the owning service/PAT row). The master key this all depends on is backed up alongside the DB by `internal/api/backup.go` -- see [backup.md](backup.md). | [backup.md](backup.md) |
| `internal/sysinfo/` | Host/cgroup introspection for the admin resource-budget view. `internal/orchestrator/resource_budget.go`'s `ComputeResourceBudget` is the actual "how much of the box is Mangrove using" computation, shared by the live endpoint and `internal/scheduler`'s periodic sampler. | — |
| `internal/notify/` | Optional Resend email notifications on deploy result. | — |
| `internal/models/` | Shared Go structs — what `internal/store` returns and what `internal/api` serializes. Also what `internal/apiclient` decodes into (see `docs/clients.md` for why that reuse matters). | [clients.md](clients.md) |
| `internal/apiclient/` | Typed HTTP client shared by `mangrove-tui`/`mangrove-mcp`. Cookie-based session, `~/.mangrove/session` on disk. | [clients.md](clients.md) |
| `internal/webui/` | `embed.go` (`go:embed` of `dist/`) serving the built SPA. `dist/` is git-ignored except a `.gitkeep`; it only exists after `cd web && npm run build`. | README Quickstart |
| `internal/config/` | `config.go` — every env var, all optional, defaults documented in the README's config table. | README |
| `web/` | The React 19 SPA. Hand-rolled router/state (`router.tsx`, `userContext.tsx`, `uiMode.tsx`, `workspaceContext.tsx` — the active-workspace "station switcher" state shared by `Layout.tsx` and `ProjectsPage.tsx`, persisted like `uiMode.tsx`) — no react-router, no Redux. `src/pages/` (technical mode) + `src/pages/simple/` (simple mode, see [modes.md](modes.md)). One hand-written design system in `src/styles.css` (CSS custom-property tokens, no Tailwind/MUI) plus an authored SVG icon set in `src/icons.tsx` — the "Field Station / Instrument Room" visual identity, recorded in [DESIGN.md](../DESIGN.md). Self-hosted fonts via `@fontsource/space-grotesk` + `@fontsource/ibm-plex-mono` (latin/latin-ext subsets only, imported in `main.tsx`) — no external font CDN. | [architecture.md](architecture.md)#why-chi-why-no-cgo-sqlite-why-a-hand-rolled-frontend-router, [DESIGN.md](../DESIGN.md) |
| `e2e/` | Playwright suite (`tests/dashboard.spec.ts`) run via `run.sh` against a real (freshly built) Mangrove + real Docker + real Caddy — nothing mocked. | see "Verified status" below |
| `deploy/systemd/` | The actual unit/slice files this project runs in production (`mangrove.service`, two memory-isolating slices, a Caddy drop-in, the optional `mangrove-mountd.service` + its `mangrove-storage-group.conf` drop-in for storage/NAS, and the optional `mangrove-mcp.service` for `cmd/mangrove-mcp`'s http transport, run as its own unprivileged `mangrove-mcp` user). | [deployment.md](deployment.md), [storage.md](storage.md), [clients.md](clients.md) |
| `setup.sh` | One-shot production installer for a fresh Debian/Ubuntu box — installs deps, builds, installs systemd units, prompts for VPS-vs-home (DDNS), creates the admin account. | README |
| `data/` | Runtime SQLite DB + master encryption key for **this box's own local dev/prod instance** (git-ignored, 0700). Not part of the source tree conceptually — don't treat its presence/absence as meaningful to the code. | — |
| `mangrove-static/` | Default `MANGROVE_STATIC_SITES_DIR` — built output for static-strategy deploys on this box. Also runtime state, not source. | — |

## Request path (one paragraph)

Browser → chi router (`internal/api/router.go`) → thin handler
(`internal/api/*.go`) → `internal/orchestrator` (all real logic) →
`internal/executor` (Docker) + `internal/proxy` (Caddy) + `internal/store`
(SQLite, the only source of truth — Docker/Caddy state is never treated as
ground truth). Full detail, including the 8-step blue/green deploy sequence:
[architecture.md](architecture.md).

## Build / run / test, by component

```sh
# Backend: build + unit/integration tests (real SQLite, fake-or-real Docker per package)
go build ./...
go vet ./...
go test ./...

# Frontend: typecheck + production build (writes internal/webui/dist, which
# the Go binary embeds — the backend build is stale for serving the UI until
# this has run at least once)
cd web && npm install && npm run build
npm run lint            # oxlint

# Full stack, local dev
cd web && npm run build && cd ..
go build -o mangrove ./cmd/mangrove
./mangrove               # needs Docker + Caddy admin API at 127.0.0.1:2019 already running

# End-to-end (builds its own binary, its own throwaway SQLite dir, real
# Docker + real Caddy, Playwright-driven, tears itself down on any exit path)
cd e2e && npm install && npx playwright install --with-deps chromium && cd ..
./e2e/run.sh
```

Other clients:

```sh
go build -o mangrovectl ./cmd/mangrovectl
go build -o mangrove-tui ./cmd/mangrove-tui
go build -o mangrove-mcp ./cmd/mangrove-mcp
```

## Where to look for a given kind of change

- **New API endpoint / route**: `internal/api/router.go` (wire it) +
  a handler file in `internal/api/` (thin — parse, call orchestrator/store,
  respond via `respond.go`'s helpers).
- **New deploy behavior**: `internal/orchestrator/deploy.go` (single-service)
  or `compose_deploy.go` (multi-service) — read `architecture.md`'s 8-step
  flow first, this is the most load-bearing file in the repo.
- **New build strategy**: `internal/executor/detect.go` (detection) +
  a new file alongside `docker.go`/`compose.go`.
- **New one-click template**: `internal/templates/data/<key>.json`, no code
  change needed unless it needs a new `generate` kind. Full schema + gotchas:
  [templates.md](templates.md).
- **New env var / config knob**: `internal/config/config.go` + the README's
  config table.
- **New DB column/table**: a new numbered file in `internal/db/migrations/`
  (forward-only, never edit a shipped migration) + the corresponding
  `internal/store/` and `internal/models/` changes.
- **New dashboard page**: `web/src/pages/`, wired into `web/src/router.tsx`
  and `web/src/App.tsx`'s route dispatch. If it needs a simple-mode
  equivalent, see [modes.md](modes.md). Build it from the existing shared
  classes in `web/src/styles.css` (`.card`, `.btn`, `.pill`, `.field`,
  table/`.kv-list` etc.) and icons from `web/src/icons.tsx` rather than
  one-off styles — see [DESIGN.md](../DESIGN.md) for the system.
- **New client-exposed operation** (TUI/MCP): add to `internal/apiclient/`
  first (typed), then the specific client. MCP's tool surface is
  deliberately narrower than the full API — see [clients.md](clients.md)'s
  "Deliberately not exposed" list before adding a destructive MCP tool.
- **Protected deployments (the password/account gate page)**:
  `internal/api/gate.go` + `gate_templates.go` (the gate/handoff/login HTTP
  handlers and their standalone HTML), `internal/gateauth/` (the signed
  tokens), `internal/proxy/caddy.go`'s `gateHandler` (how Caddy routes a
  protected deployment there instead of straight to the app),
  `internal/orchestrator/access.go`'s `GateUpstreams` (how Mangrove finds
  the real app once a visitor is let through). Read
  [protected-deployments.md](protected-deployments.md) first.
- **Storage/NAS (mounting drives, SMB shares)**: `internal/mountd/` (the
  privileged helper's protocol/server/client) and
  `internal/orchestrator/storage.go` (the deployment-shaped share on top of
  it). Read [storage.md](storage.md) first — this is the one place the
  system disk itself is one bug away from being touched, and the doc
  explains exactly where that safety boundary lives and how it's tested.
- **Custom domains (the "Domains" tab)**: `internal/orchestrator/domains.go`
  (`AddCustomDomain`/`VerifyCustomDomain`/`pushCustomDomainRoute`), backed
  by either `internal/proxy/caddy.go`'s shared `srv_public` host-matched
  routes (default `auto_tls` mode — needs `:80`/`:443` actually reachable
  from the internet) or, when `MANGROVE_CUSTOM_DOMAIN_MODE=port`
  (`internal/config/config.go`), a dedicated port from
  `internal/portregistry`'s `AllocateForCustomDomain`, routed exactly like
  a deployment's own base port — the model a box behind Hack Club Nest
  (or anything else that only forwards a registered domain→port pair) has
  to use, since it's never reachable on `:80`/`:443` directly. Read
  [deployment.md](deployment.md#custom-domains) first, specifically the
  "Custom domains on Nest" subsection for the `port` mode.
- **Backup/restore**: `internal/api/backup.go` (`GET /api/admin/backup`,
  owner-only — a WAL-consistent DB snapshot via `VACUUM INTO` plus
  `master.key`, tar+gzipped and streamed) and `cmd/mangrovectl`'s `backup`
  subcommand (streams it to a local file). Read [backup.md](backup.md)
  first — it also covers the restore path and, importantly, what a backup
  does *not* cover (running containers, app data volumes, Caddy's live
  route config).
- **New owner-only route** (host-level, no per-project meaning — user
  accounts, `/admin/*`, `/storage/*`): wrap it in the `auth.RequireOwner`-gated
  group inside `/api/admin` in `internal/api/router.go`, then add it to
  `internal/api/roles_test.go`'s `TestMemberForbiddenFromOwnerOnlyRoutes`
  table and to [multi-user.md](multi-user.md)'s numbered list. Don't assume
  a `/admin/*` route is safe for members by default — see multi-user.md for
  the history here (three routes sat open to any member until a
  2026-09-18 pass closed that gap).
- **New workspace-scoped route** (anything that acts on a project,
  deployment, service, custom domain, or deploy-history entry): wrap it
  with `auth.RequireWorkspaceRole(s.Store, minRole, resolve)` in
  `internal/api/router.go`, using (or adding, if it doesn't exist yet) a
  resolver from `internal/api/workspace_resolvers.go`. Pick `minRole` by
  what the action actually is, not by copying a neighboring route —
  viewer for any GET, editor for anything that deploys/writes non-secret
  state/execs into a container, admin for delete/secrets/access-control
  (see [multi-user.md](multi-user.md)'s "Workspace roles" section for the
  full breakdown and rationale). A route with no single resource ID to
  resolve (e.g. list/create, or one touching two different workspaces at
  once) calls `auth.HasWorkspaceRole` inline instead — see `listProjects`/
  `createProject`/`setProjectWorkspace` in `internal/api/projects.go`/
  `workspaces.go` for the pattern. Add coverage to
  `internal/api/workspace_roles_test.go`, following its existing
  `seedWorkspace`/`setRole` helpers.
- **A new deploy/delete/access-change action that should show up in the
  audit log**: call `internal/api/audit.go`'s `s.audit`/`s.auditCtxWorkspace`
  (route already ran through `RequireWorkspaceRole`) right after the write
  succeeds — see [multi-user.md](multi-user.md)'s "Audit log" section for
  the full vocabulary and which existing handlers already do this as
  examples. **If the action happens inside a deploy** (anything routed
  through `dispatchDeploy`, or a caller of `orchestrator.WithInflightDeploy`
  more generally), do **not** call `auth.UserIDFromContext`/`s.audit` from
  inside that callback — its `ctx` is `WithInflightDeploy`'s own
  `context.WithCancel(context.Background())` (deliberately detached so a
  deploy outlives the HTTP request that started it, see `cancel.go`'s
  `BeginDeploy`), which never carries the original request's session.
  This exact bug shipped and was only caught by live end-to-end testing,
  not any unit test — resolve the actor in the handler via
  `s.resolveActor(r.Context())` and pass it through explicitly instead
  (see `orchestrator.DeployRequest.ActorUserID`/`ActorEmail` and
  `auditDeploy` in `internal/api/deployments.go`, and the regression test
  `TestDispatchDeployAuditsTheRequestActorNotContext` in
  `internal/api/dispatch_test.go`).

## Verified status (2026-09-18, updated same-day for the edge-case stability pass)

Everything below was actually run on this box, not inferred from reading code. The Playwright/manual-QA and e2e rows are carried over unchanged from the 2026-09-07 dashboard-redesign pass (not re-run this time — this pass touches backend logic and status-display fixes, not those page flows); the Go/frontend build+test rows and a new race-detector pass were re-run fresh against this session's fixes.

This pass found and fixed real bugs in production code paths, not just added coverage: a deploy-cancellation bug that leaked containers and stuck deployments at "building"/"healthchecking" forever (`internal/orchestrator/cancel.go`/`deploy.go`/`compose_deploy.go`/`deploy_static.go`), stop/restart aborting on the first failed container instead of best-effort (`internal/orchestrator/lifecycle.go`), a delete-vs-in-flight-deploy race that could orphan a container and proxy route (`internal/orchestrator/delete.go`), a GitHub webhook delivery-dedup TOCTOU that surfaced a raw 500 (prompting needless GitHub retries) instead of an idempotent 200 (`internal/api/webhook.go`, `internal/store/github.go`), an unserialized mountd mount/unmount race (`internal/mountd/server.go`), a `;`-field-injection gap in NAS share creation (`internal/orchestrator/storage.go`), a misleading port-registry note key and an unvalidated `MANGROVE_CUSTOM_DOMAIN_MODE` typo that silently reproduces the "domain pending forever" bug (`internal/portregistry/portregistry.go`, `cmd/mangrove/main.go`), and a stale-error-banner bug in the PR #24 status polling that pinned a transient network error on screen forever (`web/src/pages/*.tsx`).

Two more changes landed the same day, closing gaps flagged by that pass rather than found by it: **backup/restore** went from "no code path at all" to a working `mangrovectl backup` (`internal/api/backup.go` + [backup.md](backup.md)) — verified end-to-end on this box, not just unit-tested: took a live backup, extracted it into an empty scratch data dir, started a second `mangrove` instance against it on a different port, and logged in with the original owner credentials against the restored database. And the **admin RBAC gap** flagged in multi-user.md's own spirit (destructive/system-wide operations should be owner-only) but not actually enforced for three routes is closed: `/admin/sessions`, `/admin/ports`, and `/admin/prune` are now behind `RequireOwner`, matching `/admin/users`, with the dashboard's AdminPage updated to match (no more fetching/rendering those sections, or the prune button, as a member — they'd only 403).

A fourth change, larger than the first three combined: **real per-workspace roles** (admin/editor/viewer, backed by `workspace_members`, which existed in the schema since day one but was never read by anything). Closes the exec/terminal gap directly (`POST /services/{id}/exec` and `GET /services/{id}/terminal` went from "any authenticated user" to editor+ in that service's own workspace) and devolves the four previously-global-owner-only actions (delete project/deployment, set secrets, access control, delete workspace) to workspace-admin, scoped to their own workspace — see [multi-user.md](multi-user.md)'s full breakdown. A migration (`0012_workspace_role_backfill.sql`) backfills every existing (workspace, user) pair on upgrade so no existing install loses access it already had. `internal/auth/workspace.go`, `internal/api/workspace_resolvers.go`, and `internal/store/workspace_roles.go` are the new backend surface; `web/src/workspaceContext.tsx`'s `useWorkspaceRole` and a new Members panel on the Workspaces page are the new frontend surface.

A fifth batch, three smaller features landed together: **an audit log** (who deployed/deleted/changed access, and when — `internal/store/audit.go`, `internal/api/audit.go`, migration `0014_audit_log.sql`, never pruned, matters more now that workspace-admins have the reach the fourth change gave them — see multi-user.md's "Audit log" section); **PR-preview auto-cleanup by age** (`internal/scheduler/preview_reaper.go`, `MANGROVE_PREVIEW_MAX_AGE_HOURS`, default a week — a backstop for a missed "PR closed" webhook, which previously left a stale preview eating the deployment-memory admission budget forever); and **resource-usage history** (`internal/scheduler/resource_sampler.go` samples the same live computation the admin dashboard already did every 5 minutes into `resource_usage_snapshots`, kept 30 days — `orchestrator.ComputeResourceBudget` is now the one shared computation both the live endpoint and the sampler call, replacing a copy that used to live only in the handler). This pass also caught and fixed a real bug via live end-to-end testing that no unit test caught: an interactive deploy's audit entry was being mis-attributed to the automated-webhook actor, because `orchestrator.WithInflightDeploy` deliberately runs a deploy under its own context detached from the HTTP request (so a deploy outlives the request that started it) — see the "New deploy/delete/access-change action" bullet above for the full explanation and the regression test that now guards it.

| Check | Command | Result |
|---|---|---|
| Go build | `go build ./...` | ✅ clean, all of `cmd/` + `internal/` |
| Go vet | `go vet ./...` | ✅ clean |
| Go tests | `go test ./...` | ✅ all packages pass, including new regression tests: `TestAwaitNoInflightDeployWaitsForCompletion`, `TestStopDeploymentToleratesOneFailedContainer`, `TestDeleteDeploymentWaitsForInflightDeploy` (orchestrator), webhook-dedup-race coverage in `internal/store/github_test.go`, a NAS share field-injection test in `internal/orchestrator/storage_test.go`, a port-registry note assertion in `domains_test.go`, the workspace-roles suite (`internal/auth/workspace_test.go`, `internal/store/workspace_roles_test.go`, `internal/db/workspace_role_backfill_test.go`, `internal/api/workspace_roles_test.go`), and the newest suite: `internal/scheduler/preview_reaper_test.go` + `resource_sampler_test.go`, `internal/store/resource_usage_test.go` + `audit_test.go`, `internal/api/audit_test.go`, and `TestDispatchDeployAuditsTheRequestActorNotContext` in `internal/api/dispatch_test.go` (the detached-context regression above) |
| Go race detector | `go test -race ./internal/orchestrator/... ./internal/store/... ./internal/mountd/... ./internal/auth/... ./internal/api/... ./internal/db/... ./internal/scheduler/...` | ✅ clean |
| Frontend typecheck + build | `cd web && npm run build` | ✅ `tsc -b` clean, `vite build` succeeds |
| Frontend lint | `cd web && npm run lint` (oxlint) | ✅ clean (only the same pre-existing warnings as before — see "Known issues") |
| Manual QA | throwaway instance (`MANGROVE_DATA_DIR`/`MANGROVE_PORT` against a scratch dir, admin account + sample workspaces/projects/deployments via the API), Playwright screenshots at desktop (1440×900) and mobile (390×844) across every technical- and simple-mode page | ✅ from the 2026-09-07 pass for everything that predates workspace roles; newer UI (Members panel, Activity/Audit-log views, resource-history strips) was instead verified via the live end-to-end passes below (no updated Playwright screenshots this round) |
| Workspace roles end-to-end | live `mangrove` instance, real accounts/workspaces via `curl` against a scratch data dir/port | ✅ see "Workspace roles live verification" below |
| Audit log + resource history end-to-end | live `mangrove` instance, real accounts/workspaces/deploys via `curl` against a scratch data dir/port | ✅ see "Audit log live verification" below — this is the pass that caught the actor-attribution bug |
| mangrove-mcp http transport | standalone smoke program exercising the same `NewStreamableHTTPHandler` + token-gate wiring as `cmd/mangrove-mcp/http.go`, plus a real `mcp.StreamableClientTransport` connect | ✅ wrong token → `404`; correct token → full MCP `initialize` handshake. Startup fail-fast (missing token / bad login) confirmed by running the built binary. Also verified live on this box through Caddy (`:7779`) and Nest's public hostname (correct token → `200`, wrong → `404`); also confirmed working from claude.ai's web custom-connector UI (No sign-in, Streamable HTTP) — see clients.md |
| E2E suite | `./e2e/run.sh` | ❌ fails on test 1 of 6, **not an app bug** — see "Known issues" (unchanged from the prior pass) |

### Workspace roles live verification (2026-09-18)

Ran a real `mangrove` instance (scratch data dir, port 17780, killed and cleaned up afterward) and drove it with `curl`, not mocked:

- Set up an owner, then an owner-created member account, then a second workspace (`Acme`) — confirmed the owner shows `your_role: "admin"` on *both* workspaces in `GET /api/workspaces` without an explicit `workspace_members` row needed for the one they didn't create (the global-owner bypass).
- Seeded a project/deployment/service in the new workspace as owner, then confirmed the member — who has no membership in that workspace at all — gets `403` on `GET` the project, `GET` the deployment, and **`POST .../exec`** (the headline gap this pass closed).
- Added the member as **viewer**: `GET` project → `200`; `POST .../exec` → `403`; `DELETE` project → `403`.
- Promoted to **editor**: `POST .../exec` → `422` (the role check passed and the request reached the real handler, which correctly rejected it for having no running container to exec into — not a `403`); `DELETE` project → `403`; setting a secret env var → `403`.
- Promoted to **admin**: `POST .../access` (access control) → `204`; `DELETE` project → `204` — both real, successful, previously-global-owner-only actions, now working for a workspace-admin who is a global `member`.
- Cross-workspace scoping: the same user, now admin of workspace 2, got `403` deleting a project that lives in workspace 1 — admin doesn't leak across workspaces.
- Membership management: `GET /api/workspaces/2/members` correctly listed both users with their current roles; an unauthenticated request to the same endpoint got `401`.

### Audit log live verification (2026-09-18)

Ran a second real `mangrove` instance the same way (scratch data dir, port 17781, killed and cleaned up afterward) and drove it with `curl`:

- Adding a workspace member and then deleting a project both produced real `audit_log` rows, readable via `GET /api/workspaces/{id}/audit-log`, in the right order (newest first), with the correct `actor_email`, `action`, `resource_type`/`resource_id`, and `workspace_id`.
- `GET /api/workspaces/{id}/audit-log` returned `200` for a workspace member (viewer) reading their own workspace's log; `GET /api/admin/audit-log` correctly `403`'d that same member and `200`'d for the owner, returning every event including an org-level one (`create_user`) with no `workspace_id` at all.
- **Caught a real bug this way that no unit test had**: the first attempt at a real interactive deploy (`POST /deployments/{id}/deploy`, authenticated as the owner) showed up in the audit log attributed to `actor_email: "github-webhook"` instead of the owner — `dispatchDeploy` runs under `orchestrator.WithInflightDeploy`'s own `context.WithCancel(context.Background())` (deliberately detached from the HTTP request so a deploy outlives it), which never carries the request's session. Fixed by resolving the actor in the handler (`s.resolveActor(r.Context())`) before entering that detached context and threading it through explicitly via new `DeployRequest.ActorUserID`/`ActorEmail` fields, rather than reading it back out of `ctx` deep inside the deploy. Re-ran the same live test after the fix: `actor_email: "owner@example.com"`, `actor_user_id: 1`, correct. Regression-tested in `internal/api/dispatch_test.go`'s `TestDispatchDeployAuditsTheRequestActorNotContext` so this can't silently reappear.
- `GET /api/admin/resource-budget` (the refactor that extracted `orchestrator.ComputeResourceBudget` out of the handler) still returns correct live figures — a real disk-usage reading, zeroed memory/container figures on a box with nothing deployed, no regression from the extraction.

### Known issues found

1. **E2E suite fails due to a port collision with this box's own production
   instance, not an app defect.** This machine runs a real, systemd-managed
   Mangrove instance in production (`mangrove.service`, PID visible via `ps`,
   listening on `127.0.0.1:7777` since 2026-09-03 — this box dogfoods its own
   deploys). `e2e/run.sh` hardcodes `PORT=7777` for its own throwaway
   instance. When run on this box, the e2e binary's own `bind()` fails
   (`listen tcp 127.0.0.1:7777: bind: address already in use`) and it exits
   immediately — but `run.sh`'s readiness loop only checks that
   `curl .../healthz` returns 200, which the *real* production instance
   answers just fine. Playwright then drives the real production dashboard,
   which already has an owner account, so test 1
   (`getByText("Welcome to Mangrove")`) fails immediately because "Sign in"
   renders instead — and all 5 subsequent tests in the `describe.serial`
   block never run. Confirmed by manually starting a second instance on 7777
   against a fresh data dir: the log shows the exact bind error above, on
   every attempt, deterministically (verified twice).
   - **Not a risk to production**: the suite never got far enough to touch
     the real instance's data (test 1 fails on the very first assertion,
     before any account creation or deploy).
   - **Real, reproducible weakness in `e2e/run.sh`**: it trusts `/healthz`
     alone as a "my instance is up" signal instead of also confirming its
     own process is still alive (e.g. checking `$MANGROVE_PID` via `kill -0`
     in the wait loop, or picking a random free port instead of a hardcoded
     one). On a clean CI box with nothing on 7777 already, this suite would
     very likely pass — this failure mode is specific to running it
     side-by-side with a live production instance on the same host, which
     is exactly this box's actual setup.
   - To actually verify the SPA/API flows e2e-test, rerun `e2e/run.sh` with
     `PORT` changed to something unused (e.g. edit `e2e/run.sh`'s
     `PORT=7777` line, or parameterize it), or run it on a box without a
     production instance already bound to 7777.

2. **Minor doc drift**: `internal/templates/data/nephthys.json` (Hack Club's
   Slack support bot template) exists and is presumably functional but isn't
   mentioned in the README's template list. Not a functional break, just an
   undocumented template — worth a one-line README addition next time
   templates.md/README get touched.

### Not verified (needs Docker-heavy, hardware, or interactive setup beyond this pass)

- Actual container build/health-check/blue-green swap against a real target
  repo (covered by `go test ./internal/orchestrator/...` with a fake
  executor, and by e2e in principle — blocked by issue #1 above).
- The interactive terminal websocket (`GET /api/services/{id}/terminal` ↔
  `docker exec -it` pty), now only reachable via `mangrove-tui`'s shell view
  since the dashboard's xterm.js Terminal tab was removed.
- GitHub OAuth / webhook delivery against a real GitHub App (needs live
  credentials).
- `setup.sh` end-to-end on a truly fresh box (this box already has Mangrove
  installed) — this now includes the new interactive storage/NAS install
  prompt, also unexercised.
- DDNS (needs a real router with port-forwarding). Custom domains'
  default `auto_tls` mode also needs real DNS (unverified beyond what
  already existed). The new `MANGROVE_CUSTOM_DOMAIN_MODE=port` mode is
  covered by `internal/orchestrator/domains_test.go` against a fake
  executor and `o.Proxy == nil` (so the DB/port-allocation side is
  exercised, but not a real Caddy `PutRoute` call or an actual Nest
  domain→port registration) -- see [deployment.md](deployment.md#custom-domains-on-nest-or-anywhere-else-80443-isnt-reachable).
- **The protected-deployment gate page in a real browser.** The full
  handler logic (gate page render, wrong/right password, the three-hop
  account handoff, cookie issuance) is covered by `internal/api`'s
  `TestGate*` integration tests and `internal/proxy`'s Caddy-routing test,
  but nobody has clicked through it in an actual browser against a real
  deployed container -- `gateProxyThrough`'s happy path (successfully
  streaming a real app's response, including a WebSocket upgrade) is
  exercised only by Go's stdlib `httputil.ReverseProxy` being trusted to
  do the right thing, not by an end-to-end request against a running
  container. See [protected-deployments.md](protected-deployments.md).
- **Storage/NAS against real hardware**: this environment has no removable
  block device to plug in, so `internal/mountd`'s `lsblk`/`mount`/`umount`
  invocations, the `dperson/samba` container's actual SMB behavior, and a
  real SMB client (Windows/macOS/Linux) connecting to a share have not been
  exercised end to end — only unit-tested against fixture data and a fake
  mountd client (see [storage.md](storage.md)'s own "Not verified" section
  for exactly what that does and doesn't cover). Treat this feature as
  code-reviewed and logic-tested, not field-tested, until it's tried on a
  box with a real drive.
- **`internal/mountd/server.go`'s root-disk exclusion on an LVM/dm-crypt
  root.** `rootDiskName` resolves the root disk by taking
  `filepath.Base(findmnt -no SOURCE /)` and matching it against lsblk's
  `NAME` field — verified correct on this box's plain-partition root, but on
  an LVM root, `findmnt`'s reported source and lsblk's `NAME` for the same
  dm device aren't guaranteed to agree in string form across
  util-linux/kernel versions. If they diverge, the "never touch the system
  disk" exclusion in `filterDrives` goes silently inert and the root disk
  could be offered as a mountable drive. Flagged by the 2026-09-18 edge-case
  audit; not fixed because it needs a real LVM-root box to verify either the
  bug or a fix, not a guess. Test on such a box before trusting this
  exclusion on any LVM/dm-crypt host.
