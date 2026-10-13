# site/

The public landing page for Mangrove: one static `index.html` (inline CSS,
no build step) in the dashboard's Intertidal design language (see
[DESIGN.md](../DESIGN.md)), plus the self-hosted Figtree/JetBrains Mono
fonts copied from `web/node_modules/@fontsource`.

Preview locally by opening `index.html` in a browser, or:

```sh
docker build -t mangrove-site site/ && docker run --rm -p 8080:8080 mangrove-site
```

It's hosted on Orchard, built from `site/Dockerfile` (nginx-unprivileged on
port 8080, `/healthz` for health checks).
