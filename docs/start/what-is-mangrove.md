# What is Mangrove

Mangrove turns one Linux box into a small platform for running your apps.
Point it at a Git repository, a Dockerfile, a compose file or a pre-built
image, and Mangrove builds it, runs it, checks it's healthy, and puts it
behind a reverse proxy. Push to GitHub and it redeploys itself.

It is in the same family as Coolify and Dokploy: something you run
yourself, on your own hardware, that makes deploying feel like a hosted
platform. It is built to be small. The reference box throughout
development has been 2 vCPU, 2 GB of RAM and 16 GB of disk, with Mangrove
and the apps it runs sharing that box.

## What you get

- **Deploys that can't take a working app down.** A new version starts
  next to the old one and only takes traffic once it passes its health
  check. If it never does, the old version keeps running.
- **GitHub auto-deploy**, staging environments that promote the exact
  commit they tested, and a preview deployment for every pull request.
- **Custom domains with automatic HTTPS**, including from a home server
  behind a router with no static IP.
- **One-click templates** for common self-hosted apps: Ghost, WordPress,
  Gitea, Supabase, PocketBase, NocoDB, Vaultwarden, Uptime Kuma, n8n,
  Umami, and standalone Postgres, MySQL, MongoDB and Redis.
- **A dashboard with two faces**: a technical view, and a
  [Simple mode](simple-mode.md) for people who just want "a blog" running.
- **More ways in**: a CLI, a full-screen terminal UI, and an MCP server for
  AI agents, all talking to the same API.
- **Optional extras for a home server**: turn a plugged-in drive into a
  NAS, and let idle apps sleep until someone visits.

## What it is not

**Not a hosted service.** You run it, on a box you control.

**Not a cluster scheduler.** Mangrove runs everything on one machine,
using Docker directly. There is no Kubernetes underneath and no
multi-machine scheduling.

**Not a sandbox for untrusted code.** Apps run as ordinary Docker
containers, and Mangrove itself needs Docker access, which is equivalent to
root on the box. Run code you trust, on a box that does nothing else. See
[Security and isolation](../operate/security.md).

**Not a replacement for app-level backups.** Mangrove can back up its own
state (projects, settings, secrets), but the data inside your apps'
volumes needs its own backup plan. See
[Backups and restore](../data/backups.md).

## How it fits together

```
Browser / CLI / TUI / MCP
          │
          ▼
   mangrove (one Go binary: API + dashboard, state in SQLite)
          │
   ┌──────┴───────┐
   ▼              ▼
 Docker         Caddy (admin API)
 builds and     routes each app's port
 runs apps      and custom domains, HTTPS
```

Mangrove is one binary with the dashboard embedded in it. Its state lives
in a single SQLite file. It shells out to Docker to build and run
containers, and drives Caddy through Caddy's admin API, so there is no
Caddyfile to maintain. More in [Architecture](../operate/architecture.md).
