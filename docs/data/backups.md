# Backups and restore

Mangrove's whole state lives in two files in its data directory
(`MANGROVE_DATA_DIR`; `/var/lib/mangrove/data` in a `setup.sh` install):

- `mangrove.db`: projects, deployments, settings, users, and every secret,
  encrypted.
- `master.key`: the key those secrets are encrypted with.

**Back them up together.** Without `master.key`, every stored secret
(variables, GitHub tokens, webhook secrets) is unrecoverable, even with
the database intact.

## Take a backup

Owners only:

```sh
mangrovectl backup                    # saves mangrove-backup-<timestamp>.tar.gz here
mangrovectl backup --out /backups/mangrove.tar.gz
```

The CLI can run anywhere that can reach the API (set `MANGROVE_API_URL`),
so the backup lands wherever you run it, not on the box it protects. It
won't overwrite an existing file.

The archive holds:

- `mangrove.db`: a consistent snapshot, taken safely while Mangrove is
  running.
- `master.key`.
- `BACKUP_INFO.txt`: when it was taken, and how to restore it.

**Treat the file like the box's root password.** Together, the two files
unlock every secret Mangrove holds. Store it encrypted and access-controlled.

## Schedule it

There's no built-in schedule or cloud upload. Use cron on another machine:

```cron
0 3 * * * MANGROVE_API_URL=https://mangrove.example.com mangrovectl backup --out /backups/mangrove-$(date +\%F).tar.gz
```

`mangrovectl` reuses the session saved by `mangrovectl login`. Sessions last
30 days, so log in again before it expires, or run cron as a user who
does. See [Sign-in and sessions](../account/sign-in.md).

## What isn't in it

- **Your apps' data.** Database contents and files in volumes need their
  own backups. See [Databases](databases.md#backing-up-a-database) and
  [Storage](../deploy/storage.md#backing-up-a-volume).
- **Images and running containers.** You redeploy after a restore.
- **Caddy's routes.** Redeploying pushes them again.

## Restore on a new box

1. Install Mangrove (`setup.sh` or by hand), but **don't let it start for
   the first time**. If it already did, `systemctl stop mangrove`.
2. Put the two files into the data directory:

   ```sh
   install -d -o mangrove -g mangrove -m 700 /var/lib/mangrove/data
   rm -f /var/lib/mangrove/data/mangrove.db-wal /var/lib/mangrove/data/mangrove.db-shm
   tar xzf mangrove-backup-<timestamp>.tar.gz -C /var/lib/mangrove/data mangrove.db master.key
   chown mangrove:mangrove /var/lib/mangrove/data/mangrove.db /var/lib/mangrove/data/master.key
   chmod 600 /var/lib/mangrove/data/mangrove.db /var/lib/mangrove/data/master.key
   ```

3. `systemctl start mangrove`, and sign in with any account from the backup.
4. **Redeploy every deployment.** That rebuilds images, starts containers
   and restores routes.
5. Restore your apps' data from their own backups.

More detail: [backup.md](../backup.md).
