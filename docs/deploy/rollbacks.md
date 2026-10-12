# Rollbacks

The **History** tab lists every run of a deployment, newest first: its
trigger, commit, status and duration. The one serving traffic is marked
**current**.

## Roll back

On any successful run that isn't current, choose **Revert to this** (on the
run's page or in the timeline). From the CLI:

```sh
mangrovectl history --deployment 12      # find the run id
mangrovectl rollback --history-id 345
```

A rollback is an ordinary deploy that skips the build and reuses that run's
image: same side-by-side start, same health check, same switch-over. If the
old image doesn't pass its health check now, the current version keeps
running.

A rollback doesn't revert your repository, and it doesn't undo database
migrations. The next push deploys forward from the branch again.

You can only roll back to runs whose image is still kept. Each deployment
keeps its newest 5 images. Static sites roll back by pointing Caddy at an
earlier run's files.

## Recovering from a bad deploy

1. Roll back to the last run that worked.
2. Fix the code.
3. Push, which builds and deploys forward again.

A failed deploy never replaces a working one, so usually step 1 isn't even
needed: the old version is still running.

## Restart, Stop, Redeploy

| Action | What it does | Use it when |
|---|---|---|
| **Restart** | Restarts the same container, same image. | The process is stuck but the release is fine. |
| **Stop** | Stops the containers and removes the route. Frees [memory budget](scaling.md#the-memory-budget). | You don't need it running for now. **Restart** starts it again. |
| **Redeploy** | Builds the configured branch or image again and deploys it. | You changed variables, or want the latest commit without pushing. |
| **Cancel deploy** | Stops an in-progress build. | A build is stuck or you started the wrong one. |

Restart isn't health-check-gated: it cycles the one container that's there.

## Compose stacks

Compose stacks don't get the side-by-side switch-over. Deploying one runs
`docker compose up -d --build`, which recreates changed containers in
place, with a brief interruption. See [Compose stacks](../run/compose.md).
