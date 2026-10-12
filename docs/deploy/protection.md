# Protection

Put a password in front of a deployment without touching its code: useful
for staging sites, previews and internal tools.

## Turn it on

On the deployment's **Access control** card (owners and workspace admins):

1. Keep **Public** checked.
2. Check **Password-protected** and set a password.
3. Save.

Visitors now see a "This deployment is protected" page with two ways in:

- **The password** you set.
- **Continue with Mangrove account**: anyone with a Mangrove account on
  this box can sign in instead. This needs `MANGROVE_PUBLIC_URL` set, so
  Mangrove knows where to send them back. Without it, only the password
  option appears.

Once in, a cookie keeps the visitor signed in to that deployment.

Protection covers the deployment's port and all its custom domains. It's
enforced before requests reach your app, whatever auth the app has itself.

## Leave some pages open

**Unprotected pages** takes one path per line. Those skip the gate;
everything else stays protected.

| Pattern | Matches |
|---|---|
| `/pricing` | Exactly `/pricing` (not `/pricing/`) |
| `/docs/*` | Everything under `/docs/` |
| `/assets*` | Anything starting with `/assets` |

An open page's own CSS, scripts and images need their paths listed too
(for example `/assets/*`), or they'll hit the gate. Query strings are
ignored, and tricks like `/docs/../admin` never match.

To change the list without changing the password, leave the password field
blank and save.

## Limits

- **Single-service deployments only.** Compose stacks can't be protected
  this way.
- **Meant for low traffic.** Every protected request passes through
  Mangrove itself, so it's right for a staging gate, not a busy production
  site.
- **Two protected deployments on the same host name, on different ports,
  share a cookie name.** Signing into one signs you out of the other. A
  custom domain for each avoids this.

How the gate and the account handoff work: [protected-deployments.md](../protected-deployments.md).
