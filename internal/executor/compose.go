package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ComposeExecutor handles multi-service stacks. Compose's dependency
// ordering and inter-service networking semantics are intentionally not
// reimplemented against the single-service Executor interface — this
// shells out to `docker compose` directly and reconciles resulting
// container state back into Mangrove's `services` rows.
type ComposeExecutor struct {
	NetworkName string
}

type ComposeSpec struct {
	Context     ContextSource
	RootPath    string
	ComposePath string // relative to RootPath, e.g. "docker-compose.yml"
	ProjectName string // becomes the `docker compose -p` project name
	Env         map[string]string
}

type ComposeServiceResult struct {
	ServiceName   string
	ContainerID   string
	ContainerName string
	State         string // running|exited|...
	Publishers    []ComposePublisher
}

type ComposePublisher struct {
	PublishedPort int
	TargetPort    int
}

// Up materializes the compose spec's source, then runs `docker compose up
// -d --build`, streaming output to logs, and returns the resulting
// per-service container state.
func (c *ComposeExecutor) Up(ctx context.Context, spec ComposeSpec, logs io.Writer) ([]ComposeServiceResult, error) {
	dir, cleanup, err := materialize(ctx, spec.Context)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	workDir := dir
	if spec.RootPath != "" && spec.RootPath != "." {
		workDir = filepath.Join(dir, spec.RootPath)
	}
	composeFile := spec.ComposePath
	if composeFile == "" {
		composeFile = "docker-compose.yml"
	}

	args := []string{"compose", "-p", spec.ProjectName, "-f", composeFile, "up", "-d", "--build"}
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = workDir
	cmd.Stdout = logs
	cmd.Stderr = logs
	cmd.Env = envWith(spec.Env)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("docker compose up: %w", err)
	}

	return c.ps(ctx, workDir, spec.ProjectName, composeFile)
}

// Down stops and removes every container in the compose project.
func (c *ComposeExecutor) Down(ctx context.Context, projectName string) error {
	out, err := exec.CommandContext(ctx, "docker", "compose", "-p", projectName, "down").CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose down: %w: %s", err, out)
	}
	return nil
}

func (c *ComposeExecutor) ps(ctx context.Context, workDir, projectName, composeFile string) ([]ComposeServiceResult, error) {
	cmd := exec.CommandContext(ctx, "docker", "compose", "-p", projectName, "-f", composeFile, "ps", "--format", "json")
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker compose ps: %w: %s", err, out)
	}
	return parseComposePS(out)
}

// composePSEntry matches the relevant subset of `docker compose ps
// --format json` output. Compose v2 emits either one JSON object per line
// or a single JSON array depending on version, so parseComposePS handles both.
type composePSEntry struct {
	Service    string `json:"Service"`
	Name       string `json:"Name"`
	ID         string `json:"ID"`
	State      string `json:"State"`
	Publishers []struct {
		PublishedPort int `json:"PublishedPort"`
		TargetPort    int `json:"TargetPort"`
	} `json:"Publishers"`
}

func parseComposePS(out []byte) ([]ComposeServiceResult, error) {
	var entries []composePSEntry

	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return nil, nil
	}
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &entries); err != nil {
			return nil, fmt.Errorf("parse compose ps array: %w", err)
		}
	} else {
		for _, line := range strings.Split(string(trimmed), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var e composePSEntry
			if err := json.Unmarshal([]byte(line), &e); err != nil {
				return nil, fmt.Errorf("parse compose ps line %q: %w", line, err)
			}
			entries = append(entries, e)
		}
	}

	results := make([]ComposeServiceResult, 0, len(entries))
	for _, e := range entries {
		r := ComposeServiceResult{
			ServiceName:   e.Service,
			ContainerID:   e.ID,
			ContainerName: e.Name,
			State:         e.State,
		}
		for _, p := range e.Publishers {
			if p.PublishedPort > 0 {
				r.Publishers = append(r.Publishers, ComposePublisher{PublishedPort: p.PublishedPort, TargetPort: p.TargetPort})
			}
		}
		results = append(results, r)
	}
	return results, nil
}

func envWith(extra map[string]string) []string {
	env := os.Environ()
	for k, v := range extra {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}
