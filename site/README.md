# site/

The public landing page for Mangrove: one static `index.html` (inline CSS,
no build step) in the dashboard's Intertidal design language (see
[DESIGN.md](../DESIGN.md)), plus the self-hosted Figtree/JetBrains Mono
fonts copied from `web/node_modules/@fontsource`.

The page is the product's own first hour, built from the design canvas's
boards rather than generic marketing sections: it opens as the first-run
setup card (install commands in the form fields), then the New deployment
dialog, a first deploy that plays from queued to running, a projects grid,
the deployment-settings and admin boards, and Simple mode. The direction
contract is the HTML comment at the top of `<body>`. All project names,
domains and numbers on it are illustrative and labelled as examples.

Preview locally by opening `index.html` in a browser, or:

```sh
docker build -t mangrove-site site/ && docker run --rm -p 8080:8080 mangrove-site
```

It's hosted on Orchard, built from `site/Dockerfile` (nginx-unprivileged on
port 8080, `/healthz` for health checks).
