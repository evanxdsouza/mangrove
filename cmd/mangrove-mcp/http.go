package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// runHTTP serves server over Streamable HTTP (the transport a remote client
// like claude.ai's web app uses, since it can't spawn a local stdio
// process) on listenAddr, gated by token.
//
// The gate is deliberately not OAuth. claude.ai's remote-connector client
// only starts an OAuth 2.1 + dynamic-client-registration dance if the
// server ever answers with a 401 and a WWW-Authenticate challenge; if it
// never does, the client just connects. So instead of standing up a real
// OAuth authorization server (a new class of attack surface -- token/code
// endpoints, redirect_uri validation, dynamic client registration -- for a
// tool only one person will ever use), the auth secret lives in the URL
// path itself: only /mcp/<token> is live, and a wrong or missing token
// gets a plain 404, indistinguishable from the path not existing at all.
// Rotating the token means editing MANGROVE_MCP_URL_TOKEN and restarting
// this service, then updating the connector's URL in claude.ai -- there's
// no per-request revocation, which is an acceptable tradeoff for a single
// shared secret guarding a single owner's own box.
func runHTTP(ctx context.Context, server *mcp.Server, listenAddr, token string) error {
	// The SDK's DNS-rebinding guard 403s any loopback-arriving request with a
	// non-localhost Host header, which is every request behind our reverse
	// proxy. The URL token is the gate here, so an attacker rebinding DNS
	// would still need it.
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{DisableLocalhostProtection: true})

	mux := http.NewServeMux()
	mux.HandleFunc("/mcp/{token}", func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.PathValue("token")), []byte(token)) != 1 {
			http.NotFound(w, r)
			return
		}
		mcpHandler.ServeHTTP(w, r)
	})

	srv := &http.Server{Addr: listenAddr, Handler: mux}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("mangrove-mcp: listening on %s (http transport)", listenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Printf("mangrove-mcp: shutting down")
	case err := <-errCh:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return nil
}
