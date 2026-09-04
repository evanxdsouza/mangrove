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

Last verified: 2026-09-04, against commit `da5d931` on branch `docs-refresh`.

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
| `internal/api/` | HTTP handlers (thin) + `router.go` (the whole API surface, chi). Auth middleware applied per-route here. | [architecture.md](architecture.md), [multi-user.md](multi-user.md) |
| `internal/orchestrator/` | The actual logic: `deploy.go` (blue/green flow), `compose_deploy.go`, `deploy_static.go`, `lifecycle.go` (stop/restart/redeploy/scale), `templates.go` (install), `domains.go`, `delete.go`, `cancel.go`. This is where to look first for "how does X actually work." | [architecture.md](architecture.md) |
| `internal/executor/` | Shells out to Docker CLI: build (`docker.go`, `compose.go`), git fetch (`gitfetch.go`), build-strategy detection (`detect.go` — dockerfile/nixpacks/compose/static). | [architecture.md](architecture.md) |
| `internal/proxy/` | Drives Caddy's admin API (`caddy.go`) — route PUT/DELETE, no hand-written Caddyfile. | [architecture.md](architecture.md), [deployment.md](deployment.md) |
| `internal/store/` | SQLite reads/writes — the source of truth. `store.go` is the bulk of it. | [db/migrations](../internal/db/migrations) |
| `internal/db/` | `db.go` (connection, WAL, migration runner) + `migrations/*.sql` (numbered, forward-only). | — |
| `internal/portregistry/` | Allocates/releases host ports for public deployments from `MANGROVE_PORT_RANGE_MIN/_MAX`. | [architecture.md](architecture.md) |
| `internal/auth/` | Password hashing (bcrypt), session cookie issuing/validation, role middleware (`RequireAuth`/`RequireOwner`). | [multi-user.md](multi-user.md) |
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
| `web/` | The React 19 SPA. Hand-rolled router/state (`router.tsx`, `userContext.tsx`, `uiMode.tsx`) — no react-router, no Redux. `src/pages/` (technical mode) + `src/pages/simple/` (simple mode, see [modes.md](modes.md)). | [architecture.md](architecture.md)#why-chi-why-no-cgo-sqlite-why-a-hand-rolled-frontend-router |
| `e2e/` | Playwright suite (`tests/dashboard.spec.ts`) run via `run.sh` against a real (freshly built) Mangrove + real Docker + real Caddy — nothing mocked. | see "Verified status" below |
| `deploy/systemd/` | The actual unit/slice files this project runs in production (`mangrove.service`, two memory-isolating slices, a Caddy drop-in). | [deployment.md](deployment.md) |
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
  equivalent, see [modes.md](modes.md).
- **New client-exposed operation** (TUI/MCP): add to `internal/apiclient/`
  first (typed), then the specific client. MCP's tool surface is
  deliberately narrower than the full API — see [clients.md](clients.md)'s
  "Deliberately not exposed" list before adding a destructive MCP tool.

## Verified status (2026-09-04)

Everything below was actually run on this box, not inferred from reading code.

| Check | Command | Result |
|---|---|---|
| Go build | `go build ./...` | ✅ clean, all of `cmd/` + `internal/` |
| Go vet | `go vet ./...` | ✅ clean |
| Go tests | `go test ./...` | ✅ all packages pass (`internal/api`, `orchestrator`, `executor`, `store`, `auth`, `templates`, `webhook`, `proxy`, `portregistry`, `scheduler`, `secrets`, `sysinfo`, `github`, `notify`, `db`, `webui`) |
| Frontend typecheck + build | `cd web && npm run build` | ✅ `tsc -b` clean, `vite build` succeeds (one benign warning: main JS chunk is 586 kB / 151 kB gzipped, over the 500 kB default budget — not an error, just an unaddressed code-splitting opportunity) |
| Frontend lint | `cd web && npm run lint` (oxlint) | ⚠️ 1 real error class, several pre-existing warnings — see "Known issues" below |
| E2E suite | `./e2e/run.sh` | ❌ fails on test 1 of 6, **not an app bug** — see "Known issues" |

### Known issues found

1. **Real bug — conditional hooks in `ScaleCard`**
   (`web/src/pages/DeploymentDetailPage.tsx:858-865`). `ScaleCard` early-returns
   `null` for `compose`/`static` build strategies *before* calling its four
   `useState` hooks, which violates React's rules of hooks (`oxlint`'s
   `react-hooks/rules-of-hooks` flags all four as errors). In practice a given
   deployment's `build_strategy` never changes while `ScaleCard` stays
   mounted, so this likely doesn't crash today — but it's incorrect code that
   will break under React Strict Mode double-render semantics or if this
   component is ever reused somewhere props can change. Fix: move the four
   `useState` calls above the early-return guard.

2. **E2E suite fails due to a port collision with this box's own production
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

3. **Minor doc drift**: `internal/templates/data/nephthys.json` (Hack Club's
   Slack support bot template) exists and is presumably functional but isn't
   mentioned in the README's template list. Not a functional break, just an
   undocumented template — worth a one-line README addition next time
   templates.md/README get touched.

### Not verified (needs Docker-heavy or interactive setup beyond this pass)

- Actual container build/health-check/blue-green swap against a real target
  repo (covered by `go test ./internal/orchestrator/...` with a fake
  executor, and by e2e in principle — blocked by issue #2 above).
- The interactive web terminal (xterm.js ↔ websocket ↔ `docker exec -it`
  pty) — no browser-driven manual check was done this pass.
- GitHub OAuth / webhook delivery against a real GitHub App (needs live
  credentials).
- `setup.sh` end-to-end on a truly fresh box (this box already has Mangrove
  installed).
- Custom domains / DDNS (needs real DNS + router port-forwarding).
