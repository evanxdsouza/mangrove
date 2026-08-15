// Package proxy drives Caddy's local admin API to reverse-proxy Mangrove's
// allocated ports to whichever container is currently healthy for a
// deployment. Caddy (not Traefik) was chosen specifically because its
// admin API lets Mangrove push exact route config imperatively the moment
// a deploy/swap completes, matching Mangrove's SQLite-as-source-of-truth
// model instead of fighting a label-polling discovery loop -- see plan §1.
//
// Each public deployment gets its own Caddy HTTP server block, keyed by
// its allocated host port (srv_<port>), listening on that port and
// reverse-proxying to the current container's internal docker-network
// address. Swapping which container serves traffic is a single PUT to
// that server's config -- Caddy applies it atomically, so there is no
// window where the port accepts connections but nothing answers them.
package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const defaultAdminAddr = "http://127.0.0.1:2019"

type Client struct {
	AdminAddr string
	HTTP      *http.Client
}

func NewClient(adminAddr string) *Client {
	if adminAddr == "" {
		adminAddr = defaultAdminAddr
	}
	return &Client{AdminAddr: adminAddr, HTTP: &http.Client{}}
}

// RouteOptions configures a deployment's Caddy route beyond the basic
// upstream proxy.
type RouteOptions struct {
	// PasswordProtected, when true, requires HTTP basic auth (via
	// BcryptHash) before the reverse proxy handler runs -- enforced at the
	// proxy layer, independent of whatever auth the app itself has.
	PasswordProtected bool
	Username          string
	BcryptHash        string // bcrypt hash, as Caddy's http_basic provider requires (not argon2id)
}

// EnsureBaseConfig makes sure apps.http.servers exists in Caddy's running
// config, bootstrapping it via /load if this is a fresh Caddy install
// still running its default Caddyfile-derived config. Safe to call
// repeatedly.
func (c *Client) EnsureBaseConfig(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.AdminAddr+"/config/apps/http/servers", nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("reach caddy admin API at %s: %w", c.AdminAddr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if bytes.TrimSpace(body) != nil && string(bytes.TrimSpace(body)) != "null" {
			return nil // already initialized
		}
	}

	base := map[string]any{
		"apps": map[string]any{
			"http": map[string]any{
				"servers": map[string]any{},
			},
		},
	}
	return c.load(ctx, base)
}

func (c *Client) load(ctx context.Context, config any) error {
	body, err := json.Marshal(config)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.AdminAddr+"/load", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("POST /load: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST /load: %s: %s", resp.Status, respBody)
	}
	return nil
}

// authHandlers returns the leading Caddy handler chain enforcing HTTP basic
// auth when opts.PasswordProtected is set -- shared by PutRoute and
// PutFileServerRoute so password protection works the same way regardless
// of what's serving the actual response.
func authHandlers(opts RouteOptions) []map[string]any {
	if !opts.PasswordProtected {
		return nil
	}
	return []map[string]any{
		{
			"handler": "authentication",
			"providers": map[string]any{
				"http_basic": map[string]any{
					"accounts": []map[string]any{
						{"username": opts.Username, "password": opts.BcryptHash},
					},
					"realm": "mangrove",
				},
			},
		},
	}
}

// PutRoute creates or atomically replaces the server block for a public
// port, reverse-proxying to upstreamAddr (a "host:port" reachable from the
// Mangrove host, typically a container's address on mangrove-net).
func (c *Client) PutRoute(ctx context.Context, port int, upstreamAddr string, opts RouteOptions) error {
	return c.PutRouteMulti(ctx, port, []string{upstreamAddr}, opts)
}

// PutRouteMulti is PutRoute for a replicated deployment: it configures the
// server block for a public port with several upstreams, which Caddy load
// balances across by default. When len(upstreams) == 1 it is exactly
// PutRoute. The swap is atomic either way -- a single PATCH/PUT replaces
// the whole route, so traffic never splits between old and new replicas.
func (c *Client) PutRouteMulti(ctx context.Context, port int, upstreams []string, opts RouteOptions) error {
	dials := make([]map[string]any, 0, len(upstreams))
	for _, u := range upstreams {
		dials = append(dials, map[string]any{"dial": u})
	}

	handlers := authHandlers(opts)
	handlers = append(handlers, map[string]any{
		"handler":   "reverse_proxy",
		"upstreams": dials,
		// Nest's edge terminates HTTPS for visitors but forwards plain HTTP
		// to this box with no indication of the original scheme. Without
		// this, backend apps (Ghost, WordPress, etc.) assume HTTP and can
		// misbuild absolute URLs or, worse, redirect-loop enforcing HTTPS
		// against a connection that's actually HTTP.
		"headers": map[string]any{
			"request": map[string]any{
				"set": map[string][]string{
					"X-Forwarded-Proto": {"https"},
				},
			},
		},
	})

	server := map[string]any{
		"listen": []string{fmt.Sprintf(":%d", port)},
		"routes": []map[string]any{
			{
				"handle": handlers,
			},
		},
	}

	return c.upsertPath(ctx, fmt.Sprintf("/config/apps/http/servers/srv_%d", port), server)
}

// PutFileServerRoute creates or atomically replaces the server block for a
// public port so it serves rootDir directly via Caddy's file_server --
// used by static-strategy deploys, which have no running app container to
// reverse-proxy to. rootDir must be an absolute host path reachable by the
// Caddy process (see executor.DockerExecutor.StaticSitesDir).
func (c *Client) PutFileServerRoute(ctx context.Context, port int, rootDir string, opts RouteOptions) error {
	handlers := authHandlers(opts)
	handlers = append(handlers, map[string]any{
		"handler": "file_server",
		"root":    rootDir,
	})

	server := map[string]any{
		"listen": []string{fmt.Sprintf(":%d", port)},
		"routes": []map[string]any{
			{
				"handle": handlers,
			},
		},
	}

	return c.upsertPath(ctx, fmt.Sprintf("/config/apps/http/servers/srv_%d", port), server)
}

// upsertPath sets the value at path, whether or not it already exists.
// Caddy's admin API in this version treats PUT as create-only (it errors
// "key already exists" on a path that's already set) and PATCH as
// replace-only (it errors if the path doesn't exist yet) -- neither alone
// is a true upsert, so this tries PATCH first (the common case: swapping
// an existing deployment's route) and falls back to PUT only when PATCH
// fails because the path is new. Each individual call is still a single
// atomic config write from Caddy's perspective; this just picks the right
// verb for the two cases Caddy actually distinguishes.
func (c *Client) upsertPath(ctx context.Context, path string, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}

	patchErr := c.doConfigRequest(ctx, http.MethodPatch, path, body)
	if patchErr == nil {
		return nil
	}
	return c.doConfigRequest(ctx, http.MethodPut, path, body)
}

func (c *Client) doConfigRequest(ctx context.Context, method, path string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, method, c.AdminAddr+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, respBody)
	}
	return nil
}

// DeleteRoute tears down a port's server block entirely -- used when a
// deployment is stopped, deleted, or toggled to internal-only.
func (c *Client) DeleteRoute(ctx context.Context, port int) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/config/apps/http/servers/srv_%d", c.AdminAddr, port), nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("DELETE srv_%d: %w", port, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DELETE srv_%d: %s: %s", port, resp.Status, respBody)
	}
	return nil
}
