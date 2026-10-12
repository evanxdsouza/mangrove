# HTTP API

Everything the dashboard does goes through a JSON API under `/api`. Use it
for scripts and integrations the CLI doesn't cover. The authoritative list
of routes, with the role each needs, is `internal/api/router.go`.

## Authentication

The API uses the same session cookie as the dashboard. There are no API
tokens.

```sh
curl -c jar -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"..."}' \
  https://mangrove.example.com/api/auth/login

curl -b jar https://mangrove.example.com/api/projects
```

The cookie is `mangrove_session` and lasts 30 days. Sign-in is limited to
5 attempts per 5 minutes per IP, so sign in once and reuse the cookie.
`mangrovectl login` stores the same token in `~/.mangrove/session`.

## Conventions

- Request and response bodies are JSON. Send `Content-Type: application/json`.
- Errors have a status code and a body of `{"error": "message"}`.
- `204 No Content` means success with nothing to return.
- A resource in a workspace you can't access returns `404`, the same as one
  that doesn't exist.
- IDs are integers. Deployments and projects also have slugs, but the API
  addresses them by ID.

## Main endpoints

### Account

| Method and path | What |
|---|---|
| `GET /api/auth/status` | Whether first-run setup is still needed (no sign-in) |
| `POST /api/auth/setup` | Create the first account |
| `POST /api/auth/login` · `POST /api/auth/logout` | Sign in, sign out |
| `GET /api/auth/me` | The signed-in user |
| `POST /api/auth/change-password` | `{current_password, new_password}` |

### Workspaces and projects

| Method and path | What |
|---|---|
| `GET` · `POST /api/workspaces` | List, create |
| `DELETE /api/workspaces/{id}` | Delete (projects move to the default workspace) |
| `GET` · `POST /api/workspaces/{id}/members`, `PUT` · `DELETE …/members/{userID}` | Membership |
| `GET /api/workspaces/{id}/audit-log` | Workspace activity |
| `GET` · `POST /api/projects`, `GET` · `DELETE /api/projects/{id}` | Projects |
| `POST /api/projects/{id}/workspace` | Move to another workspace |
| `GET` · `POST /api/projects/{id}/deployments` | List, create deployments |
| `GET` · `POST /api/projects/{id}/repo` | Linked GitHub repo |
| `POST /api/projects/{id}/templates/{key}/install` | Install a template |

### Deployments

| Method and path | What |
|---|---|
| `GET` · `DELETE /api/deployments/{id}` | Get, delete |
| `GET /api/deployments/{id}/services` · `/history` | Services, runs |
| `POST /api/deployments/{id}/deploy` | Deploy (`git_url`, `git_ref`, `commit_sha`, `commit_message`, `auth_token`, all optional) |
| `POST /api/deployments/{id}/redeploy` | Redeploy the configured source |
| `POST /api/deployments/{id}/cancel` · `/stop` · `/restart` | Lifecycle |
| `POST /api/deployments/{id}/scale` | `{replicas}` |
| `POST /api/deployments/{id}/access` | Public, internal, password, public paths |
| `POST /api/deployments/{id}/sleep` · `/self-heal` | Idle sleep, self-healing |
| `POST /api/deployments/{id}/repo` | Branch and auto-deploy |
| `GET` · `POST /api/deployments/{id}/staging`, `POST …/promote` | Staging |
| `GET …/previews`, `POST …/pr-previews` | PR previews |
| `GET` · `POST /api/deployments/{id}/domains`, `POST /api/domains/{id}/verify`, `DELETE /api/domains/{id}` | Custom domains |
| `POST /api/deploy-history/{id}/rollback` | Roll back to a run |

### Services

| Method and path | What |
|---|---|
| `GET /api/services/{id}` | A service |
| `GET /api/services/{id}/env` | Variables (secret values omitted) |
| `PUT` · `DELETE /api/services/{id}/env/{key}` | `{value, is_secret}` |
| `GET /api/services/{id}/health` | Recent health checks |
| `GET /api/services/{id}/logs/stream?tail=200` | Live logs (Server-Sent Events) |
| `POST /api/services/{id}/exec` | Run a one-off command |
| `GET /api/services/{id}/terminal` | Interactive shell (WebSocket) |

### Admin

| Method and path | What |
|---|---|
| `GET /api/admin/resource-budget` · `/resource-history?hours=N` · `/system-health` | Usage |
| `GET /api/admin/notifications` | Email log |
| `GET` · `POST /api/admin/users`, `DELETE …/users/{id}` | Accounts (owners) |
| `GET /api/admin/sessions`, `DELETE …/sessions/{id}` | Sessions (owners) |
| `GET` · `POST /api/admin/ports`, `DELETE …/ports/{port}` | Port registry (owners) |
| `POST /api/admin/prune` | Prune images (owners) |
| `GET /api/admin/backup` | Download a backup (owners) |
| `GET /api/admin/audit-log` | All activity (owners) |
| `/api/storage/*` | Drives and NAS shares (owners) |

### No sign-in

| Method and path | What |
|---|---|
| `GET /healthz` | `200` when Mangrove is up |
| `POST /webhooks/github/{token}` | GitHub deliveries, HMAC-verified |

## Stability

The API is the dashboard's own backend, not a versioned public API. Fields
can change between versions. The Go types in `internal/models` and the
typed client in `internal/apiclient` always match the server they were
built with.
