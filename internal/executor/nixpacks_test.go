package executor

import (
	"bytes"
	"context"
	"os/exec"
	"testing"
)

func requireNixpacks(t *testing.T) {
	t.Helper()
	if err := exec.Command("nixpacks", "--version").Run(); err != nil {
		t.Skip("nixpacks CLI not available, skipping integration test")
	}
}

func TestDockerExecutorBuildNixpacks(t *testing.T) {
	requireDocker(t)
	requireNixpacks(t)
	ctx := context.Background()

	pkgJSON := `{
  "name": "mangrove-nixpacks-test",
  "version": "1.0.0",
  "scripts": { "start": "node index.js" }
}`
	indexJS := `console.log("hello from mangrove nixpacks test build");`

	tarball := tarOf(t, map[string]string{
		"package.json": pkgJSON,
		"index.js":     indexJS,
	})

	e := &DockerExecutor{NetworkName: "mangrove-test-net"}
	var logs bytes.Buffer
	result, err := e.Build(ctx, BuildSpec{
		Strategy: StrategyNixpacks,
		Context:  ContextSource{Tarball: tarball},
		ImageTag: "mangrove-test-nixpacks:latest",
	}, &logs)
	if err != nil {
		t.Fatalf("Build (nixpacks): %v, logs: %s", err, logs.String())
	}
	if result.ImageID == "" {
		t.Error("expected non-empty image ID")
	}

	exec.Command("docker", "rmi", "-f", "mangrove-test-nixpacks:latest").Run()
}
