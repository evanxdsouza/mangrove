# Production deployment

This is the exact setup this project runs in production: a single Linux
VPS, systemd-managed, Caddy as the reverse proxy, Docker as the executor.
The unit files referenced below live in `deploy/systemd/`.

## Prerequisites on the box

- Docker, running, with a `mangrove` system user in the `docker` group
  (needed to reach the daemon socket -- effectively root-equivalent
  access; there's no way around this for a Docker-based executor, which
  is why this host should be single-purpose).
- Caddy installed and running as `caddy.service` (e.g. via the
  [official apt repo](https://caddyserver.com/docs/install)) with its
  admin API reachable at `127.0.0.1:2019` (Caddy's default -- nothing
  extra to configure there). Mangrove drives Caddy entirely through that
  admin API at runtime; there is no Caddyfile to hand-write or maintain.
- `/var/lib/mangrove` (0700, `mangrove:mangrove`) for the SQLite DB and
  master encryption key.
- `/var/lib/mangrove-static` (0755, `mangrove:mangrove`) for static-site
  build output. This is **separate** from `/var/lib/mangrove` on purpose:
  Caddy's `file_server` needs to read static-site output directly off
  disk, but it runs as a different, unprivileged system user that has no
  business reading the 0700 directory holding the DB and encryption key.

## Resource isolation (systemd slices)

Two slices split the box's memory between Mangrove-itself-and-Caddy and
the containers it deploys, so a runaway deployment can't starve SSH/the
control plane, and so the control plane can't be starved by its own leak:

- `deploy/systemd/mangrove.slice` -- `MemoryMin=384M` (protected floor,
  never reclaimed in favor of the deployments slice under pressure),
  `MemoryHigh=512M` (soft ceiling, throttles rather than kills). Both
  `mangrove.service` and `caddy.service` join this slice (see
  `caddy-mangrove-slice.conf`, a systemd drop-in for `caddy.service.d/`).
- `deploy/systemd/mangrove-deployments.slice` -- `MemoryMax=1536M` (hard
  ceiling across every deployment container combined, via
  `--cgroup-parent`), `MemorySwapMax=0`. Sized for a 2GB host as `TotalRAM
  - mangrove.slice's floor - OS/dockerd overhead`; adjust if your host's
  total RAM differs. This should match `MANGROVE_DEPLOYMENT_MEMORY_CEILING_MB`
  (see the README's config table) -- the slice is the actual kernel-enforced
  backstop, the env var is Mangrove's own admission control that's meant
  to reject an over-budget deploy *before* it ever reaches the kernel limit.

Because `mangrove.slice`'s 512M soft ceiling is shared with Caddy, and
because Go's GC doesn't know that ceiling exists on its own,
`mangrove.service` sets `GOMEMLIMIT=320MiB` (leaving headroom for Caddy
and OS/runtime overhead) -- see the comment in `mangrove.service` for the
full reasoning. `GOGC` is left at its default: `GOMEMLIMIT` is meant as a
backstop, not the primary GC pacing knob.

Install:

```sh
cp deploy/systemd/mangrove.slice deploy/systemd/mangrove-deployments.slice /etc/systemd/system/
mkdir -p /etc/systemd/system/caddy.service.d
cp deploy/systemd/caddy-mangrove-slice.conf /etc/systemd/system/caddy.service.d/mangrove-slice.conf
systemctl daemon-reload
systemctl restart caddy
```

## Installing the service

```sh
cp deploy/systemd/mangrove.service /etc/systemd/system/mangrove.service
# Secrets (MANGROVE_RESEND_API_KEY etc.) go here, never in the unit file or git:
mkdir -p /etc/mangrove && touch /etc/mangrove/mangrove.env && chmod 600 /etc/mangrove/mangrove.env
systemctl daemon-reload
systemctl enable --now mangrove
```

The unit runs the `mangrove` system user under `ProtectSystem=strict` with
`ReadWritePaths` limited to `/var/lib/mangrove` and `/var/lib/mangrove-static`,
`NoNewPrivileges=true`, and `PrivateTmp=true` (build fetch/extract needs a
writable temp dir; `ProtectSystem=strict` would otherwise make `/tmp`
read-only). `ProtectHome=true` hides the real `/home`, which breaks the
Docker CLI's `$HOME/.docker/config.json` lookup badly enough to misparse
the *next* CLI flag -- worked around by pointing `HOME` at
`/var/lib/mangrove` explicitly in the unit's `Environment=`.

## Updating the binary

Building on the box (or building elsewhere and copying the binary over)
and swapping it in:

```sh
cd web && npm run build && cd ..          # rebuilds internal/webui/dist, which the binary embeds
go build -o /tmp/mangrove-new ./cmd/mangrove

cp /tmp/mangrove-new /usr/local/bin/mangrove-staged
mv /usr/local/bin/mangrove-staged /usr/local/bin/mangrove
systemctl restart mangrove
systemctl is-active mangrove && curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:7777/healthz
```

**Why `cp` then `mv`, not a direct `cp` over the running binary**: while
`mangrove.service` is running, its binary is an open, executing file.
Overwriting it in place (`cp new /usr/local/bin/mangrove`) truncates and
rewrites the same inode the kernel is currently executing from, which
either corrupts the in-flight process's mapped pages or fails outright
with `ETXTBSY` ("text file busy") depending on timing. `mv` within the
same filesystem is a single atomic rename: the old inode stays valid and
fully intact for the still-running process (which keeps its file
descriptor/mapping to it until it exits on `systemctl restart`), and the
path starts pointing at the new binary the instant the rename completes.
Staging under a different name first (`mangrove-staged`, not
`/usr/local/bin/mangrove` directly) avoids a window where the target path
briefly doesn't exist mid-copy, which matters if anything else reads that
path concurrently.

If you changed `deploy/systemd/mangrove.service` itself (e.g. a new env
var), also `cp` it over `/etc/systemd/system/mangrove.service` and run
`systemctl daemon-reload` before `restart` -- a plain restart alone
re-reads the *old* installed unit file, not the repo's copy.

## Updating a systemd unit or slice file

Same shape as above but for the unit files themselves:

```sh
cp deploy/systemd/mangrove.service /etc/systemd/system/mangrove.service
systemctl daemon-reload
systemctl restart mangrove
```

`daemon-reload` is required any time an installed unit file changes --
systemd caches parsed unit files and won't notice an on-disk edit
otherwise.

## Custom domains

Beyond the port-based routing every deployment gets automatically
(`srv_<port>` server blocks, see `internal/proxy/caddy.go`), an owner can
point an arbitrary domain at a deployment from its "Domains" tab in the
dashboard. This programs a *host-matched* route on a separate, shared
Caddy server block (`srv_public`, listening on `:80`/`:443`) -- Caddy's
own automatic HTTPS then provisions and renews the TLS certificate for
that hostname with no further configuration needed.

Adding a domain requires proving control of its DNS first: Mangrove shows
a `mangrove-domain-verification=<token>` value to add as a TXT record on
the hostname, and the route only goes live once "Verify" finds it --
without this check, anyone who points a stale/misconfigured domain's DNS
at this box's IP could intercept traffic meant for someone else's
deployment.

`srv_public` needs `:80`/`:443` free on this box. That's true for a plain
VPS/Nest install by default (deployment routes live on the high port
range instead); it is **not** true if you've hand-configured Caddy to
proxy `:80`/`:443` to something else already (e.g. Mangrove's own
dashboard reachable through an external edge) -- custom domains won't
work until that's resolved, since two Caddy server blocks can't bind the
same port.

## Home server / DDNS

A home server sitting behind a router usually has neither a static
public IP nor forwarded ports by default, so custom domains need two
extra things `setup.sh` can configure interactively (choose "home" when
asked how the box is reachable):

- **Port forwarding**: forward `80` and `443` on your router to this
  box. There's no way around this -- it's how traffic (and the ACME
  HTTP-01 challenge Caddy uses for automatic HTTPS) reaches the box at
  all.
- **DDNS**: `internal/scheduler/ddns.go` runs a small background job
  (every 5 minutes) that keeps a [DuckDNS](https://www.duckdns.org)
  subdomain pointed at your current public IP, configured via
  `MANGROVE_DDNS_DOMAIN`/`MANGROVE_DDNS_TOKEN`/`MANGROVE_DDNS_PROVIDER`
  in `/etc/mangrove/mangrove.env`. Only `duckdns` is supported today.
  Leave `MANGROVE_DDNS_DOMAIN` unset (the default) to disable the job
  entirely -- a VPS/Nest install with a stable IP doesn't need it.

`setup.sh` also asks, separately, whether to install `mangrove-mountd` --
the privileged helper behind the Storage page's drive-to-NAS feature (mount
a plugged-in drive, share it over SMB to other devices on the LAN). Also
optional, also off by default; see [docs/storage.md](storage.md).

## Verifying without touching production

For any change riskier than a pure code diff (schema changes, teardown
logic, anything touching live containers), stand up a second throwaway
instance against the *real* Docker daemon, on a separate port/data-dir/Docker
network, rather than testing against the real production DB and
containers:

```sh
MANGROVE_PORT=7778 \
MANGROVE_DATA_DIR=/tmp/mangrove-verify \
MANGROVE_NETWORK=mangrove-verify-net \
./mangrove
```

Tear down afterward: kill the process, `docker network rm mangrove-verify-net`,
remove the temp data dir. `e2e/run.sh` automates exactly this pattern for
the full Playwright suite (including its own teardown on any exit path).
