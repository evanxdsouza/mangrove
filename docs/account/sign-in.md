# Sign-in and sessions

Mangrove has email-and-password accounts, created by an owner. There's no
public sign-up, no single sign-on, and no password-reset email.

## The first account

On a fresh install, the dashboard's first screen creates the first account,
which is always an **owner**. `setup.sh` can create it for you instead. This
screen only appears while there are no accounts at all.

## Signing in

Sign in with your email and password. Your session lasts **30 days**, in an
`HttpOnly` cookie (`mangrove_session`). **Log out** at the bottom of the
sidebar ends it.

Sign-in and first-run setup are rate-limited to **5 attempts per 5 minutes
per IP address**. After that, sign-in fails until the window passes.

Serve the dashboard over HTTPS. The cookie isn't marked `Secure`, so over
plain HTTP it can be read in transit. See
[Going to production](../operate/production.md#serve-the-dashboard-over-https).

## Other clients

`mangrovectl`, `mangrove-tui` and `mangrove-mcp` share one session file,
`~/.mangrove/session`. Sign in once with any of them:

```sh
mangrovectl login --email you@example.com --password '...'
```

There are no API tokens. Scripts and CI sign in like this and reuse the
session until it expires. See [HTTP API](../interfaces/api.md#authentication).

## Managing sessions (owners)

**Admin → Active sessions** lists every signed-in session on the box. Revoke
one to sign it out immediately: a lost laptop, an old CI login, someone
who's left.

## Forgotten passwords

- **A member** forgot theirs: an owner deletes the account under
  **Admin → Team** and creates it again with a temporary password. The
  person changes it in [Settings](profile.md#change-password). Their
  workspace memberships need adding again.
- **The only owner** forgot theirs: there's no recovery flow. Restore from a
  backup taken while you knew the password, or ask another owner. Keep at
  least two owner accounts, or a password manager entry, for this reason.

## Signing in to protected deployments

A [password-protected deployment](../deploy/protection.md) can also let
people in with their Mangrove account. They're sent to the dashboard to
sign in, then straight back.
