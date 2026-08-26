package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// CommentClient posts and updates a single bot comment on a pull request --
// used for preview-deployment status (see internal/api/webhook.go's
// handlePullRequestEvent). Kept separate from StatusClient because a PR
// comment is addressed by issue number and, once posted, by its own comment
// id -- a different shape of GitHub API than a commit status.
type CommentClient struct {
	HTTPClient *http.Client
	BaseURL    string // overridable in tests; defaults to https://api.github.com
}

func NewCommentClient() *CommentClient {
	return &CommentClient{HTTPClient: &http.Client{Timeout: 10 * time.Second}, BaseURL: "https://api.github.com"}
}

// UpsertComment posts body as a new issue comment on owner/repo#prNumber,
// or -- if commentID is non-nil -- edits that existing comment in place.
// Editing in place (rather than posting a fresh comment on every push to a
// PR) is deliberate: a PR with a dozen pushes should show one live-updating
// preview-status comment, not a dozen stale ones. Returns the comment's id,
// which the caller persists (models.Deployment.GithubPRCommentID) so the
// next update knows to edit rather than create.
func (c *CommentClient) UpsertComment(ctx context.Context, token, owner, repo string, prNumber int, commentID *int64, body string) (int64, error) {
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return 0, fmt.Errorf("encode comment body: %w", err)
	}

	var method, url string
	if commentID != nil {
		method = http.MethodPatch
		url = fmt.Sprintf("%s/repos/%s/%s/issues/comments/%d", c.BaseURL, owner, repo, *commentID)
	} else {
		method = http.MethodPost
		url = fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", c.BaseURL, owner, repo, prNumber)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(payload))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return 0, fmt.Errorf("github issue comments API: unexpected status %d", resp.StatusCode)
	}

	var result struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode comment response: %w", err)
	}
	return result.ID, nil
}
