# Networking

How traffic reaches your apps, and how your apps reach each other.

```
Internet ─▶ Caddy :<port> or :443 (custom domain) ─▶ container on mangrove-net
                                                        │
                                   other containers ◀───┘  (by alias)
```

## Public and internal-only

A service is either:

- **Public**: Caddy listens on its host port (and any custom domains) and
  forwards to the container. Its own port is never published to the
  network directly.
- **Internal only**: no host port, no route. Only other containers on
  Mangrove's network can reach it. Databases from templates are internal
  only by default.

Switch with the **Public** checkbox on the deployment's **Access control**
card. Owners and workspace admins can change it.

## Talking to other services

Mangrove runs every container on one Docker network, `mangrove-net` (set
with `MANGROVE_NETWORK`). Each service gets a stable network alias that
survives redeploys:

```
mangrove-<deployment-slug>-<service-name>
```

Services created by templates are named `app`, so a Postgres template
installed with the slug `blog-db` is reachable at `mangrove-blog-db-app:5432`.
Templates that install several deployments fill these in for you.

The service card on the Overview tab shows it as **Internal address**.
Use the alias, never a container IP or the per-deploy container name.
Both change on every deploy.

## Compose stacks

A compose stack runs on the network its compose file defines (by default,
a network of its own), and services inside it reach each other by their
compose service names. Ports listed under `ports:` in the compose file are
published by Docker directly, not through Caddy. See
[Compose stacks](../run/compose.md).

## The Mangrove API

Mangrove's own API and dashboard listen on `127.0.0.1:7777` only. Put them
behind HTTPS to reach them from elsewhere. See
[Going to production](../operate/production.md#serve-the-dashboard-over-https).

## NAS shares

A [NAS share](storage.md#share-a-drive-as-a-nas) is the one exception that
publishes a port (445, SMB) straight onto the box's network interface,
because SMB isn't HTTP and Caddy can't route it.
