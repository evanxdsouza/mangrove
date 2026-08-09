// Package notify sends deploy-success emails via Resend. The API key is a
// runtime secret (MANGROVE_RESEND_API_KEY) -- never written to a file,
// config committed to git, or logged.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// resendAPIURL is a var (not const) so tests can point it at an
// httptest.Server instead of the real Resend API.
var resendAPIURL = "https://api.resend.com/emails"

type ResendClient struct {
	APIKey     string
	FromEmail  string // defaults to onboarding@resend.dev, which needs no domain verification
	HTTPClient *http.Client
}

func NewResendClient(apiKey, fromEmail string) *ResendClient {
	if fromEmail == "" {
		fromEmail = "Mangrove <onboarding@resend.dev>"
	}
	return &ResendClient{APIKey: apiKey, FromEmail: fromEmail, HTTPClient: &http.Client{}}
}

// Enabled reports whether an API key has been configured -- callers should
// skip sending (and just log to notifications_log as "failed"/skipped)
// rather than erroring when it hasn't, since notifications are optional.
func (c *ResendClient) Enabled() bool {
	return c != nil && c.APIKey != ""
}

type sendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

func (c *ResendClient) send(ctx context.Context, to, subject, html string) error {
	body, err := json.Marshal(sendRequest{From: c.FromEmail, To: []string{to}, Subject: subject, HTML: html})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendAPIURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("resend request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend API returned %s: %s", resp.Status, respBody)
	}
	return nil
}

// DeploySuccessParams is everything the email template needs -- the app
// name, the port it's reachable on, and a suggested domain slug the user
// copies into Nest's own domain dashboard by hand (Nest has no public API
// for this, so Mangrove never attempts to register anything itself).
type DeploySuccessParams struct {
	AppName          string
	Port             int
	SuggestedDomain  string
	CommitSHA        string
	DeployHistoryURL string
}

func (c *ResendClient) SendDeploySuccess(ctx context.Context, to string, p DeploySuccessParams) error {
	subject := fmt.Sprintf("%s deployed successfully", p.AppName)
	html := fmt.Sprintf(`
<div style="font-family: -apple-system, sans-serif; max-width: 480px; margin: 0 auto;">
  <h2 style="margin-bottom: 4px;">%s is up</h2>
  <p style="color: #666;">Deployed via Mangrove.</p>
  <table style="width: 100%%; border-collapse: collapse; margin-top: 16px;">
    <tr><td style="padding: 6px 0; color: #888;">Port</td><td style="padding: 6px 0;"><code>%d</code></td></tr>
    <tr><td style="padding: 6px 0; color: #888;">Suggested domain</td><td style="padding: 6px 0;"><a href="https://%s"><code>%s</code></a></td></tr>
    <tr><td style="padding: 6px 0; color: #888;">Commit</td><td style="padding: 6px 0;"><code>%s</code></td></tr>
  </table>
  <p style="color: #888; font-size: 13px; margin-top: 20px;">
    Nest has no public domain API, so this is a manual step: copy the port and suggested domain above into
    Nest's own domains dashboard yourself.
  </p>
</div>`, p.AppName, p.Port, p.SuggestedDomain, p.SuggestedDomain, shortSHA(p.CommitSHA))

	return c.send(ctx, to, subject, html)
}

func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	if sha == "" {
		return "n/a"
	}
	return sha
}
