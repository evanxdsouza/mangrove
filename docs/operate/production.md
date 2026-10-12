# Going to production

A checklist for a box people rely on. `setup.sh` does most of this; the
details are in [deployment.md](../deployment.md).

## The box

- [ ] **Dedicated.** Mangrove's user is in the `docker` group, which is
      root-equivalent. Don't run unrelated services on the box.
- [ ] **Memory slices installed.** `mangrove.slice` protects Mangrove and
      Caddy (384 MB guaranteed, 512 MB soft cap).
      `mangrove-deployments.slice` hard-caps all app containers together
      (1536 MB, no swap). Size these for your RAM.
- [ ] **The budget matches the slice.**
      `MANGROVE_DEPLOYMENT_MEMORY_CEILING_MB` should equal the deployments
      slice's `MemoryMax`, and `MANGROVE_CGROUP_PARENT=mangrove-deployments.slice`
      must be set so containers actually join it. `setup.sh` sets both.
- [ ] **Secrets in `/etc/mangrove/mangrove.env`**, mode `600`, never in the
      unit file or git.
- [ ] **The firewall** allows SSH, 80 and 443, and Mangrove's port range
      (20000 to 21000) if apps are reached by port. Port 7777 stays closed;
      Mangrove only listens on loopback anyway.

## Serve the dashboard over HTTPS

Mangrove listens on `127.0.0.1:7777`. Choose one way in:

**Behind your host's HTTPS edge** (Hack Club Nest, a Cloudflare Tunnel, a
load balancer). Have the edge forward your dashboard's domain to a port on
the box, and add a matching block to `/etc/caddy/Caddyfile`:

```caddyfile
{
	admin 127.0.0.1:2019
}
:80 {
	reverse_proxy 127.0.0.1:7777
}
```

Because the edge owns HTTPS, set `MANGROVE_CUSTOM_DOMAIN_MODE=port` for
custom domains (see [Domains and HTTPS](../deploy/domains.md#port-mode-hack-club-nest-or-anywhere-80443-isnt-yours)).

**Caddy terminates HTTPS on the box itself.** A site block such as
`mangrove.example.com { reverse_proxy 127.0.0.1:7777 }` gets a certificate
automatically. But it takes ports 80 and 443, which Mangrove's
automatic-HTTPS custom domains also need, and Caddy won't let two servers
share them. Use port-mode custom domains with this setup too.

**SSH tunnel only.** Fine for a single operator. GitHub webhooks and the
"Continue with Mangrove account" option on protected deployments both
need a public dashboard URL, so they won't work.

Whichever you choose, set `MANGROVE_PUBLIC_URL` to the dashboard's public
address (for example `https://mangrove.example.com`) so GitHub callbacks and
protected-deployment sign-in send people to the right place.

## Caddy: never reload casually

Mangrove pushes each app's route to Caddy through its admin API when the
app deploys. Those routes live only in Caddy's memory. **Mangrove doesn't
re-push them when it starts.**

So anything that makes Caddy reload its Caddyfile (`caddy reload`,
`systemctl reload caddy`, `systemctl restart caddy`, a reboot) leaves every
app without a route until that deployment is restarted or redeployed.

- Edit the Caddyfile before you deploy apps, or plan to restart every
  deployment afterwards.
- **After a reboot, press Restart on each deployment** (or run
  `mangrovectl deploy` for each) to bring routes back.

## Before you rely on it

- [ ] **Backups scheduled and tested.** See
      [Backups and restore](../data/backups.md), and back up app data
      separately.
- [ ] **Two owner accounts**, so one forgotten password isn't a lockout.
- [ ] **Health check paths** on every service, so bad deploys are caught.
- [ ] **Self-healing** on for anything nobody is watching.
- [ ] **Email notifications** configured, if you want them.
- [ ] **An upgrade plan.** See [Upgrades](upgrades.md).

## Updating secrets

Edit `/etc/mangrove/mangrove.env`, then `systemctl restart mangrove`.
Restarting Mangrove doesn't touch running apps or Caddy's routes.
