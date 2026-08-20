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
| `build_strategy` | no | `"image"` (default, deploy a pre-built image, no build step) or `"dockerfile"` (build from `git_url` -- see below). |
| `image_ref` | if `build_strategy` is `"image"` | e.g. `"vaultwarden/server:latest"`. |
| `git_url` / `git_branch` | if `build_strategy` is `"dockerfile"` | Cloned the same way a hand-created git-backed deployment is built, with no auth token -- **only public repos work**. `git_branch` is optional; empty clones the repo's actual default branch. `Dockerfile` is expected at the repo root. |
| `internal_port` | yes | Container's listening port. |
| `force_internal_only` | no | Set for anything speaking raw TCP rather than HTTP (Postgres, MySQL, MongoDB, Redis) -- Caddy only reverse-proxies/file-serves HTTP, so there's no working way to expose these publicly through it. Still user-togglable after install via the normal access-control endpoint; this only sets the *default*. |
| `memory_limit_mb` / `cpu_limit_cores` | yes | Same admission-control/cgroup limits a hand-created deployment would set. |
| `health_check_path` | no | Skip for anything with no meaningful HTTP health endpoint. |
| `command` | no | Overrides the image's default command (array of args). |
| `volumes` | no | `[{ "name": "data", "mount_path": "/data" }]` -- named Docker volumes. |
| `files` | no | `[{ "path": "/docker-entrypoint-initdb.d/99-setup.sql", "content": "..." }]` -- small inline files bind-mounted read-only into the container before it first starts. See "Files" below. |
| `env` | no | See below. |
| `post_deploy_commands` | no | `[["sh", "-c", "...", "--", "{{env:KEY}}"], ...]` -- exec-form commands run via `docker exec` against this deployment's own container right after it deploys successfully. See "Post-deploy commands" below. |

### Env vars: literals, generated secrets, prompted values, and cross-deployment references

Each entry is `{ "key": ..., "value" | "generate" | "prompt", "generate_key", "secret", "label", "required" }`
-- **exactly one of `value`, `generate`, or `prompt` is set**:

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
  - `{{base_slug}}` -- the user's chosen base slug, i.e. the primary
    deployment's slug, regardless of which deployment the placeholder
    appears on. A deployment with a suffix (e.g. `-studio`) uses this to
    reference the primary deployment's public URL.
  - `{{base_domain}}` -- `Config.BaseDomain`.
- `"generate": "password"`, `"generate": "hex32"`, or `"generate": "hex64"`:
  produces a random value at install time, stored under `"generate_key"` so
  a *later* deployment in the same template can reference it via
  `{{generated:<generate_key>}}`:
  - `"password"` -- 24 alnum characters, safe to embed directly in a
    connection-string URL or shell command without escaping.
  - `"hex32"` -- 32 lowercase hex characters (16 random bytes), for things
    that want exactly 32 characters (e.g. Postgres-meta's `CRYPTO_KEY`).
  - `"hex64"` -- 64 lowercase hex characters (32 random bytes), for HS256
    JWT secrets / Elixir secret-key-bases that want at least 32 bytes.
  - `"jwt:<role>:{{generated:<secret_key>}}"` -- an HS256-signed Supabase
    API key (role `anon` or `service_role`), signed with the value stored
    under `<secret_key>` by an *earlier* generate (the shared JWT secret).
    PostgREST/GoTrue verify the signature and use the `role` claim, so the
    key must be minted with the exact secret every service shares. It's
    stored (secretly) like any other generated value and shown once at
    install.
- `"prompt": true`: asks the installing user for this value in the install
  form, instead of deriving it from the template -- for things a template
  can't sensibly default (API tokens, webhook secrets). `"label"` is
  optional UI copy; `"required": true` blocks install until a non-empty
  value is supplied (checked upfront, before any rows are created, and
  again in the orchestrator). The value never becomes part of the template
  itself -- the caller passes it into `InstallTemplate` as `env_overrides`,
  keyed by `slug_suffix` then env key (the same shape
  `memory_overrides_mb` uses), and it's substituted in exactly like a
  literal `value` would be.
- `"secret": true`: stored encrypted, same as a manually-added secret env
  var (see the multi-user doc for who can view/set these) -- independent of
  whether the value came from `value`, `generate`, or `prompt`.

### Files: seeding a container before first boot

Some images expect SQL/config present *before* the entrypoint's init
machinery runs (Postgres's `/docker-entrypoint-initdb.d`, the nginx config
dir). A template can't express that as an env var, so deployments can carry
`files`:

- `"path"` -- absolute path inside the container, mounted read-only.
- `"content"` -- inline text, resolved for the same placeholders an env
  var's `value` supports (`{{slug}}`, `{{base_slug}}`, `{{base_domain}}`,
  `{{alias:...}}`, `{{generated:...}}`) at install time.

The executor writes each file to a host temp file and bind-mounts it in
`docker run` (`-v <hostfile>:<path>:ro`). Files are **only** materialized
during the template install itself -- a later plain redeploy of the
resulting deployment carries no files, which is fine because they exist to
seed first boot (e.g. Supabase's role-password SQL that only runs on an
empty database). See `executor.FileMount` and the `supabase.json` template
for a real example (role-password, JWT-setting, and `_realtime`-schema init
scripts mounted into the Postgres container, mirroring the official
compose's `init-scripts/99-*.sql` / `migrations/99-*.sql`).

### Post-deploy commands: seeding state a startup script can't

Some images only read env vars for a single instance of a thing -- e.g. the
`muchobien/pocketbase` image's entrypoint auto-creates exactly **one**
superuser from `PB_ADMIN_EMAIL`/`PB_ADMIN_PASSWORD` and has no way to seed a
second or third from env vars alone. `post_deploy_commands` covers this: a
list of exec-form commands (like `command`, but run via `docker exec`
against the running container instead of becoming the container's `CMD`),
executed in order immediately after this deployment's own `Deploy()`
succeeds.

Each token is resolved for the same placeholders a `files` entry's content
supports (`{{slug}}`, `{{base_slug}}`, `{{base_domain}}`, `{{alias:...}}`,
`{{generated:...}}`), plus one more: `{{env:<key>}}` -- this deployment's
own resolved value for that env key (literal, generated, *or* prompted).
That last one is what makes optional prompted admins possible: a template
can declare `PB_ADMIN_EMAIL_2`/`PB_ADMIN_PASSWORD_2` as ordinary optional
(non-`required`) prompt env vars, then reference them in a command via
`{{env:PB_ADMIN_EMAIL_2}}`/`{{env:PB_ADMIN_PASSWORD_2}}` -- installing as
empty strings when the installer leaves them blank.

Because each command is exec-form (an argv array, not a shell string),
`docker exec` never re-interprets a resolved value -- an email or password
containing shell metacharacters can't break out of the command. If the
command itself needs a shell (e.g. to guard against an empty optional
value), invoke one explicitly and pass the resolved values as trailing argv
after `--` rather than interpolating them into the script text, so they
reach the shell as `$1`, `$2`, ... instead of being re-parsed:

```json
"post_deploy_commands": [
  [
    "sh", "-c",
    "if [ -n \"$1\" ] && [ -n \"$2\" ]; then /usr/local/bin/pocketbase superuser upsert \"$1\" \"$2\" --dir=/pb_data; fi",
    "--", "{{env:PB_ADMIN_EMAIL_2}}", "{{env:PB_ADMIN_PASSWORD_2}}"
  ]
]
```

See `pocketbase.json` for the real thing: it seeds up to two extra optional
superusers this way, on top of the one the image's own entrypoint creates
-- useful for standing up a single shared PocketBase instance with a few
trusted admins rather than a separate deployment per person.

A command is not retried, and a nonzero exit fails the whole install the
same way a failed `Deploy()` does -- write commands defensively (as above)
if a blank optional value should be a no-op rather than a failure. Like
`files`, these are **not** persisted anywhere; they only run once, during
`InstallTemplate` itself -- a later plain redeploy of the resulting
deployment does not re-run them.

### Ordering

Validation (`internal/templates/templates.go`'s `validate`, run once at
`init()` -- an invalid template file panics the binary on startup, not at
install time) enforces:

1. Exactly one deployment has `slug_suffix == ""` (the primary), and it's
   the **last** entry in the array.
2. Every `{{generated:key}}` placeholder references a `generate_key` set
   by an *earlier* deployment in the same array -- including references
   inside a `jwt:` generate kind, inside a `files` entry's content, and
   inside a `post_deploy_commands` token.
3. Every deployment has a positive `memory_limit_mb`, and a build source:
   `image_ref` for `build_strategy: "image"` (the default), `git_url` for
   `build_strategy: "dockerfile"`.
4. Every env var sets at most one of `value`, `generate`, `prompt`.
5. Every `files` entry has a non-empty `path` and `content`.
6. Every `post_deploy_commands` entry is non-empty, and every `{{env:key}}`
   placeholder inside one references an env key that actually exists on
   the *same* deployment.

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
