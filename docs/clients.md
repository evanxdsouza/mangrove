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

**Shell (`t`)** opens the same interactive-terminal websocket described in
architecture.md's "Lifecycle actions short of a full deploy" (`GET
/api/services/{id}/terminal`) and bridges it to the real local terminal: raw
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

Two transports, selected by `MANGROVE_MCP_TRANSPORT` (default `stdio`):

- **stdio** -- a locally-spawned process, for Claude Code/Desktop. This is
  the mode below.
- **http** -- a long-running server for remote clients that can't spawn a
  local process, namely claude.ai's web app. See "Remote access (http
  transport)" below.

### stdio transport (local clients)

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

### Remote access (http transport)

claude.ai's web app can't spawn a local stdio process, so reaching it from
there needs an HTTPS endpoint instead. `mangrove-mcp` supports this as a
second transport, backed by `mcp.NewStreamableHTTPHandler` from the SDK
(`cmd/mangrove-mcp/http.go`) -- no protocol code of our own.

**Auth is a token in the URL, not OAuth.** claude.ai's remote-connector
client only starts an OAuth 2.1 + dynamic-client-registration flow if the
server ever answers with a 401 and a `WWW-Authenticate` challenge; if it
never does, the client just connects. Standing up a real OAuth
authorization server (token/code endpoints, `redirect_uri` validation,
dynamic client registration) is real attack surface for a tool only one
person will ever use, so instead the auth secret lives in the URL path
itself: only `/mcp/<token>` is live, everything else (including a wrong
token) gets a plain 404 -- indistinguishable from the path not existing.
Rotating access means changing `MANGROVE_MCP_URL_TOKEN` and restarting the
service, then updating the connector's URL in claude.ai; there's no
per-request revocation, an acceptable tradeoff for a single shared secret
guarding a single owner's own box. Treat the full URL like a password --
whoever has it can trigger everything in the tool list above, including
`redeploy`/`rollback`/`run_command`.

The SDK's DNS-rebinding guard (403 for a non-localhost `Host` header on a
loopback listener) is disabled in `http.go`, since every request behind the
reverse proxy arrives that way; the URL token is the gate instead.

Env vars (http mode only):

| Var | Required | Default | Purpose |
|---|---|---|---|
| `MANGROVE_MCP_TRANSPORT` | for http mode | `stdio` | Set to `http` to enable this transport. |
| `MANGROVE_MCP_URL_TOKEN` | yes | -- | High-entropy path token (`openssl rand -hex 32`). Startup fails fast without one. |
| `MANGROVE_MCP_LISTEN_ADDR` | no | `127.0.0.1:7778` | Loopback by default -- a reverse proxy is what actually faces the internet. |
| `MANGROVE_EMAIL` / `MANGROVE_PASSWORD` | yes | -- | Required in http mode (not just a `~/.mangrove/session` fallback) -- a long-running service re-authenticates every 24h on its own (`keepSessionAlive` in `main.go`) so it never goes stale between restarts, well inside the 30-day session TTL (`internal/auth.SessionTTL`). |

Run it as `deploy/systemd/mangrove-mcp.service` (its own doc comment has
the install steps) alongside the main `mangrove.service`. It listens only
on loopback -- getting it onto the internet is an operator step outside
Mangrove's own dynamic Caddy code (`internal/proxy/caddy.go` only manages
per-deployment routes): reverse-proxy a port to
`MANGROVE_MCP_LISTEN_ADDR` the same way you already expose the dashboard
itself (e.g. one more block in a hand-maintained Caddyfile, or a port
registered against a hostname in Nest's dashboard on a Nest-style install
-- see `docs/deployment.md`'s port-mode section).

Once `https://<your-host>/mcp/<token>` resolves, add it in claude.ai under
Settings -> Connectors -> Add custom connector.
