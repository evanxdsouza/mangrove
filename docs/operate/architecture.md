# Architecture

Mangrove is one Go binary, one SQLite file, and two things it drives:
Docker and Caddy.

```
 Browser · mangrovectl · mangrove-tui · mangrove-mcp
                    │  HTTP (session cookie)
                    ▼
 ┌──────────────── mangrove (127.0.0.1:7777) ────────────────┐
 │ chi router ── /api/*         handlers (thin)              │
 │            ├─ /webhooks/github/{token}  (HMAC-verified)   │
 │            ├─ /healthz                                    │
 │            └─ /*             embedded SvelteKit dashboard │
 │                                                           │
 │ orchestrator ── executor ──▶ docker / docker compose / git│
 │      │          proxy    ──▶ Caddy admin API              │
 │      │          store    ──▶ SQLite (WAL)                 │
 │      │          portregistry                              │
 │ scheduler: health checks · self-healing · idle sleep ·    │
 │            image pruning · resource samples · DDNS ·      │
 │            PR-preview cleanup                             │
 └───────────────────────────────────────────────────────────┘
          │ Unix socket (optional)
          ▼
 mangrove-mountd (root): list / mount / unmount drives
```

## The pieces

- **SQLite is the source of truth.** Every action writes the database
  first. There's no job queue or cache.
- **Docker** builds and runs everything. Mangrove shells out to the
  `docker`, `docker compose`, `git` and `nixpacks` CLIs.
- **Caddy** is the only thing that listens on public ports. Mangrove
  drives it through its admin API, so there's no Caddyfile to generate.
  Each public service gets a server on its own port (`srv_<port>`); verified
  custom domains share one on 80/443 (`srv_public`).
- **The scheduler** runs background jobs inside the same process: health
  checks, self-healing retries, idle sleep, image pruning (every 6 hours),
  resource samples (every 5 minutes), DDNS updates and PR-preview cleanup.
- **`mangrove-mountd`** is a separate root helper for the optional NAS
  feature, so the main process never needs mount privileges.

## A deploy, step by step

1. **Admission**: refuse if the memory budget would be exceeded.
2. **Build** the image (or skip, for an existing image).
3. **Reserve a port**, for public services.
4. **Start the new container** next to the old one, on the internal
   network only.
5. **Health-check** it for up to 60 seconds. If it fails, remove it; the old
   one never stopped.
6. **Switch Caddy** to the new container in one atomic update.
7. **Remove the old container**.
8. Record the run, prune old images, send notifications.

A rollback is the same flow without step 2. Compose stacks use
`docker compose up` instead, and static sites skip containers entirely.

## Why it's built this way

- **One binary, embedded dashboard, no cgo.** The binary can be copied to
  a box and run, with nothing else to install for Mangrove itself.
- **SQLite.** A small box doesn't need a database server, and backups are
  one file plus a key.
- **Caddy's admin API.** Routes change atomically, without a config file to
  template and reload.
- **One box.** Mangrove targets a 2 GB VPS or a home server. Everything,
  from memory admission to slices, assumes one machine.

The full design, with the reasoning behind each decision:
[architecture.md](../architecture.md). Where the code lives:
[REPO_MAP.md](../REPO_MAP.md).
