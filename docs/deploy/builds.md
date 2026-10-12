# Builds

Every deployment has a **build strategy**: how its source becomes something
Mangrove can run. Pick it when you create the deployment; **Deploy from
GitHub** picks one for you.

| Strategy | Use it for | Produces |
|---|---|---|
| **Dockerfile** | Any repo with a Dockerfile | An image, run as a container |
| **Nixpacks** | Node, Python, Go, Ruby and more, with no Dockerfile | An image, run as a container |
| **Docker Compose** | Several services that belong together | A compose stack, see [Compose stacks](../run/compose.md) |
| **Existing image** | A pre-built image from a registry | Nothing to build, see [From an image](from-an-image.md) |
| **Static site** | HTML, or a frontend that builds to files | A folder of files Caddy serves directly |

## How detection works

When Mangrove reads a repository, it takes the first match:

1. A `docker-compose.yml`, `docker-compose.yaml`, `compose.yml` or
   `compose.yaml` → **Docker Compose**.
2. A `Dockerfile` → **Dockerfile**, with the port taken from its last
   `EXPOSE`.
3. An `index.html` and no `package.json` → **Static site**, no build step.
4. A `package.json` whose build has no start command (a frontend that only
   builds files) → **Static site**, with nixpacks' build command and output
   directory.
5. Anything else → **Nixpacks**.

It also lists the environment variables the repository seems to expect,
guessing which are secret from their names. You review all of it before
deploying.

## Common settings

- **Root path**: the subdirectory to build from, for monorepos. Defaults to
  the repository root.
- **Branch**: which branch to build. Pushes to it redeploy when
  [auto-deploy](from-github.md#deploy-on-every-push) is on.
- **Dockerfile path** / **Compose file path**: if not at the default
  location.

## Dockerfile

Mangrove runs `docker build` with your Dockerfile and tags the image
`mangrove/<slug>-<service>:<run>`. A multi-stage Dockerfile builds its last
stage.

## Nixpacks

[Nixpacks](https://nixpacks.com) detects the language and builds an image
with no Dockerfile. It must be installed on the box (`nixpacks` on the
`PATH`).

Two settings are always passed to nixpacks builds:

- `NIXPACKS_NODE_VERSION=22`, because the last nixpacks release defaults to
  Node 18, which no longer installs.
- `NODE_OPTIONS=--max-old-space-size=1536`, which caps Node's heap so a
  large build fails with a clear error instead of being killed by the
  kernel.

Neither can be overridden yet. If an app needs another Node version, give
it a Dockerfile.

## Static site

Caddy serves the files directly. There's no container, so there are no logs,
replicas or health checks.

- **No build command**: Mangrove copies the **output directory** (or the
  whole repo) as-is.
- **With a build command** (such as `npm run build`): Mangrove builds with
  nixpacks, copies the **output directory** (such as `dist`) out of the
  result, and throws the builder image away.

Each run's output is kept in its own folder under
`/var/lib/mangrove-static`, so rolling back just points Caddy at an earlier
folder.

## Build-time variables

A deployment's [variables](../run/environments.md#variables) are given to
the running container, **not** to the build. Dockerfile builds get no
`--build-arg`s and nixpacks builds get no extra `--env`s. If a build step
needs a value (a public API URL baked into a frontend, say), put it in the
repository or a Dockerfile `ARG` default.

Compose stacks don't receive Mangrove's variables at all; configure them
in the compose file or an `.env` file in the repository. See
[Compose stacks](../run/compose.md).

## Build output and failures

A run's page in the **History** tab shows its status, trigger, commit and
duration. A failed run also shows the error. A failed build never touches
the running version.

**Cancel deploy** stops a build that's in progress. The run is marked failed
and the running version is left alone.

## Old images

Each deployment keeps its newest **5** images; older ones are pruned
automatically. Mangrove also prunes dangling images and build cache every
6 hours, and an owner can prune on demand from **Admin → Image / build
cache pruning**. You can only [roll back](rollbacks.md) to a run whose image
is still kept.
