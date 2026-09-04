# Storage: turning a plugged-in drive into a NAS

Entirely optional, and off by default. The Storage page lets an owner
mount a removable drive and share it over SMB to other devices on the
LAN -- a plain USB-drive-to-NAS flow, built on the same deployment
machinery as everything else Mangrove runs, plus one new, narrowly-scoped
privileged component.

## Why a second privileged process

`mangrove.service` runs deliberately sandboxed (see
[deployment.md](deployment.md)): `ProtectSystem=strict`, `ProtectHome=true`,
writes confined to `/var/lib/mangrove{,-static}`. Mounting an arbitrary
block device needs root or `CAP_SYS_ADMIN` -- capabilities that sandbox
exists specifically to deny the main process, and giving them to it
permanently to support one optional feature would be a real, permanent
expansion of what a compromised control-plane process could do to the
host.

Instead, `cmd/mangrove-mountd` is a second, separate binary/systemd unit
that owns exactly three operations -- list removable drives, mount one,
unmount one -- and nothing else. It talks to the main process over a local
Unix domain socket only (`/run/mangrove-mountd.sock`, `0660`,
`root:mangrove-mount`); `mangrove.service` reaches it by being in the
`mangrove-mount` group (see `deploy/systemd/mangrove-storage-group.conf`),
the same privilege-separation shape this project already uses for Docker
(`mangrove.service`'s own comment: "the mangrove user must be in the
`docker` group -- effectively root-equivalent access; there is no way
around this for a Docker-based executor"). A compromised main process can
ask the helper to mount/unmount a drive; it can't do anything else root
could do, because the helper's protocol (`internal/mountd`) doesn't expose
anything else.

**Safety boundary inside the helper itself**: `internal/mountd/server.go`'s
`listDrives` resolves the disk backing `/` (via `findmnt` + lsblk's
`PKNAME` parent-chain) and excludes it and every one of its partitions from
every list/mount/unmount call, unconditionally -- there's no way to ask the
helper to touch the system disk, even from a fully compromised main
process feeding it arbitrary UUIDs. See
`TestFilterDrives_NeverOffersSystemDisk` in `internal/mountd/server_test.go`.

## Installing it

Skipped by default -- `setup.sh` only asks about it, and only installs
`mangrove-mountd` if you say yes. To add it to an existing install
afterward, either re-run `setup.sh` (idempotent, same as everything else it
does) or by hand:

```sh
go build -o /usr/local/bin/mangrove-mountd ./cmd/mangrove-mountd
apt-get install -y exfatprogs ntfs-3g   # exFAT/NTFS drive support
groupadd -f mangrove-mount
usermod -aG mangrove-mount mangrove
mkdir -p /var/lib/mangrove-drives && chown root:root /var/lib/mangrove-drives && chmod 755 /var/lib/mangrove-drives
cp deploy/systemd/mangrove-mountd.service /etc/systemd/system/
mkdir -p /etc/systemd/system/mangrove.service.d
cp deploy/systemd/mangrove-storage-group.conf /etc/systemd/system/mangrove.service.d/storage-group.conf
systemctl daemon-reload
systemctl enable --now mangrove-mountd
systemctl restart mangrove   # picks up the new supplementary group
```

Without this, the Storage page still renders -- it just shows "the storage
helper isn't reachable" (`internal/mountd.ErrUnavailable`, surfaced as
`GET /api/storage/drives`'s `helper_available: false`) instead of an error.

## What "share as NAS" actually does

1. **Mount**: the Storage page lists every drive `mangrove-mountd` finds
   (`lsblk`-backed, `internal/mountd/server.go`), and "Mount" asks the
   helper to mount it under `/var/lib/mangrove-drives/<filesystem-uuid>`
   with `nosuid,nodev`. ext4/xfs/btrfs/vfat/exfat mount natively; ntfs
   tries the in-kernel `ntfs3` driver first (Linux 5.15+), falling back to
   the `ntfs-3g` FUSE driver.
2. **Share**: "Share as NAS" (`POST /api/storage/shares`,
   `Orchestrator.CreateNASShare` in `internal/orchestrator/storage.go`)
   creates a normal `deployments`/`services` row -- in a shared "Storage"
   project, auto-created on first use -- running
   [`dperson/samba`](https://github.com/dperson/samba) with the mounted
   drive bind-mounted in at `/share` (`executor.RunSpec.HostMount`, new for
   this feature -- see its doc comment for why this is the *only* thing
   allowed to construct one) and port 445 published straight onto the
   host's real network interface, not just `127.0.0.1`
   (`executor.RunSpec.PublicBind`). That's the one deliberate exception to
   every other deployment's "only Caddy can reach the container directly"
   rule: SMB isn't HTTP, so Caddy (which only reverse-proxies/file-serves
   HTTP -- see [templates.md](templates.md)'s `force_internal_only`) has no
   way to route it at all. `IsInternalOnly` is set on the service so the
   dashboard doesn't offer a Caddy-based access-control toggle that
   wouldn't do anything.
3. From there it's a real, first-class deployment: logs, resource stats,
   `docker exec`, an interactive terminal, stop/restart, and delete
   (`DELETE /api/deployments/{id}`, no changes needed -- see
   `Orchestrator.DeleteDeployment`) all work exactly like any other
   deployment, because it *is* one.

## What it deliberately doesn't do

- **No redeploy/rollback/scale.** A NAS share holds an exclusive host port
  (445) for its entire lifetime. The normal blue/green swap
  (`Orchestrator.Deploy`) briefly runs the old and new containers side by
  side during its health-check gate -- impossible with an exclusive port,
  which is exactly why `RunSpec.HostPort` was otherwise never wired up to
  begin with (see `deploy.go`'s comment on that field). `Deploy()` refuses
  outright for a service with `DirectPublishPort` set; to change a share's
  config, delete it and create a new one.
- **One share per drive.** `CreateNASShare` refuses if the drive's mount
  path is already bind-mounted into another share
  (`Orchestrator.driveInUseBy`).
- **Unmount refuses while shared**, for the same reason: pulling the
  filesystem out from under a running container's bind mount can corrupt
  writes in flight.
- **No auto-mount on plug-in.** The helper only mounts a drive when asked
  (`POST /api/storage/drives/{uuid}/mount`); nothing watches for a new
  device appearing and mounts it automatically. Plugging in a drive gets it
  *listed*; sharing it is still two explicit clicks.
- **No NFS, no fine-grained ACLs, one flat share per drive.** SMB was
  chosen because it's what every mainstream OS (Windows/macOS/Linux) can
  mount natively with no extra client software -- see the design
  discussion that led here for the alternatives considered (WebDAV, a bare
  web file browser).
- **Credentials are plaintext SMB creds, shown once.** They're passed to
  the container as exec-form `docker run`/`docker exec` args (never
  through a shell -- see `sambaCommand`'s doc comment), and stored in
  `services.command` the same way any other deployment's command override
  is (not specially encrypted the way a secret env var is). Anyone with
  dashboard owner access who can read that deployment's config can recover
  them; the underlying assumption is the same one the rest of Mangrove
  makes about its own admin surface -- an owner account is already
  full trust.

## API surface

`/api/storage/*` is owner-only end to end (`auth.RequireOwner` on the whole
route group in `internal/api/router.go`), same bar as `/api/admin/*`:
mounting host drives and creating shares with plaintext credentials is
system-level, not something a member's deploy/rollback/logs access implies.

| Route | What |
|---|---|
| `GET /api/storage/drives` | List drives (`helper_available: false` instead of an error if `mangrove-mountd` isn't reachable) |
| `POST /api/storage/drives/{uuid}/mount` | Mount |
| `POST /api/storage/drives/{uuid}/unmount` | Unmount (refused while shared) |
| `GET /api/storage/shares` | List active NAS shares, enriched with their deployment name/slug |
| `POST /api/storage/shares` | Create a share: `{drive_uuid, slug, share_name, username, password}` |

Not exposed via `mangrove-mcp` or `mangrove-tui` -- see
[clients.md](clients.md)'s "deliberately not exposed" list for the same
reasoning: this is exactly the kind of action (mounting host filesystems,
minting SMB credentials) where a model or a scripted client acting on a
misread is hardest to undo.

## Not verified

This was built and unit-tested (`internal/mountd`'s device-filtering logic
against fixture `lsblk` output, `internal/executor`'s host-mount/public-bind
behavior against real Docker, `internal/orchestrator`'s NAS-share lifecycle
against a fake mountd client) but **not exercised against a real plugged-in
drive or a real SMB client** in this pass -- that needs physical hardware
this environment doesn't have. Before relying on this in production: plug
in a real drive, mount it, share it, and connect from an actual Windows/
macOS/Linux SMB client to confirm read/write actually works end to end.
