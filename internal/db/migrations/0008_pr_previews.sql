-- Per-PR preview deployments: opt a production deployment into spinning up
-- an ephemeral deployment for every open pull request against its linked
-- repo, torn down when the PR closes -- see internal/api/webhook.go's
-- handlePullRequestEvent and internal/github/comment.go.
--
-- A preview deployment is, like staging, just another deployments row
-- (environment='preview', promotes_to_deployment_id names the production
-- deployment it was cloned from) -- pr_number is what additionally keys it
-- to a specific PR instead of a hand-picked branch.
ALTER TABLE deployments ADD COLUMN pr_previews_enabled BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE deployments ADD COLUMN pr_number INTEGER;

-- The bot comment posted on the PR is edited in place on every subsequent
-- push/close rather than re-posted, so this is the one thing about a
-- preview deployment that isn't derivable from the deployments row alone.
ALTER TABLE deployments ADD COLUMN github_pr_comment_id INTEGER;

-- At most one preview deployment per (production deployment, PR number) --
-- a rapid double-delivery of the same "opened"/"synchronize" webhook must
-- find the existing row instead of creating a second one.
CREATE UNIQUE INDEX idx_deployments_preview_pr ON deployments(promotes_to_deployment_id, pr_number) WHERE pr_number IS NOT NULL;
