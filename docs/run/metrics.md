# Metrics and health

Mangrove shows how each app and the box as a whole are doing. It doesn't
have alerting rules; for alerts on an app, run a monitor like
[Uptime Kuma](templates.md) against it.

## Per deployment

On a deployment's **Overview** tab, each service shows:

- its latest **health check**: healthy or not, and the response time;
- its CPU and memory limits, replica count, image, ports and internal
  address.

Health checks run every 30 seconds by default and are kept for 7 days. See
[Health checks and self-healing](../deploy/health-checks.md).

The **History** tab shows every run, and the deployment's header shows a
sparkline of its recent deploy outcomes.

## The box

**Server health** in the sidebar shows the whole machine: CPU, memory,
swap, disk, load average and uptime, refreshed every few seconds.

**Admin → Resource budget** shows Mangrove's own view:

- memory **allocated** to deployments (their limits) against the
  [budget ceiling](../deploy/scaling.md#the-memory-budget);
- memory **actually used** by containers;
- running containers and disk usage;
- a **history** of all of these, sampled every 5 minutes and kept for 30
  days.

## Warnings

- For owners, the Mangrove mark in the sidebar turns red, with a link to
  Server health, when the box's memory or disk use goes over **90%**. It
  checks every 5 minutes.
- A deploy that would exceed the memory budget is refused with an error
  rather than allowed to starve the box.

## Getting metrics out

There's no Prometheus endpoint. The same numbers are available from the
API (`GET /api/admin/system-health`, `/api/admin/resource-budget`,
`/api/admin/resource-history?hours=24`) if you want to scrape them. See
[HTTP API](../interfaces/api.md).
