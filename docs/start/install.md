# Install

Mangrove runs on a Debian or Ubuntu box with Docker and Caddy. The
installer sets everything up; you can also do it by hand.

## Requirements

- **A Linux box you don't use for anything else.** 2 GB of RAM is enough
  for Mangrove plus a few small apps. Mangrove needs Docker access, which
  is equivalent to root, so don't share the box with unrelated services.
- **Docker** with the Compose plugin.
- **Caddy**, with its admin API on `127.0.0.1:2019` (Caddy's default).
- **Go 1.25+ and Node 20+** to build Mangrove from source.
- **nixpacks** on the `PATH`, only if you want the nixpacks build strategy
  (see [Builds](../deploy/builds.md)). The installer doesn't install it.

## One command

```sh
git clone https://github.com/evanxdsouza/mangrove.git
cd mangrove
sudo ./setup.sh
```

`setup.sh`:

1. Installs Docker, Caddy, Go and Node if they're missing.
2. Builds the dashboard and the `mangrove` binary.
3. Creates the `mangrove` system user and its directories
   (`/var/lib/mangrove`, `/var/lib/mangrove-static`, `/etc/mangrove`).
4. Installs the systemd units and memory slices from `deploy/systemd/`.
5. Asks whether the box has a public IP or sits behind a home router, and
   sets up [DDNS](../deploy/domains.md#home-server-ddns) for the latter.
6. Asks whether to install the optional storage helper (for
   [turning a drive into a NAS](../deploy/storage.md#share-a-drive-as-a-nas)).
7. Starts the service and creates your admin account.

It prompts for the admin email and password. To run it unattended, pass
them in the environment instead:

```sh
sudo MANGROVE_ADMIN_EMAIL=me@example.com MANGROVE_ADMIN_PASSWORD='...' ./setup.sh
```

Re-running `setup.sh` is safe. Every step checks what's already there.

When it finishes, Mangrove is running at `http://127.0.0.1:7777`.

## Reach the dashboard

Mangrove's API and dashboard listen on **loopback only**
(`127.0.0.1:7777`). Nothing on the network can reach them until you choose
how. Two ways:

- **An SSH tunnel**, for a quick look or a private box:

  ```sh
  ssh -L 7777:127.0.0.1:7777 you@your-box
  ```

  Then open `http://localhost:7777` on your own machine.
- **A domain with HTTPS**, for everyday use: add a site to Caddy (or your
  host's edge) that reverse-proxies to `127.0.0.1:7777`. See
  [Going to production](../operate/production.md#serve-the-dashboard-over-https).

## Try it locally

To try Mangrove on a laptop or dev box without systemd:

```sh
caddy run &                                     # Caddy with its default admin API
cd web && npm install && npm run build && cd ..  # build the dashboard
go build -o mangrove ./cmd/mangrove
./mangrove
```

This starts Mangrove on `http://127.0.0.1:7777`, with its database and
encryption key in `./data`. The first visit asks you to create the first
account, which becomes the owner.

Mangrove creates its own Docker network (`mangrove-net`) the first time it
needs it.

## After installing

- Put the dashboard behind HTTPS (see above).
- Set up [backups](../data/backups.md). Losing the data directory's
  `master.key` makes every stored secret unrecoverable.
- [Deploy something](first-deploy.md).

## If the install fails

- **`apt-get not found`**: the installer only supports Debian and Ubuntu.
  On another distro, install Docker, Caddy, Go and Node yourself, then
  follow [Going to production](../operate/production.md) by hand.
- **The service starts but the dashboard is blank**: the dashboard wasn't
  built before the binary. Run `cd web && npm run build`, rebuild the
  binary, and restart. See [Upgrades](../operate/upgrades.md).
- **Port 7777 is taken**: set `MANGROVE_PORT` in `/etc/mangrove/mangrove.env`
  and restart. See [Configuration](../operate/configuration.md).

More in [Troubleshooting](../operate/troubleshooting.md).
