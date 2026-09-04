# Mangrove

Mangrove is a small, self-hosted PaaS control plane: point it at a Git repo,
a Dockerfile, a docker-compose file, or a pre-built image, and it builds,
runs, health-checks, and reverse-proxies it -- blue/green, with automatic
rollback if the new container never turns healthy. A GitHub-linked
deployment redeploys itself on every push, and can spin off staging
environments -- each tracking its own branch, deployed independently, one
click away from promoting the exact commit it's running into production --
or opt into a preview deployment for every open pull request, complete with
a bot comment linking to it that updates itself as the PR gets pushed to
and torn down when it closes
(see [docs/architecture.md](docs/architecture.md#github-auto-deploy-and-staging-environments)).
Beyond the port-based routing every deployment gets automatically, an owner
can point a custom domain at one straight from its dashboard, with Caddy
provisioning and renewing TLS for it automatically -- including from behind
a home router with no static IP, via a built-in DDNS updater (see
[docs/deployment.md](docs/deployment.md#custom-domains)). Every service also
gets a full interactive terminal in the browser (xterm.js over a websocket,
attached to a real pseudo-terminal via `docker exec -it`), not just a log
tail -- see
[docs/architecture.md](docs/architecture.md#lifecycle-actions-short-of-a-full-deploy).
It also ships a library
of one-click templates (Ghost, WordPress, Gitea, Supabase, PocketBase,
NocoDB, Vaultwarden, Uptime Kuma, n8n, Umami, and standalone
Postgres/MySQL/MongoDB/Redis) for deploying common self-hosted apps without
writing a Dockerfile at all. For a home server, an optional Storage page
turns a plugged-in drive into a NAS -- mount it and share it over SMB to
other devices on the LAN, backed by a small separate privileged helper so
the main control plane never needs mount capabilities itself (see
[docs/storage.md](docs/storage.md)).

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
- **Clients**: the dashboard, plus a CLI, a terminal UI, and an MCP server
  for LLM agents, all in `cmd/`, all in this one module -- see
  [docs/clients.md](docs/clients.md).

See [docs/architecture.md](docs/architecture.md) for the full request path
and the blue/green deploy flow, [docs/deployment.md](docs/deployment.md)
for running this in production, [docs/templates.md](docs/templates.md) for
the one-click template format, and [docs/storage.md](docs/storage.md) for
the drive-to-NAS feature.

## Production install

On a fresh Debian/Ubuntu VPS (or a home server), `sudo ./setup.sh` installs
Docker, Caddy, Go, and Node if missing; builds the frontend and binary;
installs the systemd units/slices from `deploy/systemd/` (see
[docs/deployment.md](docs/deployment.md) for what these do); asks whether
the box is reachable on a public IP or sits behind a router at home,
configuring the DDNS updater custom domains need in the latter case (see
[docs/deployment.md](docs/deployment.md#home-server--ddns)); starts the
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
| `MANGROVE_GITHUB_OAUTH_CLIENT_ID` / `_SECRET` | (empty) | Enables "Deploy from GitHub" (OAuth connect + repo picker); both empty disables it, pasted PATs still work. Register an OAuth App at github.com/settings/developers with callback URL `https://<your-host>/api/github/oauth/callback` |
| `MANGROVE_PUBLIC_URL` | (empty; request-derived) | Overrides scheme+host detection for GitHub callback URLs (OAuth redirect, auto-registered webhooks). Set this if you're behind an edge that terminates HTTPS but doesn't forward `X-Forwarded-Proto` to Mangrove's own dashboard route, e.g. `https://mangrove.example.com` |
| `MANGROVE_DDNS_DOMAIN` / `_TOKEN` / `_PROVIDER` | (empty) / (empty) / `duckdns` | Keeps a DDNS domain pointed at this box's current public IP -- for [custom domains](docs/deployment.md#custom-domains) from a home server with no static IP. Empty `MANGROVE_DDNS_DOMAIN` (the default) disables the job entirely; a VPS with a stable IP doesn't need it. Only `duckdns` is supported today -- see [docs/deployment.md](docs/deployment.md#home-server--ddns) |

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

## Clients: CLI, TUI, MCP

Besides the dashboard, this repo (one module, built together -- the
"mono" in the name) ships three more ways to drive Mangrove, all talking
to the same HTTP API over the network:

- `cmd/mangrovectl` -- a thin, scriptable CLI (`mangrovectl setup`,
  `project create`, `deploy`, `rollback`, etc.) -- run `mangrovectl` with
  no arguments for the full command list.
- `cmd/mangrove-tui` -- a full-screen terminal dashboard: browse
  projects/deployments, redeploy/restart/stop/scale, roll back, tail live
  logs, and open a real interactive shell into a container's `docker
  exec -it`, all without leaving the terminal.
- `cmd/mangrove-mcp` -- an MCP server exposing the same operations as
  tools an LLM agent (Claude Code, Claude Desktop, etc.) can call.

All three (`mangrovectl` aside, which predates and isn't migrated to it)
share `internal/apiclient`, decoding responses straight into the same Go
types the backend itself returns rather than each guessing at the JSON
shape independently -- see [docs/clients.md](docs/clients.md) for why that
matters, how auth/session-sharing works across all three, and how to wire
`mangrove-mcp` into an MCP client's config.
