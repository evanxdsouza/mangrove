package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
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
