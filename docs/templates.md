# One-click templates

Templates are the JSON files in `internal/templates/data/*.json`, embedded
into the binary at build time (`//go:embed data/*.json`) -- not database
rows, not admin-editable at runtime. Adding one is "add a file, rebuild,"
with no migration or seed script.

Installing a template (`POST /api/projects/{id}/templates/{key}/install`)
creates one or more real deployment/service/volume/env rows and deploys
them through the exact same `Orchestrator.Deploy` path (and so the same
admission control and port registry) as a hand-configured deployment --
see [architecture.md](architecture.md).

## Schema

A template file is one JSON object:

```jsonc
{
  "key": "vaultwarden",          // must match the filename's stem; used in the install URL and as the map key
  "name": "Vaultwarden",         // shown in the gallery
  "description": "...",          // shown in the gallery -- technical mode only, see modes.md
  "category": "security",        // free-text grouping, shown in the gallery
  "deployments": [ /* one or more Deployment objects, see below */ ]
}
```

Each entry in `deployments` describes one deployment to create:

| Field | Required | Notes |
|---|---|---|
| `slug_suffix` | yes | Appended to the user's chosen base slug. **Exactly one** deployment in the template must have `""` here (the primary), and it **must be listed last** -- see "Ordering" below. |
| `name_suffix` | no | Appended to the deployment's display name (e.g. `" MySQL"`). |
| `build_strategy` | yes | Almost always `"image"` (deploy a pre-built image, no build step) -- not hardcoded, so a future template could use another strategy. |
| `image_ref` | yes | e.g. `"vaultwarden/server:latest"`. |
| `internal_port` | yes | Container's listening port. |
| `force_internal_only` | no | Set for anything speaking raw TCP rather than HTTP (Postgres, MySQL, MongoDB, Redis) -- Caddy only reverse-proxies/file-serves HTTP, so there's no working way to expose these publicly through it. Still user-togglable after install via the normal access-control endpoint; this only sets the *default*. |
| `memory_limit_mb` / `cpu_limit_cores` | yes | Same admission-control/cgroup limits a hand-created deployment would set. |
| `health_check_path` | no | Skip for anything with no meaningful HTTP health endpoint. |
| `command` | no | Overrides the image's default command (array of args). |
| `volumes` | no | `[{ "name": "data", "mount_path": "/data" }]` -- named Docker volumes. |
| `env` | no | See below. |

### Env vars: literals, generated secrets, and cross-deployment references

Each entry is `{ "key": ..., "value" | "generate", "generate_key", "secret" }`
-- **either `value` or `generate` is set, never both**:

- `"value"`: a literal string, which may contain placeholders resolved at
  install time and can appear inline within a larger string (e.g. a
  combined connection URL):
  - `{{alias:<slug_suffix>}}` -- a sibling deployment's stable
    network/container alias (stable across redeploys, unlike the
    per-deploy container name), so one deployment can address another by
    name over the Docker network.
  - `{{generated:<generate_key>}}` -- a value produced by an earlier
    deployment's `generate` (see below).
  - `{{slug}}` -- this deployment's own final slug.
  - `{{base_domain}}` -- `Config.BaseDomain`.
- `"generate": "password"` or `"generate": "hex32"`: produces a random
  value at install time, stored under `"generate_key"` so a *later*
  deployment in the same template can reference it via
  `{{generated:<generate_key>}}`.
- `"secret": true`: stored encrypted, same as a manually-added secret env
  var (see the multi-user doc for who can view/set these).

### Ordering

Validation (`internal/templates/templates.go`'s `validate`, run once at
`init()` -- an invalid template file panics the binary on startup, not at
install time) enforces:

1. Exactly one deployment has `slug_suffix == ""` (the primary), and it's
   the **last** entry in the array.
2. Every `{{generated:key}}` placeholder references a `generate_key` set
   by an *earlier* deployment in the same array.
3. Every deployment has a non-empty `image_ref` and a positive
   `memory_limit_mb`.

The ordering rule exists because deployments install in array order, and
a later deployment's env (like WordPress's `WORDPRESS_DB_HOST` referencing
`{{alias:-mysql}}`) needs its dependency to already exist and have a
resolvable network alias. Putting the primary last means "the thing the
user actually opens" is the last one created, after all its dependencies.

## Example: a linked two-deployment template

`wordpress.json` -- MySQL first (dependency), WordPress second (primary,
`slug_suffix: ""`), referencing MySQL's generated password and stable
alias:

```json
{
  "key": "wordpress",
  "name": "WordPress",
  "description": "WordPress with a linked MySQL database, deployed as two connected deployments in this project.",
  "category": "cms",
  "deployments": [
    {
      "slug_suffix": "-mysql",
      "name_suffix": " MySQL",
      "build_strategy": "image",
      "image_ref": "mysql:8",
      "internal_port": 3306,
      "force_internal_only": true,
      "memory_limit_mb": 320,
      "cpu_limit_cores": 0.4,
      "volumes": [{ "name": "data", "mount_path": "/var/lib/mysql" }],
      "env": [
        { "key": "MYSQL_ROOT_PASSWORD", "generate": "password", "generate_key": "db_root_password", "secret": true },
        { "key": "MYSQL_DATABASE", "value": "wordpress" },
        { "key": "MYSQL_USER", "value": "wordpress" },
        { "key": "MYSQL_PASSWORD", "generate": "password", "generate_key": "db_user_password", "secret": true }
      ]
    },
    {
      "slug_suffix": "",
      "build_strategy": "image",
      "image_ref": "wordpress:6-apache",
      "internal_port": 80,
      "memory_limit_mb": 320,
      "cpu_limit_cores": 0.5,
      "health_check_path": "/",
      "volumes": [{ "name": "data", "mount_path": "/var/www/html" }],
      "env": [
        { "key": "WORDPRESS_DB_HOST", "value": "{{alias:-mysql}}:3306" },
        { "key": "WORDPRESS_DB_USER", "value": "wordpress" },
        { "key": "WORDPRESS_DB_PASSWORD", "value": "{{generated:db_user_password}}", "secret": true },
        { "key": "WORDPRESS_DB_NAME", "value": "wordpress" }
      ]
    }
  ]
}
```

## Adding a new template

1. Drop a new `internal/templates/data/<key>.json` file following the
   schema above. The filename's stem should match `"key"`.
2. `go build ./...` -- an invalid template panics on startup (fails fast
   at build/boot time, not silently at install time), so this alone
   catches most schema mistakes.
3. If it's meant to be user-facing (not a bare backing database), add it
   to `SIMPLE_TEMPLATES` in `web/src/pages/simple/plainCopy.ts` with a
   plain-language label and one-sentence blurb -- otherwise it only shows
   up in the technical dashboard's gallery. See
   [modes.md](modes.md) for why databases are deliberately left out of
   simple mode.
4. `go test ./...` -- `internal/orchestrator/templates_test.go` exercises
   the install path against a fake executor; add a case there for
   anything with new install-time behavior (e.g. a new `generate` kind).
