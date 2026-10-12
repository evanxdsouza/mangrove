# Webhooks

Mangrove receives webhooks from GitHub to deploy on push and to manage PR
previews. It doesn't send webhooks of its own; for outgoing events, see
[Notifications](notifications.md).

## The endpoint

```
POST https://<your-dashboard>/webhooks/github/<token>
```

Each linked repository gets its own random token and secret. The endpoint
is the only part of Mangrove, besides the dashboard, that has to be
reachable from the internet. It doesn't use sign-in. Instead:

- Every delivery's `X-Hub-Signature-256` is checked against the repo's
  secret before anything else happens.
- Each delivery is recorded by its GitHub delivery ID, so a GitHub retry
  never deploys twice.

## Events

| Event | What Mangrove does |
|---|---|
| `push` | Deploys the pushed commit to every deployment with auto-deploy on for that branch |
| `pull_request` opened, reopened, synchronize | Creates or updates the PR's preview, if PR previews are on |
| `pull_request` closed | Deletes the PR's preview |
| anything else | Recorded and ignored |

Only one deploy of a deployment runs at a time. A push that arrives while
that deployment is already deploying is **skipped**, not queued. If the
deploy that was running built an older commit, press **Redeploy** (or push
again) to get the newest one.

## Setting it up

Linking a repository with a GitHub OAuth connection registers the webhook
for you. With a pasted token, add it yourself from the values on the
project's **GitHub** tab. See
[From GitHub](../deploy/from-github.md#the-webhook).

## Delivery log

The project's **GitHub** tab lists recent deliveries, each one:

- **verified** or **rejected** (bad signature);
- **processed** (it started a deploy) or **ignored_no_match** (nothing was
  set to deploy for it).

**Resync webhook** tries registering again; the URL and secret can be shown
again at any time.

## Deploy from anywhere else

For CI systems or Git hosts other than GitHub, call the deploy API instead
of a webhook. See [From GitLab, Gitea and others](../deploy/from-other-git.md#deploy-on-push-from-ci).
