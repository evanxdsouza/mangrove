# Idle sleep

Let a deployment stop itself when nobody's using it, and start again when
someone visits. Useful for side projects and demos on a small box.

## Turn it on

On the deployment's Overview tab, turn on **Idle sleep** and choose how many
minutes without traffic before it sleeps (30 by default).

- After that long with no requests, Mangrove stops the container. Its
  status becomes **sleeping**.
- The next visitor sees a short "waking up" page that refreshes itself
  every 3 seconds. Mangrove starts the container in the background, and the
  next refresh goes straight through to the app.
- **Wake up** on the deployment's page starts it ahead of time.

A fresh deploy, or turning sleep on, counts as activity, so a deployment
never falls asleep the moment you enable it.

## With password protection

A protected deployment checks the password first. Visitors who aren't
signed in see the gate, not the waking page, so strangers can't keep
waking your app.

## Limits

- **Single-service deployments only.** Compose stacks can't sleep, and
  sleep does nothing useful for a static site, which has no container.
- **Wakes take a few seconds** (a container start, not a rebuild). Don't use
  sleep for anything that must answer instantly, like a webhook receiver
  with a short timeout.
- **A sleeping deployment still counts toward the
  [memory budget](../deploy/scaling.md#the-memory-budget)**, because it can
  wake at any time. Stop it instead to free budget.
- Only available in technical mode. Not exposed in the CLI, TUI or MCP.

How it works: [idle-sleep.md](../idle-sleep.md).
