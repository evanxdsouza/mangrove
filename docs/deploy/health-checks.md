# Health checks and self-healing

A health check is how Mangrove knows a container is actually working, not
just running.

## Setting one

Give a service a **health check path**, such as `/healthz` or `/`, when you
create it. Mangrove sends an HTTP `GET` to that path on the container's
internal port. Any `2xx` or `3xx` response is healthy.

No path? Mangrove waits three seconds and treats "still running" as
healthy. That catches a container that crashes on start, but not one that
starts and then hangs, so set a path whenever the app has one.

## During a deploy

The new version starts next to the old one and is checked repeatedly for
up to **60 seconds**:

- **Healthy**: Caddy switches traffic to it, then the old version is
  removed.
- **Never healthy**: the new version is removed. The old one never stopped
  serving.

## While running

Mangrove keeps checking each running service on its interval (30 seconds by
default). The latest result and response time show on the deployment's
**Overview** tab. Results are kept for 7 days.

A container that crashes is restarted by Docker automatically. A container
that stays up but stops answering is only acted on if you turn on
self-healing.

## Self-healing

Turn on the **Self-healing** card on a deployment's Overview tab. It does
two things:

1. **Restarts an unhealthy service.** After **3** failed health checks in a
   row, Mangrove restarts the deployment, the same as pressing **Restart**.
2. **Retries a failed deploy.** If a deploy fails, Mangrove tries again
   after 1 minute, then after 5 more minutes, then gives up. Retries show in
   the **History** tab as `auto-retry`. Any successful deploy resets the
   count.

Both are off by default, and one switch controls both. The thresholds are
fixed.

Retries reuse the deployment's linked repository or image, so they don't
work for deployments built from a [non-GitHub URL](from-other-git.md).

How it works underneath: [self-healing.md](../self-healing.md).
