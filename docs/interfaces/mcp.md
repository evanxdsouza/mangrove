# MCP for agents

`mangrove-mcp` lets AI agents such as Claude Code, Claude Desktop and
claude.ai operate Mangrove through the
[Model Context Protocol](https://modelcontextprotocol.io).

```sh
go build -o mangrove-mcp ./cmd/mangrove-mcp
```

## Tools

| Tool | What it does |
|---|---|
| `list_workspaces`, `list_projects`, `get_project` | Browse |
| `list_deployments`, `get_deployment` | Deployments and their status |
| `list_services`, `get_service` | Services |
| `list_deploy_history` | Runs |
| `get_logs` | Recent container output |
| `redeploy`, `rollback` | Ship or revert |
| `restart_deployment`, `stop_deployment`, `scale_deployment` | Lifecycle |
| `run_command` | A one-off command inside a container |

It's an **operations** surface. There are deliberately no tools to delete
anything, create deployments from scratch, install templates, manage users
or secrets, or change domains: the actions hardest to undo if a model
misreads a situation. There's no interactive shell either; `run_command`
returns output when the command finishes.

The agent acts as the account it signs in with, and the API applies that
account's [roles](../teams/workspaces-and-roles.md). Give it a dedicated
account with only the workspace roles it needs.

## Local: Claude Code and Claude Desktop

The default transport is stdio: the client starts `mangrove-mcp` itself.

```json
{
  "mcpServers": {
    "mangrove": {
      "command": "/path/to/mangrove-mcp",
      "env": {
        "MANGROVE_API_URL": "https://mangrove.example.com",
        "MANGROVE_EMAIL": "agent@example.com",
        "MANGROVE_PASSWORD": "..."
      }
    }
  }
}
```

For Claude Code, put that in `.mcp.json` or use `claude mcp add`. Leave out
the email and password if `~/.mangrove/session` already holds a session
from `mangrovectl login`.

The password is read once at startup. It's never a tool argument, so it
can't end up in a transcript.

## Remote: claude.ai

claude.ai can't start local processes, so it needs an HTTPS URL. Run
`mangrove-mcp` as a service in HTTP mode:

```sh
MANGROVE_MCP_TRANSPORT=http
MANGROVE_MCP_URL_TOKEN=$(openssl rand -hex 32)
MANGROVE_MCP_LISTEN_ADDR=127.0.0.1:7778
MANGROVE_EMAIL=agent@example.com
MANGROVE_PASSWORD=...
```

1. Install `deploy/systemd/mangrove-mcp.service` (its comments have the
   steps) next to `mangrove.service`.
2. Put a reverse proxy with HTTPS in front of `127.0.0.1:7778`, the same way
   you expose the dashboard.
3. In claude.ai: **Settings → Connectors → Add custom connector**, with the
   URL `https://<your-host>/mcp/<token>`.

**The URL is the password.** Only `/mcp/<token>` answers; every other path,
including a wrong token, returns 404. Anyone with the URL can redeploy,
roll back and run commands. To revoke access, change
`MANGROVE_MCP_URL_TOKEN`, restart the service, and update the connector.

The service signs in again every 24 hours, so it never goes stale.

More detail: [clients.md](../clients.md#mangrove-mcp).
