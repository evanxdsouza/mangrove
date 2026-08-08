package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func requireCaddy(t *testing.T) *Client {
	t.Helper()
	c := NewClient("")
	if err := c.EnsureBaseConfig(context.Background()); err != nil {
		t.Skipf("caddy admin API not reachable, skipping integration test: %v", err)
	}
	return c
}

// startBackend runs a trivial HTTP server on an OS-assigned loopback port,
// standing in for a container's internal address.
func startBackend(t *testing.T, body string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	})}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
	return ln.Addr().String()
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func TestPutRouteProxiesToBackend(t *testing.T) {
	c := requireCaddy(t)
	ctx := context.Background()

	backend := startBackend(t, "hello from mangrove test backend")
	port := freePort(t)

	if err := c.PutRoute(ctx, port, backend, RouteOptions{}); err != nil {
		t.Fatalf("PutRoute: %v", err)
	}
	t.Cleanup(func() { c.DeleteRoute(ctx, port) })

	body := getWithRetry(t, fmt.Sprintf("http://127.0.0.1:%d/", port), 200)
	if body != "hello from mangrove test backend" {
		t.Errorf("got body %q, want backend response", body)
	}
}

func TestPutRouteReplacesUpstreamAtomically(t *testing.T) {
	c := requireCaddy(t)
	ctx := context.Background()

	backend1 := startBackend(t, "backend one")
	backend2 := startBackend(t, "backend two")
	port := freePort(t)

	if err := c.PutRoute(ctx, port, backend1, RouteOptions{}); err != nil {
		t.Fatalf("PutRoute (backend1): %v", err)
	}
	t.Cleanup(func() { c.DeleteRoute(ctx, port) })

	if got := getWithRetry(t, fmt.Sprintf("http://127.0.0.1:%d/", port), 200); got != "backend one" {
		t.Fatalf("expected backend one, got %q", got)
	}

	// This is the swap operation: repoint the same server block at a new
	// upstream, simulating what orchestrator.Deploy does after a new
	// container passes its health check.
	if err := c.PutRoute(ctx, port, backend2, RouteOptions{}); err != nil {
		t.Fatalf("PutRoute (backend2): %v", err)
	}

	if got := getWithRetry(t, fmt.Sprintf("http://127.0.0.1:%d/", port), 200); got != "backend two" {
		t.Fatalf("expected backend two after swap, got %q", got)
	}
}

func TestDeleteRouteRemovesServer(t *testing.T) {
	c := requireCaddy(t)
	ctx := context.Background()

	backend := startBackend(t, "will be removed")
	port := freePort(t)

	if err := c.PutRoute(ctx, port, backend, RouteOptions{}); err != nil {
		t.Fatalf("PutRoute: %v", err)
	}
	getWithRetry(t, fmt.Sprintf("http://127.0.0.1:%d/", port), 200)

	if err := c.DeleteRoute(ctx, port); err != nil {
		t.Fatalf("DeleteRoute: %v", err)
	}

	time.Sleep(300 * time.Millisecond)
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
	if err == nil {
		resp.Body.Close()
		t.Errorf("expected connection to fail after DeleteRoute, got HTTP %d", resp.StatusCode)
	}
}

func TestPutRouteWithBasicAuthRequiresCredentials(t *testing.T) {
	c := requireCaddy(t)
	ctx := context.Background()

	backend := startBackend(t, "secret content")
	port := freePort(t)

	// bcrypt hash of "testpassword123", generated once for this fixture.
	const bcryptHash = "$2a$10$IAn/zt/YT8JBvFZORD82w.VzaDnqxPcOnMhE8pXoiW9J5IhqDw1ga"

	if err := c.PutRoute(ctx, port, backend, RouteOptions{
		PasswordProtected: true,
		Username:          "mangrove",
		BcryptHash:        bcryptHash,
	}); err != nil {
		t.Fatalf("PutRoute: %v", err)
	}
	t.Cleanup(func() { c.DeleteRoute(ctx, port) })

	deadline := time.Now().Add(5 * time.Second)
	var status int
	for time.Now().Before(deadline) {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
		if err == nil {
			status = resp.StatusCode
			resp.Body.Close()
			if status != 0 {
				break
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	if status != http.StatusUnauthorized {
		t.Errorf("expected 401 without credentials, got %d", status)
	}
}

// staticSitesTestRoot mirrors config.Config.StaticSitesDir's production
// default (see deploy/systemd/mangrove.service): a world-readable directory
// outside of both /tmp and any 0700 data directory. Used when present so
// this test exercises the same permission shape a real deploy needs; falls
// back to a plain temp dir on hosts without it (e.g. CI), where the
// unsandboxed default Caddy setup has no such restriction to exercise.
const staticSitesTestRoot = "/var/lib/mangrove-static"

// nonTmpDir returns a fresh, world-readable directory outside of /tmp,
// cleaned up on test exit. Static-site output directories can't live under
// /tmp or a 0700 data dir: Caddy's systemd unit runs with PrivateTmp=true
// (its own private /tmp, blind to the host's) and as its own unprivileged
// user (blind to anything not explicitly opened up to it) -- a real deploy
// serving from either would 404/403 exactly like this test does if
// misconfigured that way.
func nonTmpDir(t *testing.T) string {
	t.Helper()
	base := "."
	if info, err := os.Stat(staticSitesTestRoot); err == nil && info.IsDir() {
		base = staticSitesTestRoot
	}
	dir, err := os.MkdirTemp(base, "caddytest-")
	if err != nil {
		t.Fatalf("create fixture dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatalf("chmod fixture dir: %v", err)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("resolve fixture dir: %v", err)
	}
	return abs
}

func TestPutFileServerRouteServesDirectory(t *testing.T) {
	c := requireCaddy(t)
	ctx := context.Background()

	dir := nonTmpDir(t)
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("hello from static site"), 0644); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}
	port := freePort(t)

	if err := c.PutFileServerRoute(ctx, port, dir, RouteOptions{}); err != nil {
		t.Fatalf("PutFileServerRoute: %v", err)
	}
	t.Cleanup(func() { c.DeleteRoute(ctx, port) })

	body := getWithRetry(t, fmt.Sprintf("http://127.0.0.1:%d/", port), 200)
	if body != "hello from static site" {
		t.Errorf("got body %q, want static file contents", body)
	}
}

func TestPutFileServerRouteSwapsRootAtomically(t *testing.T) {
	c := requireCaddy(t)
	ctx := context.Background()

	dir1, dir2 := nonTmpDir(t), nonTmpDir(t)
	os.WriteFile(filepath.Join(dir1, "index.html"), []byte("output v1"), 0644)
	os.WriteFile(filepath.Join(dir2, "index.html"), []byte("output v2"), 0644)
	port := freePort(t)

	if err := c.PutFileServerRoute(ctx, port, dir1, RouteOptions{}); err != nil {
		t.Fatalf("PutFileServerRoute (dir1): %v", err)
	}
	t.Cleanup(func() { c.DeleteRoute(ctx, port) })
	if got := getWithRetry(t, fmt.Sprintf("http://127.0.0.1:%d/", port), 200); got != "output v1" {
		t.Fatalf("expected output v1, got %q", got)
	}

	// This is what a static-strategy rollback does: repoint the same server
	// block's root at a previous deploy's output directory, no rebuild.
	if err := c.PutFileServerRoute(ctx, port, dir2, RouteOptions{}); err != nil {
		t.Fatalf("PutFileServerRoute (dir2): %v", err)
	}
	if got := getWithRetry(t, fmt.Sprintf("http://127.0.0.1:%d/", port), 200); got != "output v2" {
		t.Fatalf("expected output v2 after swap, got %q", got)
	}
}

func getWithRetry(t *testing.T, url string, wantStatus int) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err != nil {
			lastErr = err
			time.Sleep(200 * time.Millisecond)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != wantStatus {
			lastErr = fmt.Errorf("got status %d, want %d", resp.StatusCode, wantStatus)
			time.Sleep(200 * time.Millisecond)
			continue
		}
		return string(body)
	}
	t.Fatalf("GET %s never succeeded: %v", url, lastErr)
	return ""
}
