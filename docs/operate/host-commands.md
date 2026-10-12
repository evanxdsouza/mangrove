# Host commands

Commands for the box Mangrove runs on. Everything here needs `sudo` or root.

## The service

```sh
systemctl status mangrove         # running?
systemctl restart mangrove        # after changing /etc/mangrove/mangrove.env
journalctl -u mangrove -f         # follow Mangrove's own logs
journalctl -u mangrove --since "1 hour ago"
curl -s http://127.0.0.1:7777/healthz   # 200 if the API is up
```

Restarting Mangrove doesn't affect running apps.

## Files

| Path | What |
|---|---|
| `/usr/local/bin/mangrove` | The binary |
| `/etc/mangrove/mangrove.env` | Configuration and secrets |
| `/var/lib/mangrove/data/mangrove.db` | The database |
| `/var/lib/mangrove/data/master.key` | The encryption key. Back it up with the database. |
| `/var/lib/mangrove-static/` | Static sites' built files |
| `/var/lib/mangrove-drives/` | Mounted drives (NAS feature) |
| `/etc/systemd/system/mangrove*.{service,slice}` | Units and memory slices |
| `~/.mangrove/session` | A CLI user's saved session |

## Apps

Every app container is named `mangrove-<deployment-slug>-<service>-<run>`:

```sh
docker ps --filter name=mangrove-                       # all app containers
docker logs -f $(docker ps -qf name=mangrove-blog-app)  # one app's logs
docker stats --no-stream                                 # live CPU and memory
docker network inspect mangrove-net                      # who's on the network
```

Prefer the dashboard, CLI or TUI for anything that changes state. Mangrove
treats its database as the truth, so a container removed by hand isn't
noticed until that deployment is next deployed or restarted.

## Caddy

```sh
curl -s 127.0.0.1:2019/config/apps/http/servers | jq 'keys'   # what's routed
systemctl status caddy
```

Each app's port appears as a server named `srv_<port>`; verified custom
domains share `srv_public`.

**Don't reload or restart Caddy** unless you're ready to restart every
deployment afterwards. Reloading drops every route Mangrove pushed. See
[Going to production](production.md#caddy-never-reload-casually).

## Disk

Images and build cache are pruned every 6 hours. To free space now:

```sh
docker system df                  # what's using space
```

Then **Admin → Image / build cache pruning** in the dashboard, which never
removes an image a deployment is using or could roll back to.

## Memory slices

```sh
systemctl status mangrove.slice mangrove-deployments.slice
systemd-cgtop                     # live usage per slice
```

## Backup from the box

```sh
sqlite3 /var/lib/mangrove/data/mangrove.db ".backup /root/mangrove.db"
cp /var/lib/mangrove/data/master.key /root/
```

This needs the `sqlite3` CLI. `mangrovectl backup` does the same through the API
and is usually easier.
See [Backups and restore](../data/backups.md).
