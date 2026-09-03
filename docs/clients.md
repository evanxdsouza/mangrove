# Clients: mangrovectl, mangrove-tui, mangrove-mcp

The dashboard (`web/`, embedded into the `mangrove` binary -- see
[architecture.md](architecture.md)) is one way to drive Mangrove. This repo
also ships three more, all in `cmd/`, all talking to the exact same HTTP
API over the network (nothing here needs local Docker/SQLite access of its
own) -- this is the "monorepo" of the project's own name: control plane,
dashboard, CLI, TUI, and MCP server, one module, one version, built
together.

| Binary | What it's for |
|---|---|
| `cmd/mangrove` | The control plane itself -- see the rest of the docs. |
| `cmd/mangrovectl` | Scriptable CLI (`mangrovectl deploy ...`) -- predates the other two and has its own small hand-rolled HTTP client, not `internal/apiclient` (see below). |
| `cmd/mangrove-tui` | Full-screen terminal dashboard -- browse, deploy, roll back, scale, tail logs, and open a real shell into a container, without leaving the terminal. |
| `cmd/mangrove-mcp` | MCP (Model Context Protocol) server -- exposes the same operations as tools an LLM agent (Claude Code, Claude Desktop, etc.) can call. |

## internal/apiclient

`mangrove-tui` and `mangrove-mcp` share `internal/apiclient`, a typed Go
client that decodes responses straight into the same `internal/models` /
`internal/store` types the backend itself returns and writes with
`writeJSON` -- not a redeclared shadow struct per client. That's a
deliberate reaction to a real bug: `store.WorkspaceProjectCount` once had
no `json` tags at all, so `GET /api/workspaces` serialized as
`{"Workspace":...,"ProjectCount":...}` while the dashboard's hand-shaped
frontend expectation was `{"workspace":...,"project_count":...}` --
every row silently failed to decode, and with no error boundary anywhere
in the SPA at the time, that took down the *entire* dashboard, not just
the Workspaces tab (see `web/src/components/ErrorBoundary.tsx`). Reusing
the backend's actual Go types for two more clients, rather than writing a
third and fourth hand-shaped guess at the JSON shape, means that class of
bug becomes a compile error here instead of a silent runtime mismatch.

`cmd/mangrovectl` predates `internal/apiclient` and isn't migrated to it --
changing already-working, separately-tested code purely for architectural
symmetry isn't worth the regression risk. Its own client works the same
way apiclient's `do` does (cookie in, cookie out, `~/.mangrove/session` on
disk), just with `map[string]any` instead of typed structs.

## Authentication

All three CLI-ish clients share one session file, `~/.mangrove/session` --
log in once with any of them and the others pick it up:

```sh
mangrovectl login --email you@example.com --password '...'
mangrove-tui                 # already logged in
mangrove-mcp                 # already logged in
```

`mangrove-tui` also has its own login screen (first-run setup or a plain
login form) if no session is present yet, so it doesn't strictly need
`mangrovectl` first.

`mangrove-mcp` is the one exception: it never accepts a password through a
tool call -- an MCP tool argument is something the model constructs and
that can end up in transcripts and logs, which is not where a password
belongs. It authenticates once at process startup instead, from
`~/.mangrove/session` or, failing that, `MANGROVE_EMAIL`/`MANGROVE_PASSWORD`
environment variables.

All three respect `MANGROVE_API_URL` (default `http://127.0.0.1:7777`,
matching `MANGROVE_PORT`'s default) if the API isn't local.

## mangrove-tui

```sh
go build -o mangrove-tui ./cmd/mangrove-tui
./mangrove-tui
```

Navigation: `projects → deployments → deployment detail`, plus detail's
`logs`/`history`/`shell` sub-views. Keys are shown in each view's own help
line at the bottom; the headline ones on the deployment detail view are
`d` redeploy, `R` restart, `x` stop, `+`/`-` scale, `l` live logs, `h`
history (`enter` on an entry rolls back to it), and `t` shell.

**Shell (`t`)** opens the exact same interactive-terminal websocket the
dashboard's xterm.js Terminal tab does (`GET
/api/services/{id}/terminal` -- see architecture.md's "Lifecycle actions
short of a full deploy") and bridges it to the real local terminal: raw
mode via `golang.org/x/term`, `SIGWINCH` forwarded as resize control
messages, the works. It hands the terminal over via bubbletea's
`Program.ReleaseTerminal`/`RestoreTerminal` (see `cmd/mangrove-tui/
terminal.go`'s doc comment for exactly how and the one known rough edge:
once the remote shell exits, returning to the TUI needs one more keypress,
since there's no portable way to cancel a blocked `os.Stdin.Read` from
another goroutine).

## mangrove-mcp

```sh
go build -o mangrove-mcp ./cmd/mangrove-mcp
MANGROVE_EMAIL=you@example.com MANGROVE_PASSWORD='...' ./mangrove-mcp
```

Runs over stdio (the standard MCP transport for a locally-spawned server).
To wire it into an MCP client, point it at the built binary, e.g. for
Claude Code (`.mcp.json` or `claude mcp add`) or Claude Desktop's config:

```json
{
  "mcpServers": {
    "mangrove": {
      "command": "/path/to/mangrove-mcp",
      "env": {
        "MANGROVE_API_URL": "http://127.0.0.1:7777",
        "MANGROVE_EMAIL": "you@example.com",
        "MANGROVE_PASSWORD": "..."
      }
    }
  }
}
```

(Omit `MANGROVE_EMAIL`/`MANGROVE_PASSWORD` if `~/.mangrove/session` already
holds a valid login from `mangrovectl login`.)

Tools exposed: `list_workspaces`, `list_projects`, `get_project`,
`list_deployments`, `get_deployment`, `list_services`, `get_service`,
`list_deploy_history`, `redeploy`, `rollback`, `stop_deployment`,
`restart_deployment`, `scale_deployment`, `run_command`, `get_logs`.

**Deliberately not exposed**: deleting a project or deployment, creating
one from scratch, installing a template, managing users or secrets, custom
domains. This is an *operations* surface (status, deploy, roll back,
scale, shell out for diagnostics via `run_command`/`get_logs`), not a full
API mirror -- the actions left out are exactly the ones where a model
acting on a misread is hardest to undo. There's also no interactive-shell
tool: unlike `mangrove-tui`'s shell view, an MCP tool call is
request/response, not a persistent stream, so `run_command` (one-off,
buffered, `docker exec`) is the model-facing equivalent -- the same
endpoint `RunCommandCard` and `POST /api/services/{id}/exec` use.
