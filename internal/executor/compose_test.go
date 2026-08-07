package executor

import (
	"bytes"
	"context"
	"os/exec"
	"testing"
)

func requireComposePlugin(t *testing.T) {
	t.Helper()
	if err := exec.Command("docker", "compose", "version").Run(); err != nil {
		t.Skip("docker compose plugin not available, skipping integration test")
	}
}

func TestComposeExecutorUpDown(t *testing.T) {
	requireDocker(t)
	requireComposePlugin(t)
	ctx := context.Background()

	pullIfNeeded(t, "nginx:alpine")
	pullIfNeeded(t, "alpine:latest")

	composeYML := `services:
  web:
    image: nginx:alpine
  worker:
    image: alpine:latest
    command: sleep 3600
`
	tarball := tarOf(t, map[string]string{"docker-compose.yml": composeYML})

	c := &ComposeExecutor{NetworkName: "mangrove-test-net"}
	projectName := "mangrove-test-compose"

	// Best-effort cleanup from a previous failed run.
	c.Down(ctx, projectName)

	var logs bytes.Buffer
	results, err := c.Up(ctx, ComposeSpec{
		Context:     ContextSource{Tarball: tarball},
		ComposePath: "docker-compose.yml",
		ProjectName: projectName,
	}, &logs)
	if err != nil {
		t.Fatalf("Up: %v, logs: %s", err, logs.String())
	}
	defer c.Down(ctx, projectName)

	if len(results) != 2 {
		t.Fatalf("expected 2 services reconciled from `compose ps`, got %d: %+v", len(results), results)
	}

	byName := map[string]ComposeServiceResult{}
	for _, r := range results {
		byName[r.ServiceName] = r
	}
	for _, want := range []string{"web", "worker"} {
		got, ok := byName[want]
		if !ok {
			t.Errorf("expected a result for service %q, got services: %v", want, byName)
			continue
		}
		if got.ContainerID == "" {
			t.Errorf("service %q: expected non-empty container ID", want)
		}
	}

	if err := c.Down(ctx, projectName); err != nil {
		t.Fatalf("Down: %v", err)
	}
}
