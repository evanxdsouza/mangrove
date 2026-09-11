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

Last verified: 2026-09-11, updated same-day after adding custom domains' `MANGROVE_CUSTOM_DOMAIN_MODE=port` routing mode -- see the "Verified status" section below.

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
| `cmd/mangrovectl/` | Scriptable CLI, predates `internal/apiclient`, has its own hand-rolled HTTP client (`map[string]any`, not typed). | [clients.md](clients.md) |
| `cmd/mangrove-tui/` | Full-screen terminal dashboard (bubbletea). Shares `internal/apiclient`. | [clients.md](clients.md) |
| `cmd/mangrove-mcp/` | MCP server exposing a curated operations subset as tools for an LLM agent. Shares `internal/apiclient`. | [clients.md](clients.md) |
| `cmd/mangrove-mountd/` | Privileged helper for the storage/NAS feature — mounts/unmounts removable drives so `cmd/mangrove` itself never needs mount capabilities. Separate binary, separate systemd unit, talks to the main process only over a local Unix socket. | [storage.md](storage.md) |
| `internal/mountd/` | The mountd wire protocol (`protocol.go`), the privileged server logic `cmd/mangrove-mountd` runs (`server.go` — device enumeration via `lsblk`, mount/unmount, the root-disk exclusion safety check), and the client `internal/orchestrator` uses to talk to it (`client.go`). | [storage.md](storage.md) |
| `internal/api/` | HTTP handlers (thin) + `router.go` (the whole API surface, chi). Auth middleware applied per-route here. | [architecture.md](architecture.md), [multi-user.md](multi-user.md) |
| `internal/orchestrator/` | The actual logic: `deploy.go` (blue/green flow), `compose_deploy.go`, `deploy_static.go`, `lifecycle.go` (stop/restart/redeploy/scale), `templates.go` (install), `domains.go`, `delete.go`, `cancel.go`. This is where to look first for "how does X actually work." | [architecture.md](architecture.md) |
| `internal/executor/` | Shells out to Docker CLI: build (`docker.go`, `compose.go`), git fetch (`gitfetch.go`), build-strategy detection (`detect.go` — dockerfile/nixpacks/compose/static). | [architecture.md](architecture.md) |
| `internal/proxy/` | Drives Caddy's admin API (`caddy.go`) — route PUT/DELETE, no hand-written Caddyfile. | [architecture.md](architecture.md), [deployment.md](deployment.md) |
| `internal/store/` | SQLite reads/writes — the source of truth. `store.go` is the bulk of it. | [db/migrations](../internal/db/migrations) |
| `internal/db/` | `db.go` (connection, WAL, migration runner) + `migrations/*.sql` (numbered, forward-only). | — |
| `internal/portregistry/` | Allocates/releases host ports for public deployments from `MANGROVE_PORT_RANGE_MIN/_MAX`. | [architecture.md](architecture.md) |
| `internal/auth/` | Password hashing (bcrypt), session cookie issuing/validation, role middleware (`RequireAuth`/`RequireOwner`). | [multi-user.md](multi-user.md) |
| `internal/gateauth/` | Signed, tamper-evident tokens (sealed under the same master key as `internal/secrets`) backing a password-protected deployment's gate cookie and its cross-domain "continue with your Mangrove account" handoff -- no new DB table. | [protected-deployments.md](protected-deployments.md) |
| `internal/github/` | GitHub OAuth, repo listing, commit-status posting, PR comment upsert. | [architecture.md](architecture.md)#github-auto-deploy |
| `internal/webhook/` | `githubWebhook` HTTP handler's supporting logic — HMAC verify, delivery dedup. (Handler itself is `internal/api/webhook.go`.) | [architecture.md](architecture.md)#github-auto-deploy |
| `internal/templates/` | `templates.go` (loader + `validate()`, panics at `init()` on a bad template) + `data/*.json` (the templates themselves, embedded via `go:embed`). | [templates.md](templates.md) |
| `internal/scheduler/` | Background jobs: `health.go` (deployment health polling), `prune.go` (old image cleanup), `ddns.go` (DuckDNS updater, every 5 min). | [deployment.md](deployment.md)#home-server--ddns |
| `internal/secrets/` | Encryption at rest for secret env vars / PATs (AAD bound to the owning service/PAT row). | — |
| `internal/sysinfo/` | Host/cgroup introspection for the admin resource-budget view. | — |
| `internal/notify/` | Optional Resend email notifications on deploy result. | — |
| `internal/models/` | Shared Go structs — what `internal/store` returns and what `internal/api` serializes. Also what `internal/apiclient` decodes into (see `docs/clients.md` for why that reuse matters). | [clients.md](clients.md) |
| `internal/apiclient/` | Typed HTTP client shared by `mangrove-tui`/`mangrove-mcp`. Cookie-based session, `~/.mangrove/session` on disk. | [clients.md](clients.md) |
| `internal/webui/` | `embed.go` (`go:embed` of `dist/`) serving the built SPA. `dist/` is git-ignored except a `.gitkeep`; it only exists after `cd web && npm run build`. | README Quickstart |
| `internal/config/` | `config.go` — every env var, all optional, defaults documented in the README's config table. | README |
| `web/` | The React 19 SPA. Hand-rolled router/state (`router.tsx`, `userContext.tsx`, `uiMode.tsx`, `workspaceContext.tsx` — the active-workspace "station switcher" state shared by `Layout.tsx` and `ProjectsPage.tsx`, persisted like `uiMode.tsx`) — no react-router, no Redux. `src/pages/` (technical mode) + `src/pages/simple/` (simple mode, see [modes.md](modes.md)). One hand-written design system in `src/styles.css` (CSS custom-property tokens, no Tailwind/MUI) plus an authored SVG icon set in `src/icons.tsx` — the "Field Station / Instrument Room" visual identity, recorded in [DESIGN.md](../DESIGN.md). Self-hosted fonts via `@fontsource/space-grotesk` + `@fontsource/ibm-plex-mono` (latin/latin-ext subsets only, imported in `main.tsx`) — no external font CDN. | [architecture.md](architecture.md)#why-chi-why-no-cgo-sqlite-why-a-hand-rolled-frontend-router, [DESIGN.md](../DESIGN.md) |
| `e2e/` | Playwright suite (`tests/dashboard.spec.ts`) run via `run.sh` against a real (freshly built) Mangrove + real Docker + real Caddy — nothing mocked. | see "Verified status" below |
| `deploy/systemd/` | The actual unit/slice files this project runs in production (`mangrove.service`, two memory-isolating slices, a Caddy drop-in, plus the optional `mangrove-mountd.service` + its `mangrove-storage-group.conf` drop-in for storage/NAS). | [deployment.md](deployment.md), [storage.md](storage.md) |
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

## Verified status (2026-09-11, updated same-day for custom-domain port-routing mode)

Everything below was actually run on this box, not inferred from reading code. The Playwright/manual-QA and e2e rows are carried over unchanged from the 2026-09-07 dashboard-redesign pass (not re-run this time — this change doesn't touch the pages they cover); the Go/frontend build+test rows were re-run fresh against the custom-domain port-routing-mode feature specifically.

| Check | Command | Result |
|---|---|---|
| Go build | `go build ./...` | ✅ clean, all of `cmd/` + `internal/` |
| Go vet | `go vet ./...` | ✅ clean |
| Go tests | `go test ./internal/...` | ✅ all packages pass, including new coverage in `internal/orchestrator/domains_test.go` (`TestAddCustomDomainPortModeIsLiveImmediately`, `TestRemoveCustomDomainPortModeReleasesPort`, `TestVerifyCustomDomainPortModeIsANoOp`, plus a default-mode regression test) and an updated `internal/db` migration-count test |
| Frontend typecheck + build | `cd web && npm run build` | ✅ `tsc -b` clean, `vite build` succeeds |
| Frontend lint | `cd web && npm run lint` (oxlint) | ✅ clean (only the same pre-existing warnings as before — see "Known issues") |
| Manual QA | throwaway instance (`MANGROVE_DATA_DIR`/`MANGROVE_PORT` against a scratch dir, admin account + sample workspaces/projects/deployments via the API), Playwright screenshots at desktop (1440×900) and mobile (390×844) across every technical- and simple-mode page | ✅ from the 2026-09-07 pass, unchanged by this session — see "Not verified" below for what *this* change specifically hasn't been through a real browser for |
| E2E suite | `./e2e/run.sh` | ❌ fails on test 1 of 6, **not an app bug** — see "Known issues" (unchanged from the prior pass) |

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
