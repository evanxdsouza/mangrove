// Command mangrove-mcp exposes Mangrove's control-plane API as an MCP
// (Model Context Protocol) server over stdio, so an LLM agent (Claude
// Desktop, Claude Code, or any other MCP client) can inspect and operate
// deployments -- list projects/deployments/services, trigger a redeploy or
// rollback, scale, run a one-off command, pull a log tail -- the same
// actions the dashboard and mangrovectl drive, through the same HTTP API,
// via the shared internal/apiclient package.
//
// Authentication happens once at process startup (MANGROVE_EMAIL/
// MANGROVE_PASSWORD, or a session already saved by `mangrovectl login`
// under ~/.mangrove/session), never through a tool call -- an MCP tool
// argument is something the model constructs and can end up in transcripts
// and logs, which is not where a password belongs.
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

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/evanxdsouza/mangrove/internal/apiclient"
)

func main() {
	log.SetOutput(os.Stderr) // stdout is the MCP JSON-RPC stream -- never write to it directly

	baseURL := os.Getenv("MANGROVE_API_URL")
	if baseURL == "" {
		baseURL = apiclient.DefaultBaseURL
	}
	client := apiclient.New(baseURL)
	client.LoadSession()

	if !client.IsAuthenticated() {
		if email, password := os.Getenv("MANGROVE_EMAIL"), os.Getenv("MANGROVE_PASSWORD"); email != "" && password != "" {
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

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("mangrove-mcp: server exited: %v", err)
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
