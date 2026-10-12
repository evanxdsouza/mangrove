# From GitLab, Gitea and others

Mangrove's repository linking, push webhooks, staging and PR previews are
GitHub-only today. You can still deploy from any Git host that serves
HTTPS clones. You just trigger the deploys yourself.

## Deploy a public repository

1. Create the deployment with the build strategy you want
   (**New deployment** in the dashboard, or `mangrovectl deployment create`).
   See [Builds](builds.md).
2. Trigger a deploy with the repository URL:

   ```sh
   mangrovectl deploy --deployment 12 \
     --git-url https://gitlab.com/you/app.git \
     --git-ref main
   ```

`--git-ref` takes a branch, a tag or a full commit SHA. Mangrove clones that
ref, builds it, and runs it through the same health-checked swap as any
other deploy.

## Private repositories

The deploy API (`POST /api/deployments/{id}/deploy`) accepts an
`auth_token` alongside `git_url`. Mangrove sends it as HTTP Basic auth with
the username `x-access-token`. GitHub accepts that; other hosts' token
schemes haven't been tested. The CLI doesn't expose this field, so call the
API directly. See [HTTP API](../interfaces/api.md).

## Deploy on push from CI

Without webhooks, have your CI call Mangrove after each push. For example,
from a GitLab CI job with a stored session cookie:

```sh
curl -fsS -X POST "$MANGROVE_URL/api/deployments/12/deploy" \
  -H 'Content-Type: application/json' \
  -b "mangrove_session=$MANGROVE_SESSION" \
  -d "{\"git_url\":\"$CI_REPOSITORY_URL\",\"git_ref\":\"$CI_COMMIT_SHA\",\"commit_sha\":\"$CI_COMMIT_SHA\"}"
```

Sessions last 30 days, so CI needs a way to refresh one. See
[Sign-in and sessions](../account/sign-in.md).

## What doesn't work yet

- **Redeploy** and **automatic deploy retries** reuse a deployment's linked
  repository. A deployment built from another host has none, so the
  **Redeploy** button returns an error. Trigger a new deploy with the URL
  instead, or roll back to an earlier run.
- No push webhooks, staging environments or PR previews for non-GitHub
  hosts.
