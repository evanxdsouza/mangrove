// Command mangrovectl is a thin HTTP client for Mangrove's local API --
// the Phase 1 CLI, driving the exact same endpoints the dashboard will.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	baseURL := os.Getenv("MANGROVE_API_URL")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:7777"
	}
	c := &client{baseURL: baseURL, sessionPath: sessionFilePath()}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "setup":
		err = setupCmd(c, args)
	case "login":
		err = loginCmd(c, args)
	case "logout":
		err = logoutCmd(c, args)
	case "project":
		err = projectCmd(c, args)
	case "deployment":
		err = deploymentCmd(c, args)
	case "deploy":
		err = deployCmd(c, args)
	case "history":
		err = historyCmd(c, args)
	case "rollback":
		err = rollbackCmd(c, args)
	case "services":
		err = servicesCmd(c, args)
	default:
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `mangrovectl <command> [flags]

Commands:
  setup --email EMAIL --password PASSWORD   (first-run admin account creation)
  login --email EMAIL --password PASSWORD
  logout
  project create --name NAME --slug SLUG [--description TEXT]
  project list
  deployment create --project SLUG --name NAME --slug SLUG --strategy dockerfile|nixpacks|compose|image
                     [--git-branch BRANCH] [--image-ref REF] [--root-path PATH]
                     [--dockerfile-path PATH] [--compose-path PATH]
                     [--service-name NAME] [--internal-port PORT] [--internal-only]
                     [--cpu CORES] [--memory MB] [--health-path PATH]
  deployment list --project SLUG
  deploy --deployment ID [--git-url URL] [--git-ref REF] [--commit-sha SHA] [--commit-message MSG]
  history --deployment ID
  rollback --history-id ID
  services --deployment ID`)
}

// ---- HTTP client ----

// sessionFilePath returns where the CLI persists its session cookie
// between invocations -- each `mangrovectl` call is a fresh process, so
// login state has to live on disk, not in memory.
func sessionFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".mangrove", "session")
}

type client struct {
	baseURL     string
	sessionPath string
}

func (c *client) do(method, path string, body any, out any) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token, err := os.ReadFile(c.sessionPath); err == nil {
		req.AddCookie(&http.Cookie{Name: "mangrove_session", Value: string(token)})
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to mangrove API failed (is `mangrove` running?): %w", err)
	}
	defer resp.Body.Close()

	for _, ck := range resp.Cookies() {
		if ck.Name == "mangrove_session" {
			c.saveSession(ck.Value)
		}
	}

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("not authenticated -- run `mangrovectl login` (or `mangrovectl setup` on a fresh install) first")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, string(respBody))
	}
	if out != nil && len(respBody) > 0 {
		return json.Unmarshal(respBody, out)
	}
	return nil
}

func (c *client) saveSession(token string) {
	if token == "" {
		os.Remove(c.sessionPath)
		return
	}
	os.MkdirAll(filepath.Dir(c.sessionPath), 0700)
	os.WriteFile(c.sessionPath, []byte(token), 0600)
}

// ---- setup / login / logout ----

func setupCmd(c *client, args []string) error {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	email := fs.String("email", "", "admin email")
	password := fs.String("password", "", "admin password (min 8 characters)")
	fs.Parse(args)

	var out map[string]any
	if err := c.do("POST", "/api/auth/setup", map[string]string{"email": *email, "password": *password}, &out); err != nil {
		return err
	}
	fmt.Println("admin account created and logged in as", out["email"])
	return nil
}

func loginCmd(c *client, args []string) error {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	email := fs.String("email", "", "email")
	password := fs.String("password", "", "password")
	fs.Parse(args)

	var out map[string]any
	if err := c.do("POST", "/api/auth/login", map[string]string{"email": *email, "password": *password}, &out); err != nil {
		return err
	}
	fmt.Println("logged in as", out["email"])
	return nil
}

func logoutCmd(c *client, args []string) error {
	if err := c.do("POST", "/api/auth/logout", nil, nil); err != nil {
		return err
	}
	c.saveSession("")
	fmt.Println("logged out")
	return nil
}

// ---- project ----

func projectCmd(c *client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("expected a subcommand: create|list")
	}
	switch args[0] {
	case "create":
		fs := flag.NewFlagSet("project create", flag.ExitOnError)
		name := fs.String("name", "", "project name")
		slug := fs.String("slug", "", "project slug")
		desc := fs.String("description", "", "project description")
		fs.Parse(args[1:])

		var out map[string]any
		if err := c.do("POST", "/api/projects", map[string]string{"name": *name, "slug": *slug, "description": *desc}, &out); err != nil {
			return err
		}
		return printJSON(out)
	case "list":
		var out []map[string]any
		if err := c.do("GET", "/api/projects", nil, &out); err != nil {
			return err
		}
		return printJSON(out)
	default:
		return fmt.Errorf("unknown project subcommand %q", args[0])
	}
}

func resolveProjectID(c *client, slug string) (float64, error) {
	var projects []map[string]any
	if err := c.do("GET", "/api/projects", nil, &projects); err != nil {
		return 0, err
	}
	for _, p := range projects {
		if p["slug"] == slug {
			return p["id"].(float64), nil
		}
	}
	return 0, fmt.Errorf("no project with slug %q", slug)
}

// ---- deployment ----

func deploymentCmd(c *client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("expected a subcommand: create|list")
	}
	switch args[0] {
	case "create":
		fs := flag.NewFlagSet("deployment create", flag.ExitOnError)
		project := fs.String("project", "", "project slug")
		name := fs.String("name", "", "deployment name")
		slug := fs.String("slug", "", "deployment slug")
		strategy := fs.String("strategy", "", "dockerfile|nixpacks|compose|image")
		gitBranch := fs.String("git-branch", "", "branch to deploy")
		imageRef := fs.String("image-ref", "", "image ref (strategy=image)")
		rootPath := fs.String("root-path", ".", "subdirectory to build from")
		dockerfilePath := fs.String("dockerfile-path", "", "Dockerfile path (strategy=dockerfile)")
		composePath := fs.String("compose-path", "", "compose file path (strategy=compose)")
		serviceName := fs.String("service-name", "web", "primary service name")
		internalPort := fs.Int("internal-port", 0, "port the app listens on inside the container")
		internalOnly := fs.Bool("internal-only", false, "never expose a public port")
		cpu := fs.Float64("cpu", 0.5, "CPU cores limit")
		memory := fs.Int("memory", 256, "memory limit in MB")
		healthPath := fs.String("health-path", "", "HTTP health check path")
		fs.Parse(args[1:])

		projectID, err := resolveProjectID(c, *project)
		if err != nil {
			return err
		}

		payload := map[string]any{
			"name":                  *name,
			"slug":                  *slug,
			"build_strategy":        *strategy,
			"git_branch":            *gitBranch,
			"image_ref":             *imageRef,
			"root_path":             *rootPath,
			"dockerfile_path":       *dockerfilePath,
			"compose_path":          *composePath,
			"image_retention_count": 5,
		}
		if *strategy != "compose" {
			payload["service"] = map[string]any{
				"name":              *serviceName,
				"internal_port":     *internalPort,
				"is_internal_only":  *internalOnly,
				"cpu_limit_cores":   *cpu,
				"memory_limit_mb":   *memory,
				"health_check_path": *healthPath,
			}
		}

		var out map[string]any
		if err := c.do("POST", fmt.Sprintf("/api/projects/%d/deployments", int64(projectID)), payload, &out); err != nil {
			return err
		}
		return printJSON(out)
	case "list":
		fs := flag.NewFlagSet("deployment list", flag.ExitOnError)
		project := fs.String("project", "", "project slug")
		fs.Parse(args[1:])

		projectID, err := resolveProjectID(c, *project)
		if err != nil {
			return err
		}
		var out []map[string]any
		if err := c.do("GET", fmt.Sprintf("/api/projects/%d/deployments", int64(projectID)), nil, &out); err != nil {
			return err
		}
		return printJSON(out)
	default:
		return fmt.Errorf("unknown deployment subcommand %q", args[0])
	}
}

// ---- deploy / history / rollback / services ----

func deployCmd(c *client, args []string) error {
	fs := flag.NewFlagSet("deploy", flag.ExitOnError)
	deploymentID := fs.String("deployment", "", "deployment id")
	gitURL := fs.String("git-url", "", "git repo URL")
	gitRef := fs.String("git-ref", "", "git ref (branch/tag/sha)")
	commitSHA := fs.String("commit-sha", "", "commit SHA")
	commitMessage := fs.String("commit-message", "", "commit message")
	fs.Parse(args)
	if *deploymentID == "" {
		return fmt.Errorf("--deployment is required")
	}

	var out map[string]any
	err := c.do("POST", "/api/deployments/"+*deploymentID+"/deploy", map[string]string{
		"git_url": *gitURL, "git_ref": *gitRef, "commit_sha": *commitSHA, "commit_message": *commitMessage,
	}, &out)
	printJSON(out)
	return err
}

func historyCmd(c *client, args []string) error {
	fs := flag.NewFlagSet("history", flag.ExitOnError)
	deploymentID := fs.String("deployment", "", "deployment id")
	fs.Parse(args)
	if *deploymentID == "" {
		return fmt.Errorf("--deployment is required")
	}
	var out []map[string]any
	if err := c.do("GET", "/api/deployments/"+*deploymentID+"/history", nil, &out); err != nil {
		return err
	}
	return printJSON(out)
}

func rollbackCmd(c *client, args []string) error {
	fs := flag.NewFlagSet("rollback", flag.ExitOnError)
	historyID := fs.String("history-id", "", "deploy_history id to roll back to")
	fs.Parse(args)
	if *historyID == "" {
		return fmt.Errorf("--history-id is required")
	}
	var out map[string]any
	err := c.do("POST", "/api/deploy-history/"+*historyID+"/rollback", nil, &out)
	printJSON(out)
	return err
}

func servicesCmd(c *client, args []string) error {
	fs := flag.NewFlagSet("services", flag.ExitOnError)
	deploymentID := fs.String("deployment", "", "deployment id")
	fs.Parse(args)
	if *deploymentID == "" {
		return fmt.Errorf("--deployment is required")
	}
	var out []map[string]any
	if err := c.do("GET", "/api/deployments/"+*deploymentID+"/services", nil, &out); err != nil {
		return err
	}
	return printJSON(out)
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
