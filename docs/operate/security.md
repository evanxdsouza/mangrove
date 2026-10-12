# Security and isolation

What Mangrove protects, what it doesn't, and why.

## The trust model

**Mangrove runs code you trust, on a box dedicated to it.** It isn't a
sandbox for strangers' code.

- **Docker access is root access.** The `mangrove` user is in the `docker`
  group, so anyone who controls Mangrove, or any process running as
  `mangrove`, controls the box. That's unavoidable for a Docker-based
  platform, and why the box should do nothing else.
- **Apps run as ordinary containers.** Containers share the host kernel.
  There's no gVisor or Kata isolation. A container escape is a box
  compromise.
- **An owner account is full trust.** Owners can read NAS share passwords,
  run commands in any container, and download a backup containing every
  secret.

## What's isolated

**Mangrove itself.** The systemd unit runs it as an unprivileged user,
with `ProtectSystem=strict`, `ProtectHome=true`, `NoNewPrivileges=true`, a
private `/tmp`, and write access only to its two data directories.

**Mangrove's API.** It listens on `127.0.0.1` only. Expose it deliberately,
over HTTPS. See [Going to production](production.md#serve-the-dashboard-over-https).

**Apps from the network.** Containers' ports are never published on the
box. Only Caddy forwards to them, and only for public services.
Internal-only services, such as databases, have no route at all. The one
exception is a NAS share's SMB port.

**Apps from each other: not isolated.** Every app is on the same Docker
network and can reach every other app's internal port. Rely on each
service's own authentication (database passwords and so on), not the
network.

**Memory.** Each container has a memory and CPU limit, and in production
all app containers together sit in a slice with a hard memory cap, so one
runaway app can't starve SSH or Mangrove. See
[Scaling and resources](../deploy/scaling.md).

**Drive mounting.** Mounting needs root, so it's done by a separate small
helper, `mangrove-mountd`, which can only list, mount and unmount
removable drives and refuses to touch the system disk. Mangrove talks to it
over a Unix socket and never gains mount privileges itself. See
[storage.md](../storage.md).

## Secrets

- **Encrypted at rest** with AES-256-GCM under `master.key`: secret
  variables, GitHub tokens, webhook secrets. Each secret is bound to the
  record it belongs to, so a ciphertext can't be moved to another record.
- **Write-only** in the API and dashboard. A secret variable's value is
  never returned after it's saved.
- **Decrypted only to use them**: injecting variables into a container,
  cloning a repo, checking a webhook signature.
- `master.key` plus the database is everything. Protect backups
  accordingly. See [Backups and restore](../data/backups.md).

## Accounts and sessions

- Account passwords are hashed with argon2id (minimum 8 characters).
  Deployment gate passwords are hashed with bcrypt.
- Sessions last 30 days in an `HttpOnly`, `SameSite=Lax` cookie. Serve the
  dashboard over HTTPS; the cookie isn't marked `Secure` by Mangrove.
- Sign-in and setup are limited to 5 attempts per 5 minutes per IP.
- Owners can see and revoke every session.
- Workspace roles are enforced by the API on every request, not by the
  dashboard. See [Workspaces and roles](../teams/workspaces-and-roles.md).

## Inbound webhooks

The GitHub webhook endpoint requires no sign-in but checks every
delivery's HMAC signature with a constant-time comparison before doing
anything, and ignores replays of a delivery it has already seen.

## Protected deployments

The password gate runs in Mangrove, in front of the app. Gate cookies and
account handoff tokens are signed and short-lived, and bound to one
deployment. Paths in the "unprotected pages" list can't be abused with
`..` or `//` tricks. See [Protection](../deploy/protection.md).

## The MCP server

The remote (HTTP) MCP transport is guarded by a secret token in its URL,
not OAuth. Anyone with the URL can redeploy, roll back and run commands.
Treat it like a password, and rotate it by changing
`MANGROVE_MCP_URL_TOKEN`. The MCP server deliberately has no tools for
deleting things, managing users or reading secrets. See
[MCP for agents](../interfaces/mcp.md).

## Known gaps

- GitHub tokens are shared across the whole install, not scoped per
  workspace.
- NAS share passwords are stored unencrypted.
- There's no two-factor authentication, single sign-on, or API token with
  limited scope.
