package executor

import (
	"archive/tar"
	"bytes"
	"context"
	"os/exec"
	"testing"
	"time"
)

// requireDocker skips the test if the Docker daemon isn't reachable —
// these tests exercise the real local Docker daemon, not a mock.
func requireDocker(t *testing.T) {
	t.Helper()
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker daemon not available, skipping integration test")
	}
}

func TestDockerExecutorRunHealthCheckStopRemove(t *testing.T) {
	requireDocker(t)
	ctx := context.Background()

	exec_, err := NewDockerExecutor(ctx, "mangrove-test-net")
	if err != nil {
		t.Fatalf("NewDockerExecutor: %v", err)
	}

	containerName := "mangrove-test-nginx"
	// Best-effort cleanup from a previous failed run.
	exec_.Remove(ctx, containerName)

	spec := RunSpec{
		ImageRef:      "nginx:alpine",
		ContainerName: containerName,
		InternalPort:  80,
		MemoryLimitMB: 128,
		CPULimitCores: 0.5,
		RestartPolicy: "no",
	}

	pullIfNeeded(t, spec.ImageRef)

	result, err := exec_.Run(ctx, spec)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	defer exec_.Remove(ctx, containerName)

	if result.ContainerID == "" {
		t.Fatal("expected non-empty container ID")
	}
	if result.ContainerAddr == "" {
		t.Fatal("expected non-empty container address")
	}

	// Give nginx a moment to start listening.
	var status HealthStatus
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status, err = exec_.HealthCheck(ctx, containerName, HealthCheckSpec{Path: "/", Port: 80, TimeoutSeconds: 2})
		if err == nil && status.Healthy {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !status.Healthy {
		t.Fatalf("expected healthy container, got %+v (err=%v)", status, err)
	}

	stats, err := exec_.Stats(ctx, containerName)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.MemLimitMB <= 0 {
		t.Errorf("expected positive MemLimitMB, got %v", stats.MemLimitMB)
	}

	logs, err := exec_.Logs(ctx, containerName, LogOptions{Tail: "50"})
	if err != nil {
		t.Fatalf("Logs: %v", err)
	}
	logs.Close()

	if err := exec_.Stop(ctx, containerName, 5*time.Second); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := exec_.Remove(ctx, containerName); err != nil {
		t.Fatalf("Remove: %v", err)
	}
}

func TestDockerExecutorBuildDockerfile(t *testing.T) {
	requireDocker(t)
	ctx := context.Background()

	dockerfile := `FROM alpine:latest
CMD ["echo", "hello from mangrove test build"]
`
	tarball := tarOf(t, map[string]string{"Dockerfile": dockerfile})

	e := &DockerExecutor{NetworkName: "mangrove-test-net"}
	var logs bytes.Buffer
	result, err := e.Build(ctx, BuildSpec{
		Strategy: StrategyDockerfile,
		Context:  ContextSource{Tarball: tarball},
		ImageTag: "mangrove-test-build:latest",
	}, &logs)
	if err != nil {
		t.Fatalf("Build: %v, logs: %s", err, logs.String())
	}
	if result.ImageID == "" {
		t.Error("expected non-empty image ID")
	}

	exec.Command("docker", "rmi", "-f", "mangrove-test-build:latest").Run()
}

// tarOf builds an in-memory tar stream from a flat set of file contents,
// standing in for the tarball a caller would hand to Build in place of a
// git URL.
func tarOf(t *testing.T, files map[string]string) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0644, Size: int64(len(content))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("tar header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("tar write: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	return &buf
}

func TestParseSizeToMB(t *testing.T) {
	cases := map[string]float64{
		"256MiB": 256,
		"1GiB":   1024,
		"512KiB": 0.5,
		"1MB":    1,
		"2GB":    2000,
	}
	for input, want := range cases {
		got := parseSizeToMB(input)
		if got != want {
			t.Errorf("parseSizeToMB(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestParseMemUsage(t *testing.T) {
	used, limit := parseMemUsage("12.5MiB / 256MiB")
	if used != 12.5 || limit != 256 {
		t.Errorf("got used=%v limit=%v, want used=12.5 limit=256", used, limit)
	}
}

func pullIfNeeded(t *testing.T, ref string) {
	t.Helper()
	if exec.Command("docker", "image", "inspect", ref).Run() == nil {
		return
	}
	out, err := exec.Command("docker", "pull", ref).CombinedOutput()
	if err != nil {
		t.Skipf("could not pull %s (offline?): %v: %s", ref, err, out)
	}
}
