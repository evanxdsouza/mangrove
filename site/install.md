# Install Mangrove (instructions for an AI agent)

You are a coding agent installing **Mangrove**, a small self-hosted PaaS,
on the machine you are running on, for the person you are working with.
Follow these steps in order. Source: https://github.com/evanxdsouza/mangrove

## Ground rules

- **Only install on a Debian or Ubuntu machine the person says is meant
  for this.** Mangrove runs apps in Docker, and Docker access is
  equivalent to root, so the machine shouldn't host unrelated services.
- **Ask before every command that uses `sudo`, installs packages or
  changes system services.** Show the exact command first.
- **Never ask for, read, print or store the admin password.** The person
  types it into their own terminal in step 5. Don't set
  `MANGROVE_ADMIN_EMAIL` or `MANGROVE_ADMIN_PASSWORD` yourself.
- **Don't expose the dashboard.** Mangrove listens on `127.0.0.1:7777` on
  purpose. Don't open firewall ports or change the network.
- **If a step fails, stop and show the output.** Don't improvise fixes
  that edit system config.

## 1. Check the machine

Run these and summarise the results for the person:

```sh
cat /etc/os-release        # must be Debian or Ubuntu
free -h                    # 2 GB of RAM is enough for Mangrove plus a few small apps
df -h /                    # around 16 GB of disk is fine to start
ss -ltn | grep ':7777 '    # should print nothing: Mangrove needs port 7777
systemctl is-active caddy  # if Caddy already serves other sites, tell the person before going on
```

If it isn't Debian or Ubuntu, stop: the installer uses `apt-get`.

## 2. Get the code

```sh
git clone https://github.com/evanxdsouza/mangrove.git
cd mangrove
```

## 3. Run the installer (ask first)

```sh
sudo ./setup.sh
```

It takes several minutes. It installs Docker, Caddy, Go and Node if
they're missing, builds the dashboard and the `mangrove` binary, creates
the `mangrove` system user and its directories, installs the systemd
units, and starts the service.

Because your shell isn't an interactive terminal, the installer skips its
optional questions (home server or VPS, dynamic DNS, drive sharing) and
then **stops at account creation** with:

```
no admin account exists yet and this isn't an interactive terminal -- set MANGROVE_ADMIN_EMAIL and MANGROVE_ADMIN_PASSWORD and re-run
```

That's expected. Don't set those variables. Go to step 4.

If `sudo` asks for a password you can't enter, ask the person to run
`sudo ./setup.sh` in their own terminal instead, then continue at step 4.

## 4. Check it's running

```sh
systemctl is-active mangrove          # active
curl -s http://localhost:7777/healthz # ok
```

If either fails, show the person:

```sh
systemctl status mangrove --no-pager
journalctl -u mangrove -n 100 --no-pager
```

## 5. Hand over to the person

Tell them to run this **in their own terminal**, in the `mangrove`
directory:

```sh
sudo ./setup.sh
```

Re-running is safe: everything already done is skipped. This time it asks
whether the box is a VPS or a home server (and sets up dynamic DNS for a
home server), whether to set up drive sharing, and then for an email and
password, and creates the first account, which is the instance's owner.

Then they can reach the dashboard through an SSH tunnel from their own
computer:

```sh
ssh -L 7777:127.0.0.1:7777 <user>@<this-machine>
```

and open http://localhost:7777. To serve it over HTTPS on a domain, point
them at the "Going to production" page in the docs.

## 6. Optional: operate Mangrove with an agent

Mangrove ships an MCP server, `mangrove-mcp`, so an agent can list
deployments, read logs, redeploy, roll back, restart, scale and run
one-off commands. It deliberately can't delete things, manage users or
secrets, or change domains.

```sh
go build -o mangrove-mcp ./cmd/mangrove-mcp
```

Suggest the person creates a dedicated account for the agent (Admin →
Team), with only the workspace roles it needs. The client config looks
like this; the person fills in the password themselves:

```json
{
  "mcpServers": {
    "mangrove": {
      "command": "/path/to/mangrove-mcp",
      "env": {
        "MANGROVE_API_URL": "http://localhost:7777",
        "MANGROVE_EMAIL": "agent@example.com",
        "MANGROVE_PASSWORD": "..."
      }
    }
  }
}
```

## If the install fails

- **`apt-get: not found`**: not Debian or Ubuntu. Stop and tell the person.
- **The service runs but the dashboard is blank**: the dashboard wasn't
  built before the binary. `cd web && npm run build`, then re-run
  `sudo ./setup.sh`.
- **Port 7777 is taken**: the person can set `MANGROVE_PORT` in
  `/etc/mangrove/mangrove.env` and restart. Ask before editing it.

Full docs: https://github.com/evanxdsouza/mangrove/tree/main/docs
