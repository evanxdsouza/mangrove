# Scaling and resources

Mangrove runs on one box, so the questions are how much of it each app may
use and how many copies of it run.

## Memory and CPU limits

Every service has a **memory limit** (MB) and a **CPU limit** (cores), set
when you create the deployment (default 256 MB and 0.5 cores). Docker
enforces both: a service that goes over its memory limit is killed and
restarted.

Limits can't be edited after creation yet. To change them, create a new
deployment with the new limits.

Compose stacks are the exception: Mangrove doesn't inject limits into them.
Set them in the compose file under `deploy.resources.limits`.

## The memory budget

Mangrove adds up the memory limits of every deployment that isn't stopped
(limit × replicas). If a new deploy would push that total over
`MANGROVE_DEPLOYMENT_MEMORY_CEILING_MB` (default `1536`), it's refused before
anything starts, with a clear error, instead of the box running out of
memory later.

In production a systemd slice enforces the same ceiling at the kernel
level as a backstop. See [Going to production](../operate/production.md).

To free budget:

- **Stop** a deployment you don't need right now. Stopped deployments
  don't count.
- Delete deployments you're done with.

[Idle sleep](../run/idle-sleep.md) saves real memory while an app is asleep,
but a sleeping deployment still counts toward the budget, because it can
wake at any moment.

Anyone signed in can see the budget, what's using it, and a 30-day history
under **Admin → Resource budget**.

## Replicas

The **Replicas** card on a deployment's Overview tab runs 1 to 32 copies of
it. Changing the count triggers a deploy: the new set of containers starts,
passes health checks, and only then replaces the old set.

- Only **Dockerfile**, **Nixpacks** and **Existing image** deployments can
  scale. Compose stacks and static sites stay at one.
- Each replica counts against the memory budget.
- All replicas share the deployment's port, and Caddy spreads requests
  across them.

From the CLI or TUI: `+` and `-` on a deployment in `mangrove-tui`, or the
`scale_deployment` MCP tool.

## Seeing usage

- **Overview** tab: each service's limits, replica count, and its latest
  health check with response time.
- **Admin → Resource budget**: memory allocated to deployments
  against memory actually used, running containers, disk, and their
  history.
- **Server health** (sidebar): the whole box's CPU, memory, swap, disk,
  load and uptime.
- The sidebar's Mangrove mark turns red for owners when the box's memory or
  disk passes 90%.

See [Metrics and health](../run/metrics.md).
