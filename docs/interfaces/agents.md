# Docs for agents

Two kinds of agent work with Mangrove: agents that **operate** a running
install, and agents that **change Mangrove's code**. Each has its own way in.

## Operating Mangrove

Use the [MCP server](mcp.md). It gives an agent status, logs, deploys,
rollbacks, scaling and one-off commands, and nothing destructive.

Tips worth putting in an agent's instructions:

- **Read before acting.** `get_deployment` and `list_deploy_history` show
  the current state and what changed; `get_logs` shows why something
  failed.
- **Roll back before debugging.** A failed deploy leaves the old version
  running. When a working deploy has gone bad, `rollback` to the last
  successful run, then investigate.
- **Variables need a redeploy.** Changing them doesn't affect the running
  container.
- **`run_command` is one-shot.** Keep commands short and non-interactive.

## Working on the code

This repository is set up for coding agents:

- **[CLAUDE.md](../../CLAUDE.md)** at the repository root: the working rules.
  Read [REPO_MAP.md](../REPO_MAP.md) first, and keep it accurate in the same
  change.
- **[REPO_MAP.md](../REPO_MAP.md)**: every directory and what it's for, how
  to build, run and test each part, where to make each kind of change, and
  a dated snapshot of what was last verified working.
- **The Internals docs** listed in the [docs index](../README.md): the
  reasoning behind each subsystem. Read the relevant one before changing
  it.
- **[frontend.md](../frontend.md)** for the dashboard, and
  [DESIGN.md](../../DESIGN.md) for how it should look.

### Rules that matter

- **Never test against a production install.** Run a second instance with
  its own data directory, port, Docker network and port range, and point
  its Caddy admin address at a closed port. See
  [frontend.md](../frontend.md#qa-against-a-throwaway-instance) and
  [deployment.md](../deployment.md#verifying-without-touching-production).
- **Don't run `./e2e/run.sh` on a box that runs Mangrove for real.** Its
  cleanup force-removes every `mangrove-*` container.
- **Don't reload Caddy** on a live box. It drops every app's route. See
  [Going to production](../operate/production.md#caddy-never-reload-casually).
- **Migrations only move forward.** Add a new numbered file in
  `internal/db/migrations/`; never edit one that has shipped.
- **The API enforces permissions.** Hiding a button isn't access control;
  add the role check in `internal/api/router.go`.

## Keeping these docs current

When a change adds, removes or renames something users see, update the
matching page in `docs/` (see the [index](../README.md)) in the same
change. Design-level changes go in the Internals docs, and structural ones
in `REPO_MAP.md`.
