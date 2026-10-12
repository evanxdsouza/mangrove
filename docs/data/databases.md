# Databases

Databases are ordinary deployments made from a template, with a volume for
their data. Mangrove has no separate managed-database layer.

## Create one

In a project, **Deploy from template** and pick one:

| Template | Image | Port | Credentials |
|---|---|---|---|
| PostgreSQL | `postgres:16-alpine` | 5432 | user `postgres`, database `postgres`, generated password |
| MySQL | `mysql:8` | 3306 | generated root password |
| MongoDB | `mongo:7` | 27017 | generated root user and password |
| Redis | `redis:7-alpine` | 6379 | generated password |

Choose a slug, such as `blog-db`. Generated passwords are shown **once**,
right after install. Copy them then. They're also stored as secret
variables on the database's service, which can be replaced but not read
back.

Apps that need a database, such as WordPress or Ghost, install their own
alongside themselves. You don't need to add one first.

## Connect from an app

Database templates are **internal only**: nothing outside the box can
reach them. Apps on the same box connect over Mangrove's Docker network
using the database service's alias, `mangrove-<slug>-app`:

```
DATABASE_URL=postgres://postgres:<password>@mangrove-blog-db-app:5432/postgres
```

Set that as a [secret variable](../run/environments.md#variables) on your
app, then **Redeploy** the app so it picks it up.

The database's **Overview** tab shows its **Internal address**.

## Reach it from your laptop

There's no public port for a database. Use an SSH tunnel to the container's
address on the box:

```sh
# On the box: find the container's IP on mangrove-net
docker inspect -f '{{(index .NetworkSettings.Networks "mangrove-net").IPAddress}}' \
  $(docker ps -qf name=mangrove-blog-db-app)

# From your laptop
ssh -L 5432:<that-ip>:5432 you@your-box
psql postgres://postgres:<password>@localhost:5432/postgres
```

The IP changes on every deploy of the database.

## Run commands and migrations

The **Run command** card on the database's Overview tab runs a one-off
command inside its container:

```sh
psql -U postgres -c 'CREATE DATABASE app;'
```

It suits short commands. Output is returned when the command finishes.
For an interactive shell, use `mangrove-tui` (see
[Logs and shell](../run/logs-and-shell.md)).

## Backing up a database

Mangrove's own backups cover its settings and secrets, **not** the data
in your databases. Dump them yourself, on a schedule:

```sh
docker exec $(docker ps -qf name=mangrove-blog-db-app) \
  pg_dump -U postgres postgres | gzip > blog-db-$(date +%F).sql.gz
```

Use `mysqldump`, `mongodump` or Redis's `BGSAVE` for the others. Copy the
result off the box.

## Good to know

- **Deleting the deployment deletes its data volume.** Back it up first.
- A database's memory limit counts toward the
  [memory budget](../deploy/scaling.md#the-memory-budget) like any other
  deployment.
- Upgrading a major version (Postgres 16 to 17, say) is the database's own
  dump-and-restore process. Mangrove doesn't automate it.
