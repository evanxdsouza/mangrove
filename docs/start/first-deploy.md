# First deploy

The shortest path: a GitHub repo, deployed, reachable on a port, redeploying
itself on every push.

```
GitHub repo ─push─▶ Build on the box ─▶ Start next to the old version
                                          │
                                          ├─ healthy ─▶ Caddy switches traffic ─▶ old version removed
                                          └─ not healthy ─▶ new version removed, old one keeps serving
```

## 1. Create a project

**Projects → New project.** A project groups related deployments, such as
an app and its database. Give it a name; the slug is filled in for you.

## 2. Deploy from GitHub

Open the project and choose **Deploy from GitHub**.

1. **Connect GitHub.** If the owner has set up a GitHub OAuth app, this
   opens GitHub in a popup. Otherwise, paste a personal access token under
   **Admin → GitHub Personal Access Tokens** first. See
   [From GitHub](../deploy/from-github.md).
2. **Pick a repository.** Private repositories work if your token can see
   them.
3. **Check what Mangrove detected.** It reads the repo and picks a build
   strategy: a compose file, then a Dockerfile, then a plain `index.html`,
   then nixpacks for anything else. It suggests the port from the
   Dockerfile's `EXPOSE`, and lists the environment variables the repo
   seems to expect. Fill those in, and mark anything sensitive as secret.
4. **Deploy.**

Mangrove links the repo to the project, creates the deployment, and starts
the first build. Every push to the chosen branch deploys again from now on.

## 3. Watch it build

The deployment's page shows the build as a run in the **History** tab, and
the status at the top moves from building to running. A failed build or a
failed health check leaves the previous version running, so a bad push never
takes a working app down.

## 4. Open it

Each public deployment gets its own port from Mangrove's range (20000 to
21000 by default). The service card on the **Overview** tab shows it. Your
app is at `http://<your-box>:<port>`.

For a real domain with HTTPS, add one from the **Domains** card. See
[Domains and HTTPS](../deploy/domains.md).

## 5. Give it a database

**Deploy from template → PostgreSQL** (or MySQL, MongoDB, Redis) in the
same project. Database templates are internal-only, so nothing outside the
box can reach them. Your app connects over Mangrove's Docker network. See
[Databases](../data/databases.md).

## If the build fails

- **No Dockerfile, and nixpacks says "No start command could be found".**
  The repo is probably a static frontend. Re-create the deployment with the
  **Static** strategy, a build command (`npm run build`) and an output
  directory (`dist`). See [Builds](../deploy/builds.md).
- **The health check never passes.** The app is listening on a different
  port from the one configured, or only on `127.0.0.1` inside the
  container. Make it listen on `0.0.0.0` and the configured internal port.
- **It needs a database at build time.** Run migrations when the app
  starts, not during the build. You can also run them by hand with
  **Run command** on the Overview tab.

The error from a failed run is on its page in the **History** tab.
More in [Troubleshooting](../operate/troubleshooting.md).
