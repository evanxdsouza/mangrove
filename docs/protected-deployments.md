# Protected deployments: the gate page

A password-protected deployment (the "Access control" card's password
toggle, `POST /api/deployments/{id}/access`) no longer sits behind Caddy's
native HTTP Basic Auth popup. Instead, a visitor sees a styled "This
deployment is protected" page offering two ways in: the deployment's own
password, or signing in with a Mangrove account. Modeled on the same idea
as Vercel/Cloudflare Access-style deployment protection pages.

## Why not just Caddy's `http_basic` auth

The old mechanism worked but couldn't offer a "continue with your account"
option or any custom styling -- it's a browser-native credentials prompt,
full stop. Getting a real page in front of visitors means Caddy can no
longer gate the route by itself; something has to decide, per request,
whether to render a page or let traffic through.

## The flow

```
visitor -> Caddy (public route) -> Mangrove's own loopback API -> real app
```

`internal/proxy/caddy.go`'s `gateHandler` replaces the normal
reverse_proxy/file_server handler on a password-protected route (port-based
*and* custom-domain routes -- see below) with an unconditional reverse
proxy to Mangrove's own `127.0.0.1:<APIPort>`, tagged with
`X-Mangrove-Gate-Deployment: <id>` (Caddy's `headers.set` always overwrites
any client-supplied value of the same name, and the API port is
loopback-only, so this header can be trusted). `internal/api/gate.go`'s
`gateIntercept` middleware, mounted ahead of all other routing, catches
every request carrying that header and handles it entirely on its own --
it never reaches `/api`, `/healthz`, or the dashboard SPA:

- No valid `mangrove_gate` cookie yet -> render the gate page.
- `POST /__mangrove_gate__/password` -> check the deployment's stored
  bcrypt hash (`Store.GetDeploymentPasswordHash`) directly; on success, set
  the cookie and redirect back.
- `GET /__mangrove_gate__/callback?token=...` -> the far side of the
  account handoff below.
- A valid cookie -> `Orchestrator.GateUpstreams` resolves where the real
  app actually is (a running container's address, or a static build's
  output directory) and Mangrove reverse-proxies the request through
  itself (`gateProxyThrough`).

### The account handoff

The protected deployment and the dashboard are different origins, so
there's no session cookie to just check -- this is a small OAuth-shaped
redirect dance, all three legs backed by signed, tamper-evident tokens
(`internal/gateauth`, sealed under the same master key as secrets at rest;
no new database table):

1. The gate page's "Continue with Mangrove account" button links to
   `<PublicURL>/gate-auth?token=<handoff token>` -- minted on the
   deployment's own domain, TTL 5 minutes. The return URL travels *inside*
   the token; `/gate-auth` never trusts a raw query parameter for it, since
   that endpoint is a public, unauthenticated GET anyone could link to.
2. `/gate-auth`, on the dashboard's own domain, checks the visitor's
   existing `mangrove_session` cookie (or shows a small standalone login
   form if there isn't one -- any role, owner or member, can complete this;
   see [multi-user.md](multi-user.md), there's no finer-grained per-project
   ACL to check). On success it mints a callback token (TTL 2 minutes) and
   redirects back to the deployment's own domain.
3. `GET /__mangrove_gate__/callback` redeems that token, sets the
   `mangrove_gate` cookie, and redirects to the page the visitor originally
   asked for.

If `MANGROVE_PUBLIC_URL` isn't set, there's no safe absolute origin to send
a visitor back to, so the account button is simply omitted -- the password
field still works.

### Custom domains

`internal/orchestrator/domains.go`'s `pushCustomDomainRoute` passes the
same `RouteOptions` (built from the deployment's own `PasswordProtected`
flag) that the port-based route gets, and `SetAccessControl` re-pushes a
deployment's verified custom domain route on every access-control change --
previously password protection only actually took effect on the port-based
route, never on a custom domain.

## Known limitations

- **No caching on the resolve-upstream path.** `GateUpstreams` does a live
  lookup (container address resolution, or a store read) on every request
  that passes the gate. Fine for what this is designed for -- a low-traffic
  staging gate, matching the existing "flip a staging build to
  password-protected without touching app code" framing -- not meant for a
  high-throughput production path.
- **Two protected deployments on the same bare hostname, different ports,
  no custom domain, will fight over the same cookie.** Cookie scoping
  ignores port, so a `mangrove_gate` cookie set for `host:31000` is also
  sent to `host:31001` -- since the cookie's payload is itself deployment-
  bound (via AAD), the wrong deployment just rejects it and re-shows its
  gate page, but only the most recently authenticated one stays "logged
  in" per hostname. A verified custom domain doesn't have this problem.
