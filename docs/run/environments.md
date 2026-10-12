# Environments and variables

## Variables

Each service has its own environment variables, on the deployment's
**Env** tab.

- **Plain variables** are visible to anyone who can view the deployment.
- **Secret variables** are encrypted at rest and **write-only**: once
  saved, the value is never shown again, only the key. Only owners and
  workspace admins can set them.

**Variables apply on the next deploy.** Saving one doesn't touch the
running container. Press **Redeploy** to start a new version with the new
values. (**Restart** keeps the old container and its old values.)

Variables are given to the running container only, not to the build. See
[Builds](../deploy/builds.md#build-time-variables).

To point one app at another, use the other service's internal alias, such
as `mangrove-blog-db-app`. See [Networking](../deploy/networking.md).

## Environments

Every deployment is **production**, **staging** or **preview**. Staging and
preview deployments aren't new infrastructure: each is a full deployment of
its own, copied from a production one, with its own port, history and
settings. They need the project to be [linked to GitHub](../deploy/from-github.md).

### Staging

On a production deployment's Overview tab, in **Staging environments**,
enter a **Branch to stage** and choose **+ New staging environment**.

Mangrove copies the production deployment's build settings, services and
variables (secrets included), turns on auto-deploy for that branch, and
deploys it. From then on, pushes to the branch update staging, and pushes to
production's branch update production, independently.

Staging deployments are ordinary deployments: protect them with a
[password](../deploy/protection.md), give them a [domain](../deploy/domains.md),
roll them back.

### Promote

On a staging deployment, **Promote to production** deploys production with
the **exact commit staging is running**, not whatever is newest on
production's branch. What you tested is what ships.

If staging's current run has no recorded commit (it was last redeployed by
hand), promote falls back to the last ref it recorded.

### PR previews

On a production deployment, turn on **PR previews**. Then for every pull
request against its repository:

- **Opened or pushed to**: Mangrove creates (or updates) a preview
  deployment, copied from production, running the PR's exact head commit.
- **A bot comment** on the PR shows "building", then the preview's address
  or the error. It's edited in place, not reposted.
- **Closed or merged**: the preview is deleted, with its containers, route
  and port.

A preview whose PR has had no pushes for a week is deleted anyway, in case
the "closed" event never arrived. Change that with
`MANGROVE_PREVIEW_MAX_AGE_HOURS` (`0` turns it off).

Previews count against the [memory budget](../deploy/scaling.md#the-memory-budget)
like any deployment, so on a small box keep an eye on how many PRs are
open.

How staging and previews work underneath:
[architecture.md](../architecture.md#staging-environments).
