# Configuration

Mangrove is configured entirely with environment variables. In production
they go in `/etc/mangrove/mangrove.env` (mode `600`); restart Mangrove after
changing them. Everything is optional, with defaults suited to local
development.

## Server

| Variable | Default | Meaning |
|---|---|---|
| `MANGROVE_PORT` | `7777` | API and dashboard port, on `127.0.0.1` only |
| `MANGROVE_DATA_DIR` | `./data` | Database and `master.key`. Relative to the working directory, so `/var/lib/mangrove/data` under systemd |
| `MANGROVE_STATIC_SITES_DIR` | `./mangrove-static` | Built static sites. The unit sets `/var/lib/mangrove-static` |
| `MANGROVE_PUBLIC_URL` | derived from each request | The dashboard's public address, e.g. `https://mangrove.example.com`. Used for GitHub callbacks, webhooks and protected-deployment sign-in. Set it whenever Mangrove is behind a proxy |

## Docker and resources

| Variable | Default | Meaning |
|---|---|---|
| `MANGROVE_NETWORK` | `mangrove-net` | Docker network for every app container. Created if missing |
| `MANGROVE_PORT_RANGE_MIN` / `_MAX` | `20000` / `21000` | Ports handed to public services and port-mode domains |
| `MANGROVE_DEPLOYMENT_MEMORY_CEILING_MB` | `1536` | The [memory budget](../deploy/scaling.md#the-memory-budget) across all deployments |
| `MANGROVE_CGROUP_PARENT` | empty | systemd slice app containers run under. `setup.sh` sets `mangrove-deployments.slice` |

## Routing and domains

| Variable | Default | Meaning |
|---|---|---|
| `MANGROVE_CADDY_ADMIN_ADDR` | `http://127.0.0.1:2019` | Caddy's admin API |
| `MANGROVE_CUSTOM_DOMAIN_MODE` | `auto_tls` | `auto_tls`: DNS-verified domains with automatic HTTPS on 80/443. `port`: each domain gets its own port, for hosts whose edge does HTTPS. See [Domains and HTTPS](../deploy/domains.md) |
| `MANGROVE_DDNS_DOMAIN` | empty (off) | DuckDNS subdomain to keep pointed at this box, without `.duckdns.org` |
| `MANGROVE_DDNS_TOKEN` | empty | DuckDNS token |
| `MANGROVE_DDNS_PROVIDER` | `duckdns` | Only `duckdns` is supported |

## GitHub

| Variable | Default | Meaning |
|---|---|---|
| `MANGROVE_GITHUB_OAUTH_CLIENT_ID` / `_SECRET` | empty | Turns on **Connect GitHub**. Callback URL: `https://<dashboard>/api/github/oauth/callback`. Pasted tokens work without these |
| `MANGROVE_PREVIEW_MAX_AGE_HOURS` | `168` | Delete a PR preview after this long without a push. `0` turns it off |

## Email

| Variable | Default | Meaning |
|---|---|---|
| `MANGROVE_RESEND_API_KEY` | empty (off) | Resend API key for deploy emails |
| `MANGROVE_NOTIFY_EMAIL` | empty | Who gets them |
| `MANGROVE_RESEND_FROM` | `Mangrove <onboarding@resend.dev>` | Sender address |
| `MANGROVE_BASE_DOMAIN` | `evanxdsouza.hackclub.app` | Only used to suggest a domain in emails |

## Storage helper

Read by Mangrove and by `mangrove-mountd`. Change them only if you've moved
things.

| Variable | Default | Meaning |
|---|---|---|
| `MANGROVE_MOUNTD_SOCKET` | `/run/mangrove-mountd.sock` | Where the helper listens |
| `MANGROVE_MOUNTD_ROOT` | `/var/lib/mangrove-drives` | Where drives are mounted (helper only) |
| `MANGROVE_MOUNTD_GROUP` | `mangrove-mount` | Group allowed to use the socket (helper only) |

## Set in the unit file

`deploy/systemd/mangrove.service` also sets `HOME=/var/lib/mangrove` (so
Docker finds its config), `GOMEMLIMIT=320MiB` (so Go's garbage collector
stays inside the slice), and the static sites directory.

## Clients

Read by `mangrovectl`, `mangrove-tui` and `mangrove-mcp`, not the server:

| Variable | Default | Meaning |
|---|---|---|
| `MANGROVE_API_URL` | `http://127.0.0.1:7777` | Where the API is |
| `MANGROVE_EMAIL` / `MANGROVE_PASSWORD` | empty | Sign-in for `mangrove-mcp` when there's no saved session |
| `MANGROVE_MCP_TRANSPORT` | `stdio` | `http` to serve MCP over HTTP |
| `MANGROVE_MCP_URL_TOKEN` | none | Required for `http`: the secret path segment |
| `MANGROVE_MCP_LISTEN_ADDR` | `127.0.0.1:7778` | Where the HTTP transport listens |

See [MCP for agents](../interfaces/mcp.md).

## Installer only

`setup.sh` reads `MANGROVE_ADMIN_EMAIL` and `MANGROVE_ADMIN_PASSWORD` to
create the first account without prompting. Mangrove itself never reads
them.
