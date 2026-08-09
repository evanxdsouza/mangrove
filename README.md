# Mangrove

Mangrove is a small, self-hosted PaaS control plane: point it at a Git repo,
a Dockerfile, a docker-compose file, or a pre-built image, and it builds,
runs, health-checks, and reverse-proxies it -- blue/green, with automatic
rollback if the new container never turns healthy. It also ships a library
of one-click templates (Ghost, WordPress, Vaultwarden, Uptime Kuma, n8n,
Umami, and standalone Postgres/MySQL/MongoDB/Redis) for deploying common
self-hosted apps without writing a Dockerfile at all.

It's built to run comfortably on a single small box -- the reference
target throughout development has been a 2 vCPU / 2GB RAM / 16GB disk VPS,
alongside whatever it's deploying.

For non-technical users, the dashboard has a **Simple mode** toggle that
swaps the technical view (projects, deployments, build strategies, env
vars) for a plain-language "your apps" flow built on the same API -- see
[docs/modes.md](docs/modes.md).

## How it fits together

- **Backend**: Go, [chi](https://github.com/go-chi/chi) router, SQLite
  (via `modernc.org/sqlite`, no cgo) as the sole source of truth, Docker as
  the executor, [Caddy](https://caddyserver.com/) as the reverse proxy
  (driven entirely through its admin API -- no hand-written Caddyfile).
- **Frontend**: a small React 19 SPA (`web/`), hand-rolled routing and
  state (no react-router, no Redux/Zustand) -- see
  [docs/architecture.md](docs/architecture.md) for why.
- **Multi-user**: owner/member roles, session-cookie auth -- see
  [docs/multi-user.md](docs/multi-user.md).

See [docs/architecture.md](docs/architecture.md) for the full request path
and the blue/green deploy flow, [docs/deployment.md](docs/deployment.md)
for running this in production, and
[docs/templates.md](docs/templates.md) for the one-click template format.

## Production install

On a fresh Debian/Ubuntu VPS, `sudo ./setup.sh` installs Docker, Caddy, Go,
and Node if missing; builds the frontend and binary; installs the systemd
units/slices from `deploy/systemd/` (see
[docs/deployment.md](docs/deployment.md) for what these do); starts the
service; and creates the admin account for you (prompted interactively, or
via `MANGROVE_ADMIN_EMAIL`/`MANGROVE_ADMIN_PASSWORD` for an unattended run)
instead of doing it through the browser's first-run screen. Safe to re-run.

## Quickstart (local dev)

Prerequisites: Go 1.25+, Node 20+, Docker (running, with the current user
able to run `docker` without `sudo`), and [Caddy](https://caddyserver.com/docs/install)
running locally with its admin API reachable at `127.0.0.1:2019` (Caddy's
default -- `caddy run` with no arguments is enough).

```sh
# 1. Build the frontend into internal/webui/dist, which the Go binary embeds.
cd web && npm install && npm run build && cd ..

# 2. Build and run the control plane.
go build -o mangrove ./cmd/mangrove
./mangrove
```

This starts Mangrove on `http://127.0.0.1:7777`, storing its SQLite DB and
encryption key under `./data` (both created on first run). Open that URL
in a browser: the first visit prompts you to create the initial account,
which is automatically the `owner` role (see
[docs/multi-user.md](docs/multi-user.md)). From there, either deploy from
Git/an image/a Dockerfile, or install a one-click template.

Mangrove auto-creates its own Docker network (`mangrove-net` by default)
on first use -- no manual `docker network create` needed.

### Configuration

Everything is environment variables, all optional with sensible local-dev
defaults (see `internal/config/config.go`):

| Variable | Default | Purpose |
|---|---|---|
| `MANGROVE_DATA_DIR` | `./data` | SQLite DB + master encryption key |
| `MANGROVE_STATIC_SITES_DIR` | `./mangrove-static` | Built output for static-strategy deploys |
| `MANGROVE_PORT` | `7777` | Mangrove's own API/dashboard port |
| `MANGROVE_PORT_RANGE_MIN` / `_MAX` | `20000` / `21000` | Auto-allocated deployment port range |
| `MANGROVE_NETWORK` | `mangrove-net` | Docker network Mangrove creates/uses |
| `MANGROVE_CGROUP_PARENT` | (empty) | systemd slice deployment containers run under (production only) |
| `MANGROVE_CADDY_ADMIN_ADDR` | `http://127.0.0.1:2019` | Caddy admin API |
| `MANGROVE_DEPLOYMENT_MEMORY_CEILING_MB` | `1536` | Admission-control ceiling across all deployments |
| `MANGROVE_RESEND_API_KEY` / `MANGROVE_NOTIFY_EMAIL` | (empty) | Optional deploy-result email notifications |
| `MANGROVE_BASE_DOMAIN` | `evanxdsouza.hackclub.app` | Cosmetic only -- suggested domain in notification emails |

### Running the test suite

```sh
go test ./...              # unit + integration tests (real SQLite, fake or real Docker depending on package)
cd web && npx tsc -b       # frontend typecheck
./e2e/run.sh                # full end-to-end: real Docker + real Caddy, Playwright-driven
```

## Production deployment

See [docs/deployment.md](docs/deployment.md) for the systemd units, the
Caddy slice drop-in, and the binary-update procedure this project actually
uses in production.

## CLI

`cmd/mangrovectl` is a thin CLI client for the same HTTP API the dashboard
uses (`mangrovectl setup`, `project create`, `deploy`, `rollback`, etc.) --
run `mangrovectl` with no arguments for the full command list.
