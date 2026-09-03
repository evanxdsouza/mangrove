// Command mangrove-tui is a terminal dashboard for Mangrove: browse
// projects and deployments, redeploy/restart/stop/scale, roll back to a
// previous deploy, tail live logs, and open a real interactive shell into
// a service's container -- all without leaving the terminal. It drives
// the same HTTP API as the web dashboard and mangrovectl, via the shared
// internal/apiclient package, and shares mangrovectl's session file
// (~/.mangrove/session) so a login from one is visible to the other.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/evanxdsouza/mangrove/internal/apiclient"
)

func main() {
	baseURL := os.Getenv("MANGROVE_API_URL")
	if baseURL == "" {
		baseURL = apiclient.DefaultBaseURL
	}
	client := apiclient.New(baseURL)
	client.LoadSession()

	m := initialModel(client)
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.program.p = p

	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "mangrove-tui:", err)
		os.Exit(1)
	}
}
