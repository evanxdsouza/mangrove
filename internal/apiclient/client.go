// Package apiclient is a typed Go client for Mangrove's own HTTP API,
// shared by cmd/mangrove-tui and cmd/mangrove-mcp so the two don't grow
// independent, silently-diverging notions of what the API returns --
// exactly the class of bug that broke the Workspaces tab (see
// store.WorkspaceProjectCount's history: a hand-shaped frontend
// expectation drifted from an untagged Go struct's default JSON
// encoding). Responses are decoded straight into the same
// internal/models / internal/store types the backend itself returns and
// writes with writeJSON, not a redeclared shadow struct, so a schema
// change to those types is a compile error here rather than a silent
// runtime mismatch.
//
// cmd/mangrovectl predates this package and isn't migrated to it --
// changing already-working, separately-tested code purely for
// architectural symmetry isn't worth the regression risk.
package apiclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/evanxdsouza/mangrove/internal/auth"
	"github.com/evanxdsouza/mangrove/internal/models"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// DefaultBaseURL matches mangrovectl's default and internal/config's
// MANGROVE_PORT default -- the dashboard/API's local address.
const DefaultBaseURL = "http://127.0.0.1:7777"

// Client is a small, cookie-authenticated HTTP client for Mangrove's API.
// It is not safe for concurrent Login/session-mutating calls, but read
// calls (the majority) are.
type Client struct {
	BaseURL string
	HTTP    *http.Client
	// SessionFile, when set, persists the session cookie across process
	// restarts (mangrove-tui and mangrove-mcp are both typically
	// re-invoked fresh each run, like mangrovectl) -- mirrors
	// mangrovectl's own ~/.mangrove/session convention so `mangrovectl
	// login` and `mangrove-tui`/`mangrove-mcp` can share one login.
	SessionFile string

	session string // in-memory cache of the cookie value
}

// New returns a Client for baseURL, using DefaultSessionFile for session
// persistence. baseURL is typically config.MANGROVE_API_URL or
// DefaultBaseURL.
func New(baseURL string) *Client {
	return &Client{
		BaseURL:     baseURL,
		HTTP:        http.DefaultClient,
		SessionFile: DefaultSessionFile(),
	}
}

// DefaultSessionFile returns ~/.mangrove/session -- the same path
// mangrovectl uses, so a login from one tool is visible to the others.
func DefaultSessionFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".mangrove", "session")
}

// LoadSession reads a previously-saved session token from SessionFile, if
// any, into memory. Not an error if the file doesn't exist yet (fresh
// install, never logged in) -- IsAuthenticated will just be false.
func (c *Client) LoadSession() {
	if c.SessionFile == "" {
		return
	}
	if b, err := os.ReadFile(c.SessionFile); err == nil {
		c.session = string(b)
	}
}

// IsAuthenticated reports whether a session token is currently held (does
// not verify it's still valid server-side -- the next request will surface
// that as an APIError with Status 401).
func (c *Client) IsAuthenticated() bool {
	return c.session != ""
}

func (c *Client) saveSession(token string) {
	c.session = token
	if c.SessionFile == "" {
		return
	}
	if token == "" {
		os.Remove(c.SessionFile)
		return
	}
	os.MkdirAll(filepath.Dir(c.SessionFile), 0700)
	os.WriteFile(c.SessionFile, []byte(token), 0600)
}

// APIError is returned for any non-2xx response other than an
// authentication failure (which do returns as ErrNotAuthenticated so
// callers can special-case "please log in" without string-matching).
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string { return fmt.Sprintf("%d: %s", e.Status, e.Message) }

// ErrNotAuthenticated is returned by do (wrapped, so errors.Is works) when
// the API responds 401 -- no valid session cookie.
var ErrNotAuthenticated = &APIError{Status: http.StatusUnauthorized, Message: "not authenticated"}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reqBody)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.session != "" {
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: c.session})
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("request to mangrove API at %s failed (is mangrove running?): %w", c.BaseURL, err)
	}
	defer resp.Body.Close()

	for _, ck := range resp.Cookies() {
		if ck.Name == auth.SessionCookieName {
			c.saveSession(ck.Value)
		}
	}

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrNotAuthenticated
	}
	if resp.StatusCode >= 400 {
		msg := string(respBody)
		var errBody struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(respBody, &errBody) == nil && errBody.Error != "" {
			msg = errBody.Error
		}
		return &APIError{Status: resp.StatusCode, Message: msg}
	}
	if out != nil && len(respBody) > 0 {
		return json.Unmarshal(respBody, out)
	}
	return nil
}

// ---- auth ----

// CurrentUser mirrors the ad hoc {id, email, role} object authMe/authLogin
// return -- there's no exported Go type for it server-side (internal/api's
// map[string]any), so it's redeclared here deliberately, unlike the model
// types below which are decoded straight from internal/models/internal/store.
type CurrentUser struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// AuthStatus is GET /api/auth/status's response.
type AuthStatus struct {
	SetupRequired bool `json:"setup_required"`
}

func (c *Client) AuthStatus(ctx context.Context) (AuthStatus, error) {
	var out AuthStatus
	err := c.do(ctx, http.MethodGet, "/api/auth/status", nil, &out)
	return out, err
}

// Login authenticates and persists the session (via SessionFile, if set)
// for subsequent calls/process invocations.
func (c *Client) Login(ctx context.Context, email, password string) (CurrentUser, error) {
	var out CurrentUser
	err := c.do(ctx, http.MethodPost, "/api/auth/login", map[string]string{"email": email, "password": password}, &out)
	return out, err
}

// Setup creates the one-time initial admin account (only valid when
// AuthStatus.SetupRequired is true) and logs in as it.
func (c *Client) Setup(ctx context.Context, email, password string) (CurrentUser, error) {
	var out CurrentUser
	err := c.do(ctx, http.MethodPost, "/api/auth/setup", map[string]string{"email": email, "password": password}, &out)
	return out, err
}

func (c *Client) Logout(ctx context.Context) error {
	err := c.do(ctx, http.MethodPost, "/api/auth/logout", nil, nil)
	c.saveSession("")
	return err
}

func (c *Client) Me(ctx context.Context) (CurrentUser, error) {
	var out CurrentUser
	err := c.do(ctx, http.MethodGet, "/api/auth/me", nil, &out)
	return out, err
}

// ---- workspaces / projects ----

func (c *Client) ListWorkspaces(ctx context.Context) ([]store.WorkspaceProjectCount, error) {
	var out []store.WorkspaceProjectCount
	err := c.do(ctx, http.MethodGet, "/api/workspaces", nil, &out)
	return out, err
}

// ListProjects lists every project, or (workspaceID != nil) only those in
// one workspace.
func (c *Client) ListProjects(ctx context.Context, workspaceID *int64) ([]store.ProjectWithWorkspace, error) {
	path := "/api/projects"
	if workspaceID != nil {
		path += "?workspace_id=" + strconv.FormatInt(*workspaceID, 10)
	}
	var out []store.ProjectWithWorkspace
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	return out, err
}

func (c *Client) GetProject(ctx context.Context, id int64) (models.Project, error) {
	var out models.Project
	err := c.do(ctx, http.MethodGet, "/api/projects/"+strconv.FormatInt(id, 10), nil, &out)
	return out, err
}

// ---- deployments / services ----

func (c *Client) ListDeployments(ctx context.Context, projectID int64) ([]models.Deployment, error) {
	var out []models.Deployment
	err := c.do(ctx, http.MethodGet, "/api/projects/"+strconv.FormatInt(projectID, 10)+"/deployments", nil, &out)
	return out, err
}

func (c *Client) GetDeployment(ctx context.Context, id int64) (models.Deployment, error) {
	var out models.Deployment
	err := c.do(ctx, http.MethodGet, "/api/deployments/"+strconv.FormatInt(id, 10), nil, &out)
	return out, err
}

func (c *Client) ListServices(ctx context.Context, deploymentID int64) ([]models.Service, error) {
	var out []models.Service
	err := c.do(ctx, http.MethodGet, "/api/deployments/"+strconv.FormatInt(deploymentID, 10)+"/services", nil, &out)
	return out, err
}

func (c *Client) GetService(ctx context.Context, id int64) (models.Service, error) {
	var out models.Service
	err := c.do(ctx, http.MethodGet, "/api/services/"+strconv.FormatInt(id, 10), nil, &out)
	return out, err
}

func (c *Client) ListDeployHistory(ctx context.Context, deploymentID int64) ([]models.DeployHistory, error) {
	var out []models.DeployHistory
	err := c.do(ctx, http.MethodGet, "/api/deployments/"+strconv.FormatInt(deploymentID, 10)+"/history", nil, &out)
	return out, err
}

// DeployOutcome is the {deploy_history_id, status|error} shape
// redeployDeployment/scaleDeployment/promoteDeployment all return.
type DeployOutcome struct {
	DeployHistoryID int64  `json:"deploy_history_id"`
	Status          string `json:"status,omitempty"`
	Error           string `json:"error,omitempty"`
}

// Redeploy re-runs the build+deploy pipeline against whatever source the
// deployment is already configured with. A failed deploy is reported via
// DeployOutcome.Error, not a non-nil error -- the request itself
// succeeded; the deploy it triggered didn't. A non-nil error means the
// request itself was rejected (e.g. a deploy already in progress).
func (c *Client) Redeploy(ctx context.Context, deploymentID int64) (DeployOutcome, error) {
	var out DeployOutcome
	err := c.do(ctx, http.MethodPost, "/api/deployments/"+strconv.FormatInt(deploymentID, 10)+"/redeploy", nil, &out)
	if apiErr, ok := err.(*APIError); ok && apiErr.Status == http.StatusUnprocessableEntity {
		return out, nil // deploy failed but the outcome body still decoded
	}
	return out, err
}

func (c *Client) Rollback(ctx context.Context, deployHistoryID int64) (DeployOutcome, error) {
	var out DeployOutcome
	err := c.do(ctx, http.MethodPost, "/api/deploy-history/"+strconv.FormatInt(deployHistoryID, 10)+"/rollback", nil, &out)
	return out, err
}

func (c *Client) Stop(ctx context.Context, deploymentID int64) error {
	return c.do(ctx, http.MethodPost, "/api/deployments/"+strconv.FormatInt(deploymentID, 10)+"/stop", nil, nil)
}

func (c *Client) Restart(ctx context.Context, deploymentID int64) error {
	return c.do(ctx, http.MethodPost, "/api/deployments/"+strconv.FormatInt(deploymentID, 10)+"/restart", nil, nil)
}

// Scale sets a deployment's replica count (1-32, single-service
// strategies only) and triggers a redeploy so it takes effect -- see
// docs/architecture.md's "Lifecycle actions short of a full deploy".
func (c *Client) Scale(ctx context.Context, deploymentID int64, replicas int) (DeployOutcome, error) {
	var out DeployOutcome
	err := c.do(ctx, http.MethodPost, "/api/deployments/"+strconv.FormatInt(deploymentID, 10)+"/scale",
		map[string]int{"replicas": replicas}, &out)
	if apiErr, ok := err.(*APIError); ok && apiErr.Status == http.StatusUnprocessableEntity {
		return out, nil
	}
	return out, err
}

// ExecResult is the outcome of RunCommand -- mirrors executor.ExecResult's
// JSON shape ({output, exit_code}), redeclared here since executor isn't
// otherwise part of this package's dependency surface.
type ExecResult struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
}

// RunCommand runs a one-off command in a service's running container (e.g.
// a database migration) via `docker exec`, synchronously, buffering
// output. For a long-running or interactive session, use a real terminal
// instead (mangrove-tui's shell view, or the dashboard's Terminal tab) --
// see docs/architecture.md.
func (c *Client) RunCommand(ctx context.Context, serviceID int64, command []string) (ExecResult, error) {
	var out ExecResult
	err := c.do(ctx, http.MethodPost, "/api/services/"+strconv.FormatInt(serviceID, 10)+"/exec",
		map[string][]string{"command": command}, &out)
	return out, err
}

// Logs fetches a bounded snapshot of a service's container log tail --
// unlike the dashboard's live-tailing SSE view, this waits for the
// response to fully arrive and returns it as a single string, one log
// line per element. follow=false is what makes this bounded: it maps to
// `docker logs --tail N` (no `-f`), which exits on its own once it's
// printed history, rather than streaming forever.
func (c *Client) Logs(ctx context.Context, serviceID int64, tailLines int) ([]string, error) {
	if tailLines <= 0 {
		tailLines = 200
	}
	path := fmt.Sprintf("/api/services/%d/logs/stream?tail=%d&follow=false", serviceID, tailLines)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if c.session != "" {
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: c.session})
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to mangrove API at %s failed (is mangrove running?): %w", c.BaseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrNotAuthenticated
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{Status: resp.StatusCode, Message: string(body)}
	}
	return parseSSELines(resp.Body), nil
}

// StreamLogs opens a live-following log stream (the SSE endpoint's default
// behavior: `docker logs -f`, never ending on its own) and returns the raw
// response body for the caller to read framed "data: ..." lines from as
// they arrive -- e.g. mangrove-tui's logs view. Unlike Logs (a bounded
// snapshot), the caller owns the connection's lifetime: cancel ctx to stop
// it, and Close the returned body when done either way.
func (c *Client) StreamLogs(ctx context.Context, serviceID int64, tailLines int) (io.ReadCloser, error) {
	if tailLines <= 0 {
		tailLines = 200
	}
	path := fmt.Sprintf("/api/services/%d/logs/stream?tail=%d", serviceID, tailLines)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if c.session != "" {
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: c.session})
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to mangrove API at %s failed (is mangrove running?): %w", c.BaseURL, err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		return nil, ErrNotAuthenticated
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, &APIError{Status: resp.StatusCode, Message: string(body)}
	}
	return resp.Body, nil
}

// parseSSELines extracts the payload of every "data: ..." SSE frame from
// r -- the same framing streamServiceLogs (internal/api/logs.go) writes.
func parseSSELines(r io.Reader) []string {
	var lines []string
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		if payload, ok := strings.CutPrefix(scanner.Text(), "data: "); ok {
			lines = append(lines, payload)
		}
	}
	return lines
}
