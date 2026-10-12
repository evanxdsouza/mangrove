# From GitHub

A GitHub-linked project builds from its repository, redeploys on every
push, and unlocks [staging environments and PR previews](../run/environments.md).

## Give Mangrove access

Mangrove needs a GitHub token to clone private repositories, register
webhooks and post commit statuses. Two ways to get one:

**OAuth (recommended).** The owner registers a GitHub OAuth App at
github.com/settings/developers with the callback URL
`https://<your-dashboard>/api/github/oauth/callback`, and sets
`MANGROVE_GITHUB_OAUTH_CLIENT_ID` and `MANGROVE_GITHUB_OAUTH_CLIENT_SECRET`.
**Deploy from GitHub → Connect GitHub** then opens GitHub in a popup.
Mangrove asks for the `repo` scope, which covers private repositories and
lets it register the webhook for you.

**A personal access token.** Paste one under **Admin → GitHub Personal
Access Tokens**. It works for cloning and commit statuses, but Mangrove
can't tell which scopes it has, so it won't try to register webhooks with
it. You add the webhook yourself (below).

Tokens are encrypted at rest and never shown again after you save them.

## Deploy a repository

From a project, **Deploy from GitHub**:

1. Pick the GitHub account (if you've connected more than one) and the
   repository.
2. Mangrove clones it and detects a build strategy, the port and the
   environment variables. Review and change anything it got wrong.
3. **Deploy.**

This links the repo to the project, creates the deployment, sets its
variables and starts the first build. See [Builds](builds.md) for what each
strategy needs.

To link a repository without deploying yet, use the project's **GitHub**
tab → **Connect a repo** (owner, repository, default branch, and which
token to use).

## Deploy on every push

Each deployment has an **Auto-deploy** card on its Overview tab: a branch
and an on/off switch. When GitHub reports a push to that branch, Mangrove
builds and deploys that commit.

One repository can feed several deployments, each tracking a different
branch. That's how staging works.

While a deploy runs, Mangrove posts a commit status (pending, then success
or failure) to the commit on GitHub, the same way a CI check appears.

## The webhook

GitHub tells Mangrove about pushes by calling
`https://<your-dashboard>/webhooks/github/<token>`. That URL must be
reachable from the internet, so the dashboard needs a public HTTPS
address. Set `MANGROVE_PUBLIC_URL` if Mangrove sits behind a proxy that
doesn't pass the original scheme and host.

- **Connected with OAuth**: the webhook is registered for you when you link
  the repo.
- **Connected with a token**, or registration failed: the project's
  **GitHub** tab shows the webhook URL and secret. In the repo on GitHub,
  go to **Settings → Webhooks → Add webhook**, paste both, set the content
  type to `application/json`, and choose the push and pull request events.

The **GitHub** tab also shows recent deliveries: whether each one arrived,
whether its signature was valid, and whether it matched a deployment.
**Resync webhook** tries registering again.

## If a push doesn't deploy

Check the recent deliveries on the project's **GitHub** tab:

- **Nothing arrived.** GitHub can't reach the webhook URL. Check the
  dashboard is public, and look at the webhook's delivery log on GitHub.
- **Signature rejected.** The secret on GitHub doesn't match. Copy it again
  from the **GitHub** tab.
- **Arrived, no match.** No deployment has auto-deploy on for that branch.
  Check the **Auto-deploy** card.

More in [Webhooks](../run/webhooks.md).
