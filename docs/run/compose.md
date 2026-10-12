# Compose stacks

Deploy a `docker-compose.yml` as one deployment: several services that
start, stop and deploy together.

## Deploy one

Create a deployment with the **Docker Compose** strategy, from a GitHub
repo that has a compose file. Detection picks it automatically if the repo
has `docker-compose.yml`, `docker-compose.yaml`, `compose.yml` or
`compose.yaml`. Set **Compose file path** if yours is elsewhere, and **Root
path** if it's in a subdirectory.

Mangrove clones the repository and runs:

```sh
docker compose -p mangrove-<slug> -f <compose file> up -d --build
```

Each service in the compose file appears as a service of the deployment,
with its own logs and **Run command**.

## How it differs from other deployments

Compose stacks are deliberately handed to Docker Compose as they are, so
several Mangrove features don't apply:

| | Single-service deployment | Compose stack |
|---|---|---|
| Zero-downtime switch-over | Yes | No: changed containers are recreated in place |
| Health-check gate | Yes | No |
| Public port and custom domains via Caddy | Yes | No: use `ports:` in the compose file |
| Password protection, idle sleep | Yes | No |
| Replicas | 1 to 32 | 1 |
| CPU and memory limits | Set in Mangrove | Set in the compose file (`deploy.resources.limits`) |
| Mangrove variables | Given to the container | Not passed; use the compose file or `.env` |

## Exposing a service

Services in a stack aren't routed through Caddy. To reach one from
outside, publish a port in the compose file:

```yaml
services:
  web:
    build: .
    ports:
      - "8080:3000"
```

Docker publishes `8080` on the box directly. Pick ports outside Mangrove's
own range (20000 to 21000 by default) so they don't collide.

## Data

Declare volumes in the compose file as usual. They survive redeploys.
Deleting the deployment removes the stack's containers but leaves its
compose volumes and network on the box. Remove them yourself with
`docker volume ls` / `docker volume rm` once you're sure.

## When to use something else

If your stack is "an app plus a database", two ordinary deployments (or a
[template](templates.md)) get you health-checked deploys, protection,
domains and rollback. Use compose when the services really must be managed
as one unit.
