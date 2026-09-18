# Backup and restore

Mangrove's entire control-plane state -- projects, deployments and their
config, env vars (including decrypted-on-demand secrets), GitHub PATs,
custom domain registrations, user accounts -- lives in one SQLite file
(`mangrove.db`). Every secret value in it is encrypted at rest under one
master key (`master.key`, see `internal/secrets`' package doc): **losing
that key file makes every secret permanently unrecoverable, even with the
database completely intact.** The two have to be backed up together, and
until this feature existed there was no code path for it at all -- the
single highest-consequence gap in the project relative to a direct
competitor like Coolify, which treats scheduled backups as core.

This is the minimum viable version: an on-demand tar of the database and
key, and a documented restore path. No scheduling, no S3/off-box upload --
run it yourself, on a schedule of your choosing, and copy the result
somewhere durable.

## Taking a backup

```sh
mangrovectl backup [--out PATH]
```

Owner-only (`GET /api/admin/backup`, gated the same way `/admin/users` is
-- see [multi-user.md](multi-user.md)). `mangrovectl` can run against a
remote box via `MANGROVE_API_URL` just like every other command, so the
backup doesn't have to be taken *on* the box it's protecting -- copy it
straight to wherever you're running the CLI from.

Without `--out`, the file is saved in the current directory under the name
the server suggests (`mangrove-backup-<UTC timestamp>.tar.gz`); the command
refuses to overwrite an existing file at the target path.

The archive contains exactly three things:

- `mangrove.db` -- a **consistent snapshot** of the live database, taken
  via SQLite's `VACUUM INTO` (see `internal/api/backup.go`'s
  `snapshotDB`), not a raw copy of the live file. Mangrove's DB runs in WAL
  mode with a live writer, so a plain file copy could catch a torn
  checkpoint; `VACUUM INTO` is SQLite's own supported mechanism for taking
  a point-in-time-consistent copy of a live, concurrently-written database.
- `master.key` -- the raw 32-byte AES-256 key everything above is
  encrypted under.
- `BACKUP_INFO.txt` -- a plain-text timestamp and a one-paragraph reminder
  of what the other two files are and how to restore them, so a backup
  found months later on some other disk is still self-explanatory without
  this doc.

**Treat the resulting file as maximally sensitive** -- paired together,
`mangrove.db` and `master.key` are equivalent to every secret on the box
in near-cleartext form (GitHub PATs, webhook secrets, every secret env
var). Store it encrypted at rest and access-controlled, the same way
you'd treat the box's own root credentials, not in a public bucket or a
shared drive.

## What this does *not* cover

- **Docker images and app data volumes.** This backs up Mangrove's own
  record of what should be running (projects, deployment config, env
  vars), not the containers/images/volumes themselves. Restoring gets you
  the control plane back; you still redeploy each service afterward (see
  below) to actually bring it up, and any app-level persistent data (a
  database volume, uploaded files) needs its own backup strategy --
  outside Mangrove's scope today, same as before this feature.
- **Caddy's running route config.** Routes are pushed to Caddy's admin API
  at deploy time, not reconciled from the DB on every Mangrove startup.
  Restoring the DB alone doesn't repopulate Caddy; redeploying each
  deployment does (it pushes routes as a normal part of the deploy flow).
- **Scheduling and off-box upload.** Run `mangrovectl backup` from cron (or
  your own scheduler) and move the result off the box yourself -- e.g.
  `scp` it out, or point cron at a mounted network share. There's no
  built-in S3/cloud-storage integration yet.

## Restoring on a fresh box

1. Provision the new box and install Mangrove up through `setup.sh`
   (or your own equivalent), but **do not let `mangrove.service` start
   for the first time** before step 3 -- if it starts first, it'll run
   the first-run setup wizard and generate its own fresh `mangrove.db` and
   `master.key`, and you'd be restoring on top of real (if empty) files
   rather than into a clean data dir. `systemctl stop mangrove` first if
   it already auto-started.
2. Extract the backup's two files into the box's data directory --
   `/var/lib/mangrove` in a `setup.sh` install (`MANGROVE_DATA_DIR`
   otherwise):
   ```sh
   tar xzf mangrove-backup-<timestamp>.tar.gz -C /var/lib/mangrove mangrove.db master.key
   chown mangrove:mangrove /var/lib/mangrove/mangrove.db /var/lib/mangrove/master.key
   chmod 600 /var/lib/mangrove/mangrove.db /var/lib/mangrove/master.key
   ```
   (matching `setup.sh`'s own system user/perms -- see `deploy/systemd/mangrove.service`'s
   `User=mangrove`/`Group=mangrove` and the 0700 data dir it creates.)
3. Start `mangrove.service`. It opens the restored database directly (no
   setup wizard -- accounts already exist) and decrypts secrets correctly,
   since the key file matches. Log in with any account that existed at
   backup time; existing session rows in the restored DB are harmless
   (they'll simply fail `ValidateSession`'s expiry check like any other
   stale session, per [multi-user.md](multi-user.md)).
4. **Redeploy every deployment** (dashboard's Redeploy button, or
   `mangrovectl deploy --deployment ID`) to actually rebuild and start
   containers and repopulate Caddy's routes -- per "What this does *not*
   cover" above, the backup restores Mangrove's record of your
   infrastructure, not the running infrastructure itself.

Verified end-to-end on this box: took a live backup, extracted it into an
empty scratch data dir, started a second instance against it on a
different port, and logged in with the original owner credentials against
the restored database.
