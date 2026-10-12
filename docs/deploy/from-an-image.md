# From an image

Run a pre-built container image without building anything: your own image
from a registry, or an off-the-shelf one like `nginx:alpine`.

## Deploy an image

From a project, **New deployment**:

1. Choose the **Existing image** build strategy.
2. Enter the image reference, such as `ghcr.io/you/app:1.4.2`.
3. Set the internal port the image listens on, its memory and CPU limits,
   and a health check path if it has one.
4. Create it, then press **Redeploy** on its page to start the first
   deploy. (A new deployment is *pending* until then.)

Or from the CLI:

```sh
mangrovectl deployment create --project my-app --name Web --slug web \
  --strategy image --image-ref ghcr.io/you/app:1.4.2 --internal-port 8080
mangrovectl deploy --deployment 12
```

Mangrove pulls the image and runs it through the same health-checked swap
as a built deployment.

## Updating

**Redeploy** pulls the configured reference again, so a moving tag like
`:latest` picks up a newer image. To pin a version, use a specific tag or a
digest (`app@sha256:...`).

## Private registries

Mangrove runs `docker pull` as the `mangrove` user. To pull from a private
registry, log that user's Docker client in on the box:

```sh
sudo -u mangrove HOME=/var/lib/mangrove docker login ghcr.io
```

## Good to know

- An image deployment needs no linked repository, so **Redeploy** and
  [self-healing](health-checks.md) retries always work for it.
- Most [templates](../run/templates.md) are image deployments with the
  wiring done for you.
