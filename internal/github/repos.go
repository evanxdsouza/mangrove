package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ReposClient lists a token's accessible repos and (best-effort) registers
// push webhooks -- the two things a PAT/OAuth token can do beyond what
// StatusClient already covers.
type ReposClient struct {
	HTTPClient *http.Client
	BaseURL    string // overridable in tests; defaults to https://api.github.com
}

func NewReposClient() *ReposClient {
	return &ReposClient{HTTPClient: &http.Client{Timeout: 15 * time.Second}, BaseURL: "https://api.github.com"}
}

type Repo struct {
	Owner         string `json:"owner"`
	Name          string `json:"name"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	Description   string `json:"description"`
	PushedAt      string `json:"pushed_at"`
}

// ListRepos returns every repo the token's owner can push to (own +
// collaborator + org member), most-recently-pushed first, capped at 300
// (3 pages of 100) -- enough for a repo picker without unbounded pagination.
func (c *ReposClient) ListRepos(ctx context.Context, token string) ([]Repo, error) {
	var all []Repo
	for page := 1; page <= 3; page++ {
		url := fmt.Sprintf("%s/user/repos?affiliation=owner,collaborator,organization_member&visibility=all&sort=pushed&per_page=100&page=%d", c.BaseURL, page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, err
		}
		var raw []struct {
			Name          string `json:"name"`
			Private       bool   `json:"private"`
			DefaultBranch string `json:"default_branch"`
			Description   string `json:"description"`
			PushedAt      string `json:"pushed_at"`
			Owner         struct {
				Login string `json:"login"`
			} `json:"owner"`
		}
		err = func() error {
			defer resp.Body.Close()
			if resp.StatusCode >= 300 {
				return fmt.Errorf("github repos API: unexpected status %d", resp.StatusCode)
			}
			return json.NewDecoder(resp.Body).Decode(&raw)
		}()
		if err != nil {
			return nil, err
		}
		for _, r := range raw {
			all = append(all, Repo{
				Owner:         r.Owner.Login,
				Name:          r.Name,
				Private:       r.Private,
				DefaultBranch: r.DefaultBranch,
				Description:   r.Description,
				PushedAt:      r.PushedAt,
			})
		}
		if len(raw) < 100 {
			break
		}
	}
	return all, nil
}

// CreateWebhook registers a push + pull_request webhook on owner/repo
// pointed at callbackURL, signed with secret. Best-effort by convention --
// callers fall back to showing the user the manual paste-into-GitHub
// instructions on error, since a token that can read a repo doesn't always
// have admin:repo_hook scope (e.g. read-only org repos). pull_request
// drives per-PR preview deployments (see internal/api/webhook.go); push
// drives everything else (production/staging auto-deploy).
func (c *ReposClient) CreateWebhook(ctx context.Context, token, owner, repo, callbackURL, secret string) error {
	body, err := json.Marshal(map[string]any{
		"name":   "web",
		"active": true,
		"events": []string{"push", "pull_request"},
		"config": map[string]string{
			"url":          callbackURL,
			"content_type": "json",
			"secret":       secret,
		},
	})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/repos/%s/%s/hooks", c.BaseURL, owner, repo)
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
		return fmt.Errorf("github hooks API: unexpected status %d", resp.StatusCode)
	}
	return nil
}
