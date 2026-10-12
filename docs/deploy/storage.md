# Storage

Containers are replaced on every deploy, so anything an app writes inside
its container is lost unless it's on a volume. Mangrove has two kinds of
storage: Docker volumes for app data, and an optional NAS feature for
sharing a plugged-in drive.

## Volumes for app data

A **named volume** is a directory Docker keeps outside any container. It
survives deploys, restarts and rollbacks, and is mounted into each new
container at the same path.

Where volumes come from:

- **Templates** declare the volumes their apps need, such as a database's
  data directory. They're named `mangrove-<slug>-<service>-<volume>`.
- **Compose stacks** declare their own under `volumes:` in the compose file.
- **Dockerfile, nixpacks and image deployments you create by hand** have no
  volumes. There's no way to add one from the dashboard yet. If an app
  needs to keep files, use a template, a compose file, or keep the data in
  a database deployment.

**Deleting a deployment deletes its volumes.** Stopping, restarting,
redeploying or rolling back never does.

### Backing up a volume

Mangrove's own backups don't include app data. To copy a volume's contents
out:

```sh
docker run --rm \
  -v mangrove-blog-db-app-data:/data:ro \
  -v "$PWD":/backup \
  alpine tar czf /backup/blog-db.tar.gz -C /data .
```

For databases, prefer the database's own dump tool. See
[Databases](../data/databases.md#backing-up-a-database).

## Share a drive as a NAS

Optional, owner-only, and off by default. The **Storage** page turns a
plugged-in USB drive into a network share that Windows, macOS and Linux can
open without extra software.

It needs a small privileged helper, `mangrove-mountd`, because mounting
drives needs root and Mangrove itself deliberately doesn't have it.
`setup.sh` asks whether to install it.

1. **Plug in the drive.** It appears on the Storage page. Mangrove never
   lists or touches the disk the system boots from.
2. **Mount.** The drive is mounted under
   `/var/lib/mangrove-drives/<uuid>`. ext4, xfs, btrfs, FAT, exFAT and NTFS
   are supported.
3. **Share as NAS.** Choose a share name, a username and a password.
   Mangrove runs a Samba container with the drive mounted inside, and
   publishes SMB (port 445) on the box's network interface.
4. **Connect** from another computer: `\\<your-box>\<share>` on Windows,
   `smb://<your-box>/<share>` on macOS and Linux.

The share is an ordinary deployment in a "Storage" project, so logs,
restart, stop and delete all work as usual.

Limits:

- One share per drive, and you can't unmount a drive while it's shared.
- No redeploy, rollback or scaling for a share. To change it, delete it and
  share again.
- Drives aren't mounted automatically when plugged in.
- The SMB password is stored in the share's settings unencrypted, and is
  visible to owners.
- Not yet tested against real hardware. Try it with your drive and client
  before relying on it.

How the helper is isolated, and how to install it by hand:
[storage.md](../storage.md).
