# CLI and TUI

Two terminal clients ship with Mangrove. Both talk to the HTTP API, so they
work from any machine that can reach it.

```sh
go build -o mangrovectl  ./cmd/mangrovectl
go build -o mangrove-tui ./cmd/mangrove-tui
export MANGROVE_API_URL=https://mangrove.example.com   # default http://127.0.0.1:7777
```

Both share the session in `~/.mangrove/session`. Sign in once with either.

## mangrovectl

A scriptable command-line client. Run it with no arguments for usage.

```sh
mangrovectl login --email you@example.com --password '...'
mangrovectl logout
mangrovectl setup --email EMAIL --password PASSWORD     # first account on a fresh install
```

### Projects and deployments

```sh
mangrovectl project list
mangrovectl project create --name "My app" --slug my-app [--description TEXT]

mangrovectl deployment list --project my-app
mangrovectl deployment create --project my-app --name Web --slug web \
  --strategy dockerfile|nixpacks|compose|image \
  [--git-branch main] [--image-ref REF] [--root-path .] \
  [--dockerfile-path PATH] [--compose-path PATH] \
  [--service-name web] [--internal-port 3000] [--internal-only] \
  [--cpu 0.5] [--memory 256] [--health-path /healthz]

mangrovectl services --deployment 12
```

The CLI can't create static-site deployments; use the dashboard.

### Deploying

```sh
mangrovectl deploy --deployment 12 \
  [--git-url URL] [--git-ref REF] [--commit-sha SHA] [--commit-message MSG]
mangrovectl history  --deployment 12
mangrovectl rollback --history-id 345
```

`deploy` without `--git-url` deploys what the deployment is configured
with: its image, for an image deployment. For repository builds, pass the
URL. See [From GitLab, Gitea and others](../deploy/from-other-git.md).

### Backups

```sh
mangrovectl backup [--out PATH]      # owners only
```

See [Backups and restore](../data/backups.md).

### Scripting tips

- Output is JSON for list and create commands. Pipe it to `jq`.
- Errors go to stderr with a non-zero exit code.
- For anything the CLI doesn't cover (variables, domains, scaling), call
  the [HTTP API](api.md) with the same session cookie.

## mangrove-tui

A full-screen terminal dashboard.

```sh
mangrove-tui
```

If you're not signed in, it shows its own sign-in (or first-run setup)
screen.

Navigate **projects → deployments → a deployment**. On a deployment:

| Key | Action |
|---|---|
| `d` | Redeploy |
| `R` | Restart |
| `x` | Stop |
| `+` / `-` | Scale replicas up or down |
| `l` | Live logs |
| `h` | History; `enter` on a run rolls back to it |
| `t` | Shell inside the container |

Each view lists its keys at the bottom.

**The shell** (`t`) is a real interactive terminal in the container: bash
if available, otherwise sh, with resizing and full key handling. Exit the
shell, then press any key to return to the TUI.

More detail: [clients.md](../clients.md).
