# Troubleshooting

Start with the symptom.

## Deploys

**The build failed.**
Open the failed run in the **History** tab for the error. Common causes:

- *nixpacks: "No start command could be found"*: the repo is a static
  frontend. Use the **Static site** strategy with a build command and output
  directory. See [Builds](../deploy/builds.md).
- *Exit code 137 during a build*: the build ran out of memory. Builds run
  outside the app memory budget but still share the box's RAM. Stop other
  deployments while building, or build an image elsewhere and deploy it
  with **Existing image**.
- *A value the build needs is missing*: variables aren't passed to builds.
  See [Build-time variables](../deploy/builds.md#build-time-variables).
- *Cloning failed*: the token can't see the repository, or the branch
  doesn't exist.

**"new containers failed health check within 60s".**
The app didn't answer its health check in time. Check that it listens on
`0.0.0.0` (not `127.0.0.1`) and on the configured internal port, that the
health path returns 2xx or 3xx, and that it starts within a minute. The
**Logs** tab shows what the new container printed. The old version is
still running.

**"deploy would allocate …MB on top of …MB already committed, exceeding the …MB ceiling".**
The deploy would push the total memory limits over the budget. Stop or
delete something, or lower limits. See
[Scaling and resources](../deploy/scaling.md#the-memory-budget).

**"a deploy is already in progress for this deployment".**
Only one deploy per deployment runs at a time. Wait, or **Cancel deploy**.

**"deployment has no linked repository to redeploy from".**
It was deployed from a non-GitHub URL. Deploy it again with the URL. See
[From GitLab, Gitea and others](../deploy/from-other-git.md).

**A new deployment shows "Cancel deploy" but nothing is running.**
A deployment that has never been deployed is *pending*. Press **Redeploy**
to start its first deploy.

## Pushes don't deploy

See [From GitHub → If a push doesn't deploy](../deploy/from-github.md#if-a-push-doesnt-deploy).
In short: check the delivery log on the project's **GitHub** tab, check the
**Auto-deploy** card has the right branch, and check GitHub can reach your
dashboard's public URL.

## Apps unreachable

**Every app stopped responding at once, but their containers are running.**
Caddy lost its routes: it was reloaded or restarted, or the box rebooted.
Mangrove doesn't re-push routes on its own. Press **Restart** on each
deployment (or redeploy it). See
[Going to production](production.md#caddy-never-reload-casually).

**One app is unreachable.**
- Is it **Public**? Check the **Access control** card.
- Is its status **running**? A **stopped** deployment has no route.
- Is the port open in the box's firewall?
- Is it **sleeping**? The first request shows a waking page.

**A custom domain doesn't work.**
See [Domains and HTTPS → If a domain doesn't work](../deploy/domains.md#if-a-domain-doesnt-work).

**An app can't reach its database.**
Use the database's internal alias (`mangrove-<slug>-app`), not
`localhost`, and check the password variable. Variables only apply after a
**Redeploy**.

## The dashboard

**Sign-in fails with "Something went wrong" after a few tries.**
You've hit the limit of 5 attempts per 5 minutes per IP. Wait five minutes.

**The dashboard is blank or out of date after an upgrade.**
The binary was built before the dashboard. Run `cd web && npm run build`,
rebuild the binary, and swap it again. See [Upgrades](upgrades.md). If it's
only stale, reload the page.

**A page shows "Something went wrong loading this page."**
That page hit an error. The rest of the dashboard still works. Reload, and
check `journalctl -u mangrove` for the API error.

## The box

**Disk filling up.**
Run `docker system df`, then **Admin → Image / build cache pruning**.
Check app volumes with `docker system df -v`.

**Memory pressure, or the sidebar mark is red.**
Check **Server health** and **Admin → Resource budget**. Stop deployments
you don't need, or turn on [idle sleep](../run/idle-sleep.md) for rarely
used ones.

**Mangrove won't start.**
`journalctl -u mangrove -n 50`. Common causes: a typo in
`/etc/mangrove/mangrove.env`, Docker not running, a missing `master.key`
next to an existing database (restore it from a backup; secrets can't be
recovered without it), or another process on port 7777.

## Still stuck

- Mangrove's logs: `journalctl -u mangrove -f`
- An app's container: `docker logs <container>`
- Caddy's view: `curl -s 127.0.0.1:2019/config/ | jq`

The design docs listed under **Internals** in the [docs index](../README.md)
explain how each part works.
