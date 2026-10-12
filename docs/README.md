# Mangrove docs

Mangrove is a self-hosted platform for running your apps on one box you
own. These docs are task-first: each page says what something does, how to
use it, and what to do when it breaks. The **Internals** pages at the
bottom explain how things work underneath.

## Start

- [What is Mangrove](start/what-is-mangrove.md)
- [Install](start/install.md)
- [First deploy](start/first-deploy.md)
- [Concepts](start/concepts.md)
- [Simple mode](start/simple-mode.md)

## Deploy

- [From GitHub](deploy/from-github.md)
- [From GitLab, Gitea and others](deploy/from-other-git.md)
- [From an image](deploy/from-an-image.md)
- [Builds](deploy/builds.md)
- [Domains and HTTPS](deploy/domains.md)
- [Networking](deploy/networking.md)
- [Scaling and resources](deploy/scaling.md)
- [Storage](deploy/storage.md)
- [Rollbacks](deploy/rollbacks.md)
- [Health checks and self-healing](deploy/health-checks.md)
- [Protection](deploy/protection.md)

## Data

- [Databases](data/databases.md)
- [Backups and restore](data/backups.md)

## Run

- [Environments and variables](run/environments.md)
- [Compose stacks](run/compose.md)
- [Templates](run/templates.md)
- [Idle sleep](run/idle-sleep.md)
- [Logs and shell](run/logs-and-shell.md)
- [Metrics and health](run/metrics.md)
- [Notifications](run/notifications.md)
- [Webhooks](run/webhooks.md)

## Account

- [Profile and settings](account/profile.md)
- [Sign-in and sessions](account/sign-in.md)

## Teams

- [Workspaces and roles](teams/workspaces-and-roles.md)
- [Audit logs](teams/audit-logs.md)

## Operate

- [Architecture](operate/architecture.md)
- [Configuration](operate/configuration.md)
- [Host commands](operate/host-commands.md)
- [Upgrades](operate/upgrades.md)
- [Going to production](operate/production.md)
- [Security and isolation](operate/security.md)
- [Troubleshooting](operate/troubleshooting.md)

## Interfaces

- [CLI and TUI](interfaces/cli.md)
- [HTTP API](interfaces/api.md)
- [MCP for agents](interfaces/mcp.md)
- [Docs for agents](interfaces/agents.md)

## Internals

These are longer, design-level documents. Code comments link to them by
path, so they stay where they are.

| Page | Covers |
|---|---|
| [architecture.md](architecture.md) | Request path, the blue/green deploy flow, lifecycle actions, GitHub auto-deploy, staging, PR previews |
| [deployment.md](deployment.md) | The production systemd setup, memory slices, binary swap, custom domains, DDNS |
| [multi-user.md](multi-user.md) | Global and workspace roles, how they are enforced, the audit log |
| [protected-deployments.md](protected-deployments.md) | The password gate, public paths, the account handoff |
| [idle-sleep.md](idle-sleep.md) | Auto-stop and wake-on-request |
| [self-healing.md](self-healing.md) | Health-triggered restarts and automatic deploy retries |
| [templates.md](templates.md) | The template file format |
| [storage.md](storage.md) | The drive-to-NAS feature and its privileged helper |
| [backup.md](backup.md) | What a backup contains and why |
| [clients.md](clients.md) | `mangrovectl`, `mangrove-tui`, `mangrove-mcp` in depth |
| [modes.md](modes.md) | Technical vs. simple mode |
| [frontend.md](frontend.md) | Working on the dashboard (`web/`) |
| [REPO_MAP.md](REPO_MAP.md) | Where everything lives in the repo, and what was last verified |
| [../DESIGN.md](../DESIGN.md) | The dashboard's visual language |
