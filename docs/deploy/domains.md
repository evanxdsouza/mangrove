# Domains and HTTPS

Every public deployment is reachable on its own port straight away. Add a
custom domain when you want a real name with HTTPS.

## The port you get for free

Each public service gets a port from Mangrove's range (`20000` to `21000`
by default, set with `MANGROVE_PORT_RANGE_MIN` and `_MAX`). Caddy listens on
that port and forwards to the container, so the app is at
`http://<your-box>:<port>`. The service card on the deployment's
**Overview** tab shows the port.

The port stays the same across deploys and rollbacks. It's released when
the deployment is deleted. Owners can see every allocation, and reserve
ports, under **Admin → Port registry**.

## Add a custom domain

Owners and workspace admins can add domains from the **Domains** card on a
deployment's Overview tab. How it works depends on how your box is
reachable, set once per install with `MANGROVE_CUSTOM_DOMAIN_MODE`.

### Automatic HTTPS (the default)

For a box that the internet can reach on ports 80 and 443.

1. Point the domain's DNS (an `A` or `CNAME` record) at your box.
2. **Domains → Add**, and enter the hostname.
3. Mangrove shows a TXT record value,
   `mangrove-domain-verification=<token>`. Add it as a TXT record on that
   hostname.
4. **Verify.** Once Mangrove finds the record, the route goes live and
   Caddy gets a certificate from Let's Encrypt and keeps it renewed.

The TXT check proves you control the domain. Without it, anyone could
point a stale domain at your box and take traffic meant for someone else.

Ports 80 and 443 must be free for Caddy. If you've already set Caddy up to
serve something else on them, custom domains won't work until that's
resolved.

### Port mode (Hack Club Nest, or anywhere 80/443 isn't yours)

Some hosts only forward a domain to a port you register in their own
dashboard, and terminate HTTPS themselves. Set:

```sh
MANGROVE_CUSTOM_DOMAIN_MODE=port
```

Now adding a domain skips verification and gives it a dedicated port from
the same range. Register `<hostname> → <that port>` in your host's
dashboard. It's live immediately. Removing the domain frees the port.

Switching modes only affects domains added afterwards.

## Home server DDNS

A home server usually has no static IP. Mangrove can keep a
[DuckDNS](https://www.duckdns.org) name pointed at your current public IP,
checking every 5 minutes:

```sh
MANGROVE_DDNS_DOMAIN=myhome           # myhome.duckdns.org
MANGROVE_DDNS_TOKEN=<your duckdns token>
```

`setup.sh` sets this up when you choose "home". You still need to forward
ports 80 and 443 on your router to the box, both for traffic and for the
Let's Encrypt challenge. Then point your custom domains at the DuckDNS
name with a `CNAME`.

## Protecting a domain

Password protection and public paths apply to custom domains the same way
as to the port. See [Protection](protection.md).

## If a domain doesn't work

- **Verify keeps failing.** TXT records can take a while to propagate.
  Check with `dig TXT your.domain`. The record goes on the exact hostname
  you added, not the apex.
- **Verified, but no certificate.** Caddy couldn't complete the HTTP
  challenge. Check that ports 80 and 443 reach the box and nothing else is
  bound to them.
- **Port mode, but nothing loads.** Check the port registered in your
  host's dashboard matches the one Mangrove shows.

How routing works underneath: [deployment.md](../deployment.md#custom-domains).
