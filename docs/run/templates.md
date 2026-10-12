# Templates

A template installs a ready-to-run app into a project: one or more
deployments, their volumes and variables, generated passwords, and the
wiring between them.

## Install one

In a project, **Deploy from template**:

1. Pick a template. Each card shows its category and roughly how much
   memory it needs.
2. Choose a **slug**. It names the deployments (`<slug>`, `<slug>-mysql`,
   and so on).
3. Fill in anything the template asks for, such as an API token. Required
   fields are marked `*`.
4. **Install.**

Mangrove creates the deployments in dependency order (databases first) and
deploys each one. When it finishes, it shows any **generated credentials**,
such as database and admin passwords. **They're shown once.** Copy them
before choosing **Done**.

The installed deployments are ordinary deployments. Change their
variables, protect them, add domains or delete them like any other.

## The library

| Template | What it is | Deployments | Memory |
|---|---|---|---|
| Ghost | Blog, using built-in SQLite | 1 | ~300 MB |
| WordPress | Website builder, with MySQL | 2 | ~640 MB |
| Gitea | Git hosting, like a private GitHub | 1 | ~512 MB |
| Supabase | Postgres, auth, auto-API, realtime and Studio | 7 | ~1280 MB |
| PocketBase | Single-file backend with database and auth | 1 | ~256 MB |
| NocoDB | Spreadsheet-style database, with Postgres | 2 | ~1216 MB |
| Vaultwarden | Bitwarden-compatible password manager | 1 | ~96 MB |
| Uptime Kuma | Uptime and status monitoring | 1 | ~224 MB |
| n8n | Workflow automation | 1 | ~768 MB |
| Umami | Privacy-friendly analytics, with Postgres | 2 | ~448 MB |
| Nephthys | Hack Club's Slack support bot, with Postgres | 2 | ~512 MB |
| PostgreSQL, MySQL, MongoDB, Redis | Standalone databases, see [Databases](../data/databases.md) | 1 | 80 to 320 MB |

Memory figures count against the
[memory budget](../deploy/scaling.md#the-memory-budget). On a 2 GB box the
default budget is 1536 MB, so Supabase or NocoDB leave little room for
anything else.

## After installing

- **Some apps start internal only**, such as Ghost and Uptime Kuma, so
  nobody can reach their setup screen before you. Finish setting them up
  over an SSH tunnel, or check **Public** on the **Access control** card when
  you're ready.
- **Check each template's description** in the gallery for first-login
  details. Umami, for example, starts with `admin` / `umami`; change it
  straight away.
- **Back up app data yourself.** Templates create volumes, and Mangrove's
  own backups don't include them. See
  [Storage](../deploy/storage.md#backing-up-a-volume).

## In Simple mode

[Simple mode](../start/simple-mode.md)'s **Add an app** shows the same
templates under plain names (Blog, Password manager, Website monitor...),
without the standalone databases, and creates a project for each app.

## Write your own

Templates are JSON files in `internal/templates/data/`, built into the
binary. Add one, rebuild, and it appears in the gallery. The full format,
including generated secrets, cross-deployment references, seed files and
post-deploy commands, is in [templates.md](../templates.md).
