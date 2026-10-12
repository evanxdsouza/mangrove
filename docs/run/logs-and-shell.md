# Logs and shell

## Live logs

A deployment's **Logs** tab streams its container's output as it happens,
starting with the last 200 lines. Pick the service if there's more than
one. It sticks to the bottom while new lines arrive; scroll up to pause.

Static sites have no container, so they have no Logs tab.

From the terminal:

- `mangrove-tui` → a deployment → `l` for live logs.
- The `get_logs` MCP tool, for agents.

Logs come from Docker (`docker logs`), so they're kept only as long as
the container is. A new deploy starts a new container, and with it a fresh
log.

### Why did my deploy fail?

Build errors aren't in the Logs tab. Open the failed run in the **History**
tab; its page shows the error. A container that crashed after starting
shows its own output in the Logs tab.

## Run a command

The **Run command** card on the Overview tab runs a one-off command inside
a running service and shows the output when it finishes:

```sh
python manage.py migrate
```

It's for short commands like migrations or checks. It isn't interactive and
it buffers all output, so don't use it for long-running processes. Editors
and above can use it. Agents get the same thing through the `run_command`
MCP tool.

## Interactive shell

For a real shell, use the terminal UI:

```sh
mangrove-tui
```

Open a deployment and press `t`. You get a full interactive shell inside the
container (bash if it has it, otherwise sh) with resizing, colours and
Ctrl-C. When the shell exits, press any key to return to the TUI.

The dashboard used to have an in-browser terminal; it was removed because
it wasn't reliable. See [CLI and TUI](../interfaces/cli.md).

## Mangrove's own logs

On the box:

```sh
journalctl -u mangrove -f
```

See [Host commands](../operate/host-commands.md).
