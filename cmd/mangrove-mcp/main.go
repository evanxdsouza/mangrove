// Command mangrove-mcp exposes Mangrove's control-plane API as an MCP
// (Model Context Protocol) server, so an LLM agent (Claude Desktop, Claude
// Code, claude.ai, or any other MCP client) can inspect and operate
// deployments -- list projects/deployments/services, trigger a redeploy or
// rollback, scale, run a one-off command, pull a log tail -- the same
// actions the dashboard and mangrovectl drive, through the same HTTP API,
// via the shared internal/apiclient package.
//
// Two transports are supported, selected by MANGROVE_MCP_TRANSPORT:
//
//   - "stdio" (default): a locally-spawned process talking newline-delimited
//     JSON-RPC over stdin/stdout, for Claude Code/Desktop. Authentication
//     happens once at process startup (MANGROVE_EMAIL/MANGROVE_PASSWORD, or
//     a session already saved by `mangrovectl login` under
//     ~/.mangrove/session), never through a tool call -- an MCP tool
//     argument is something the model constructs and can end up in
//     transcripts and logs, which is not where a password belongs.
//
//   - "http": a long-running HTTP server (see http.go) for remote clients
//     like claude.ai's web app, which can't spawn a local stdio process.
//     Auth here is a high-entropy token baked into the URL path itself
//     (MANGROVE_MCP_URL_TOKEN) rather than an OAuth flow -- see http.go's
//     doc comment for why.
//
// Deliberately not exposed here: destructive/setup-shaped actions
// (deleting a project or deployment, creating one from scratch, installing
// a template, managing users or secrets, custom domains). Those stay
// dashboard/mangrovectl-only for now -- this is an operations surface
// (status, deploy, roll back, scale, shell out for diagnostics), not a
// full API mirror, and the ones left out are exactly the ones where a
// model acting on a misread is hardest to undo.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/evanxdsouza/mangrove/internal/apiclient"
)

func main() {
	log.SetOutput(os.Stderr) // stdout is the MCP JSON-RPC stream (stdio mode) -- never write to it directly

	baseURL := os.Getenv("MANGROVE_API_URL")
	if baseURL == "" {
		baseURL = apiclient.DefaultBaseURL
	}
	client := apiclient.New(baseURL)
	client.LoadSession()

	email, password := os.Getenv("MANGROVE_EMAIL"), os.Getenv("MANGROVE_PASSWORD")
	httpMode := os.Getenv("MANGROVE_MCP_TRANSPORT") == "http"

	var urlToken string
	if httpMode {
		urlToken = os.Getenv("MANGROVE_MCP_URL_TOKEN")
		if urlToken == "" {
			log.Fatal("mangrove-mcp: MANGROVE_MCP_TRANSPORT=http requires MANGROVE_MCP_URL_TOKEN -- " +
				"an http-mode server with no token is an open door to redeploy/rollback/run_command")
		}
		if email == "" || password == "" {
			log.Fatal("mangrove-mcp: MANGROVE_MCP_TRANSPORT=http requires MANGROVE_EMAIL/MANGROVE_PASSWORD -- " +
				"a long-running service can't rely on an interactively-created ~/.mangrove/session")
		}
		if _, err := client.Login(context.Background(), email, password); err != nil {
			log.Fatalf("mangrove-mcp: startup login failed: %v", err)
		}
		go keepSessionAlive(client, email, password)
	} else if !client.IsAuthenticated() {
		if email != "" && password != "" {
			if _, err := client.Login(context.Background(), email, password); err != nil {
				log.Printf("mangrove-mcp: startup login failed, tools will report \"not authenticated\" until this is fixed: %v", err)
			}
		} else {
			log.Printf("mangrove-mcp: no session found and MANGROVE_EMAIL/MANGROVE_PASSWORD not set -- " +
				"run `mangrovectl login --email ... --password ...` first, or set those two env vars. " +
				"Starting anyway; every tool call will report \"not authenticated\" until then.")
		}
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "mangrove", Version: "0.1.0"}, nil)
	registerTools(server, client)

	var err error
	if httpMode {
		listenAddr := os.Getenv("MANGROVE_MCP_LISTEN_ADDR")
		if listenAddr == "" {
			listenAddr = "127.0.0.1:7778"
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		err = runHTTP(ctx, server, listenAddr, urlToken)
	} else {
		err = server.Run(context.Background(), &mcp.StdioTransport{})
	}
	if err != nil {
		log.Fatalf("mangrove-mcp: server exited: %v", err)
	}
}

// keepSessionAlive re-authenticates well inside the 30-day session TTL
// (internal/auth.SessionTTL) so a long-running http-mode server never goes
// stale between restarts the way a one-shot stdio invocation never needs to
// worry about.
func keepSessionAlive(client *apiclient.Client, email, password string) {
	const reloginInterval = 24 * time.Hour
	for {
		time.Sleep(reloginInterval)
		if _, err := client.Login(context.Background(), email, password); err != nil {
			log.Printf("mangrove-mcp: periodic re-login failed, will retry in %s: %v", reloginInterval, err)
		}
	}
}

// apiErrorText makes a returned error's text a little more actionable for
// the model when it's specifically an auth failure, without doing
// string-matching on error text elsewhere.
func apiErrorText(err error) error {
	if err == apiclient.ErrNotAuthenticated {
		return fmt.Errorf("not authenticated -- run `mangrovectl login` (or set MANGROVE_EMAIL/MANGROVE_PASSWORD and restart mangrove-mcp)")
	}
	return err
}
