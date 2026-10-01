package orchestrator

import (
	"context"
	"fmt"
	"strconv"

	"github.com/evanxdsouza/mangrove/internal/models"
)

// patAAD mirrors internal/api/github.go's own patAAD byte-for-byte -- it's
// the associated-data input to the same decrypt call envelope, so it must
// stay identical. Duplicated here (rather than importing the api package)
// to avoid an orchestrator -> api dependency inversion; api's own copy is
// the canonical definition, this one just has to match it.
func patAAD(id int64) []byte {
	return []byte("github_pats:" + strconv.FormatInt(id, 10))
}

// BuildRedeployRequest resolves the source a deployment is configured with
// into a DeployRequest ready to feed DispatchDeploy: for a git-backed
// deployment, the linked repo's URL/branch and decrypted PAT; for the image
// strategy, no source (a redeploy just reuses the image ref). Shared by
// internal/api/deployments.go's own buildRedeployRequest (the manual
// "Redeploy" button, scale, staging promote) and
// internal/scheduler/healer.go's automatic retry -- both need the exact
// same "what would redeploying this thing from its current config actually
// look like" resolution.
func (o *Orchestrator) BuildRedeployRequest(ctx context.Context, dep models.Deployment) (DeployRequest, error) {
	deployReq := DeployRequest{DeploymentID: dep.ID, TriggeredBy: "redeploy"}

	if dep.ProjectRepoID != nil {
		repo, err := o.Store.GetProjectRepoByID(ctx, *dep.ProjectRepoID)
		if err != nil {
			return deployReq, fmt.Errorf("load linked repo: %w", err)
		}
		ciphertext, nonce, err := o.Store.GetGithubPATEncrypted(ctx, repo.GithubPATID)
		if err != nil {
			return deployReq, fmt.Errorf("load repo credentials: %w", err)
		}
		token, err := o.Secrets.Open(patAAD(repo.GithubPATID), ciphertext, nonce)
		if err != nil {
			return deployReq, fmt.Errorf("decrypt repo credentials: %w", err)
		}
		o.Store.TouchGithubPATUsed(ctx, repo.GithubPATID)

		branch := dep.GitBranch
		if branch == "" {
			branch = repo.DefaultBranch
		}
		deployReq.GitURL = fmt.Sprintf("https://github.com/%s/%s.git", repo.RepoOwner, repo.RepoName)
		deployReq.GitRef = branch
		deployReq.AuthToken = string(token)
		return deployReq, nil
	}
	if dep.BuildStrategy != "image" {
		return deployReq, fmt.Errorf("deployment has no linked repository to redeploy from; link a repo first, or use POST .../deploy with explicit git parameters")
	}
	return deployReq, nil
}
