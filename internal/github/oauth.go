package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OAuthClient drives GitHub's classic OAuth App web flow (not a GitHub
// App): https://docs.github.com/en/apps/oauth-apps. The "repo" scope is
// requested unconditionally -- it's the only scope that covers both public
// and private repos, matching what a pasted PAT already needs to do the
// same job.
type OAuthClient struct {
	HTTPClient *http.Client
}

func NewOAuthClient() *OAuthClient {
	return &OAuthClient{HTTPClient: &http.Client{Timeout: 10 * time.Second}}
}

// AuthorizeURL builds the URL to send the user's browser to. state must be
// a random, unguessable value the caller can later verify in the callback.
func AuthorizeURL(clientID, redirectURI, state string) string {
	q := url.Values{
		"client_id":    {clientID},
		"redirect_uri": {redirectURI},
		"scope":        {"repo"},
		"state":        {state},
		"allow_signup": {"false"},
	}
	return "https://github.com/login/oauth/authorize?" + q.Encode()
}

// Exchange trades a callback's ?code= for an access token.
func (c *OAuthClient) Exchange(ctx context.Context, clientID, clientSecret, code, redirectURI string) (string, error) {
	form := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var out struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if out.Error != "" {
		return "", fmt.Errorf("github oauth error: %s (%s)", out.Error, out.ErrorDesc)
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("github oauth: no access_token in response")
	}
	return out.AccessToken, nil
}

// FetchLogin returns the GitHub username the token belongs to, so it can be
// shown in Mangrove's UI ("Connected as @alice") instead of a bare PAT label.
func (c *OAuthClient) FetchLogin(ctx context.Context, token string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("github user API: unexpected status %d", resp.StatusCode)
	}

	var out struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode user response: %w", err)
	}
	return out.Login, nil
}
