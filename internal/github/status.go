// Package github posts commit statuses back to GitHub on Mangrove's
// behalf -- so a webhook-triggered deploy shows up on the commit/PR the
// same way a CI check would, using the PAT already on file for that repo.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type State string

const (
	StatePending State = "pending"
	StateSuccess State = "success"
	StateFailure State = "failure"
)

type StatusClient struct {
	HTTPClient *http.Client
	BaseURL    string // overridable in tests; defaults to https://api.github.com
}

func NewStatusClient() *StatusClient {
	return &StatusClient{HTTPClient: &http.Client{Timeout: 10 * time.Second}, BaseURL: "https://api.github.com"}
}

// PostStatus sets a commit status on owner/repo@sha. Called from the
// deploy hot path, so a GitHub API hiccup here must never fail the deploy
// itself -- callers treat this as best-effort and only log on error.
func (c *StatusClient) PostStatus(ctx context.Context, token, owner, repo, sha string, state State, description, targetURL string) error {
	if sha == "" {
		return fmt.Errorf("commit sha is required")
	}
	body, err := json.Marshal(map[string]string{
		"state":       string(state),
		"description": truncate(description, 140),
		"context":     "mangrove",
		"target_url":  targetURL,
	})
	if err != nil {
		return fmt.Errorf("encode status body: %w", err)
	}

	url := fmt.Sprintf("%s/repos/%s/%s/statuses/%s", c.BaseURL, owner, repo, sha)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("github statuses API: unexpected status %d", resp.StatusCode)
	}
	return nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
