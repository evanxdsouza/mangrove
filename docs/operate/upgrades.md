# Upgrades

Mangrove is one binary with the dashboard built in. Upgrading means
building a new binary and swapping it in. Running apps keep running
throughout.

## Upgrade

```sh
cd /path/to/mangrove
git pull

cd web && npm ci && npm run build && cd ..     # rebuild the dashboard first
go build -o /root/mangrove-new ./cmd/mangrove  # then the binary that embeds it

cp /usr/local/bin/mangrove /root/mangrove-prev                # keep the old one
cp /root/mangrove-new /usr/local/bin/mangrove-staged
mv /usr/local/bin/mangrove-staged /usr/local/bin/mangrove      # atomic swap
systemctl restart mangrove

systemctl is-active mangrove
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:7777/healthz   # expect 200
```

Why copy then move, instead of copying straight over the binary: the
running process still has the old file open. Overwriting it in place can
crash it or fail with "text file busy". A `mv` within one filesystem is
atomic: the old process keeps its file until it exits.

Build the dashboard **before** the binary. A binary built first embeds the
old dashboard (or none at all).

On a small box, build somewhere with disk to spare, not in a RAM-backed
`/tmp`.

## What happens on restart

- **Database migrations** run automatically at startup. They only move
  forward; there are no down migrations.
- **Apps are untouched.** Containers keep running and Caddy keeps routing
  to them while Mangrove restarts.
- **Sessions survive.** Nobody is signed out.
- **Open dashboard tabs** pick up the new version on their next full page
  load.

## Roll back the binary

```sh
cp /root/mangrove-prev /usr/local/bin/mangrove-staged
mv /usr/local/bin/mangrove-staged /usr/local/bin/mangrove
systemctl restart mangrove
```

If the new version ran a migration, the old binary may not understand the
new schema. Take a [backup](../data/backups.md) before upgrading across
anything that changes the database, and restore it if you need to go back.

## Unit files

If an upgrade changes files in `deploy/systemd/`, copy them over and reload
systemd before restarting:

```sh
cp deploy/systemd/mangrove.service /etc/systemd/system/
systemctl daemon-reload
systemctl restart mangrove
```

Or re-run `sudo ./setup.sh`, which does all of the above and is safe to
repeat.
