package executor

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDockerExecutorBuildStaticNoBuildCommand(t *testing.T) {
	// This path never touches Docker (plain filesystem copy), but is kept
	// under requireDocker/skipped-by-default integration conventions for
	// consistency with the other Build() strategy tests in this package.
	ctx := context.Background()

	tarball := tarOf(t, map[string]string{
		"dist/index.html": "<h1>pre-built site</h1>",
		"dist/style.css":  "body { color: red; }",
		"README.md":       "not part of the output dir",
	})

	e := &DockerExecutor{NetworkName: "mangrove-test-net", StaticSitesDir: t.TempDir()}
	var logs bytes.Buffer
	result, err := e.Build(ctx, BuildSpec{
		Strategy:         StrategyStatic,
		Context:          ContextSource{Tarball: tarball},
		StaticOutputDir:  "dist",
		StaticOutputName: "static-no-build-test",
	}, &logs)
	if err != nil {
		t.Fatalf("Build (static, no build command): %v, logs: %s", err, logs.String())
	}
	if result.OutputPath == "" {
		t.Fatal("expected non-empty OutputPath")
	}

	body, err := os.ReadFile(filepath.Join(result.OutputPath, "index.html"))
	if err != nil {
		t.Fatalf("read copied output: %v", err)
	}
	if string(body) != "<h1>pre-built site</h1>" {
		t.Errorf("got %q, want copied index.html contents", body)
	}
	if _, err := os.Stat(filepath.Join(result.OutputPath, "..", "README.md")); err == nil {
		t.Error("README.md outside the output dir should not have been copied alongside it")
	}
}

func TestDockerExecutorBuildStaticWithBuildCommand(t *testing.T) {
	requireDocker(t)
	requireNixpacks(t)
	ctx := context.Background()

	pkgJSON := `{
  "name": "mangrove-static-build-test",
  "version": "1.0.0",
  "scripts": { "build": "mkdir -p dist && echo '<h1>built by npm run build</h1>' > dist/index.html" }
}`
	tarball := tarOf(t, map[string]string{"package.json": pkgJSON})

	e := &DockerExecutor{NetworkName: "mangrove-test-net", StaticSitesDir: t.TempDir()}
	var logs bytes.Buffer
	result, err := e.Build(ctx, BuildSpec{
		Strategy:           StrategyStatic,
		Context:            ContextSource{Tarball: tarball},
		StaticBuildCommand: "npm run build",
		StaticOutputDir:    "dist",
		StaticOutputName:   "static-build-test",
	}, &logs)
	if err != nil {
		t.Fatalf("Build (static, with build command): %v, logs: %s", err, logs.String())
	}
	if result.OutputPath == "" {
		t.Fatal("expected non-empty OutputPath")
	}

	body, err := os.ReadFile(filepath.Join(result.OutputPath, "index.html"))
	if err != nil {
		t.Fatalf("read build output: %v", err)
	}
	if string(body) != "<h1>built by npm run build</h1>\n" {
		t.Errorf("got %q, want the npm build script's output", body)
	}

	// The builder image is a throwaway -- unlike app images, nothing keeps
	// it around for rollback, and disk is scarce on a 16GB box.
	out, err := exec.Command("docker", "images", "-q", "mangrove-static-build-static-build-test").Output()
	if err != nil {
		t.Fatalf("docker images: %v", err)
	}
	if len(bytes.TrimSpace(out)) != 0 {
		t.Errorf("expected builder image to be removed after build, docker images returned: %s", out)
	}
}
