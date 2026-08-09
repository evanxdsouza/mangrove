package executor

import (
	"archive/tar"
	"bytes"
	"context"
	"os/exec"
	"strings"
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

	exec_, err := NewDockerExecutor(ctx, "mangrove-test-net", t.TempDir())
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

// TestDockerExecutorRunPullsMissingImage guards a real bug: when an image
// isn't cached locally, `docker run` pulls it itself and interleaves
// multi-line pull-progress output ahead of the final container-ID line on
// its combined stdout/stderr. Run must pull explicitly first so its own
// container-ID parsing never has to sort that noise out -- otherwise
// containerID ends up as the whole blob, which then fails every
// `docker inspect` call downstream (surfacing as "no such object").
func TestDockerExecutorRunPullsMissingImage(t *testing.T) {
	requireDocker(t)
	ctx := context.Background()

	exec_, err := NewDockerExecutor(ctx, "mangrove-test-net", t.TempDir())
	if err != nil {
		t.Fatalf("NewDockerExecutor: %v", err)
	}

	const image = "nginx:alpine"
	// Force the image to be absent locally so Run has to pull it itself,
	// reproducing the exact condition the bug depended on.
	if exec.Command("docker", "image", "inspect", image).Run() == nil {
		if out, err := exec.Command("docker", "rmi", image).CombinedOutput(); err != nil {
			t.Skipf("could not remove %s to set up test: %v: %s", image, err, out)
		}
	}

	containerName := "mangrove-test-pull-missing"
	exec_.Remove(ctx, containerName)

	result, err := exec_.Run(ctx, RunSpec{
		ImageRef:      image,
		ContainerName: containerName,
		InternalPort:  80,
		MemoryLimitMB: 128,
		CPULimitCores: 0.5,
		RestartPolicy: "no",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	defer exec_.Remove(ctx, containerName)

	if strings.Contains(result.ContainerID, "\n") || len(result.ContainerID) < 12 {
		t.Fatalf("expected a bare container ID, got %q", result.ContainerID)
	}
	if result.ContainerAddr == "" {
		t.Fatal("expected non-empty container address")
	}
}

// TestHealthCheckDoesNotFollowRedirectToUnreachableHost guards a real bug
// found while building the templates feature: Ghost (and plenty of other
// apps) redirects a plain HTTP request to its own configured public
// https:// hostname, which doesn't resolve from inside the container
// network. The default http.Client follows redirects transparently, so
// HealthCheck would chase that redirect, fail to connect, and report the
// container unhealthy even though it's actually fine -- exactly what
// resp.StatusCode >= 200 && resp.StatusCode < 400 (a bare 3xx counts as
// healthy) was already meant to allow for, just not enforced.
func TestHealthCheckDoesNotFollowRedirectToUnreachableHost(t *testing.T) {
	requireDocker(t)
	ctx := context.Background()

	exec_, err := NewDockerExecutor(ctx, "mangrove-test-net", t.TempDir())
	if err != nil {
		t.Fatalf("NewDockerExecutor: %v", err)
	}

	containerName := "mangrove-test-redirect"
	exec_.Remove(ctx, containerName)
	pullIfNeeded(t, "alpine:latest")

	spec := RunSpec{
		ImageRef:      "alpine:latest",
		ContainerName: containerName,
		InternalPort:  8080,
		MemoryLimitMB: 64,
		RestartPolicy: "no",
		// A minimal fake HTTP server that always redirects to a hostname
		// that cannot resolve -- standing in for Ghost's "url" config.
		Command: []string{"sh", "-c",
			`while true; do printf 'HTTP/1.1 302 Found\r\nLocation: https://nonexistent.invalid.test.example/\r\nContent-Length: 0\r\nConnection: close\r\n\r\n' | nc -l -p 8080; done`,
		},
	}
	if _, err := exec_.Run(ctx, spec); err != nil {
		t.Fatalf("Run: %v", err)
	}
	defer exec_.Remove(ctx, containerName)

	var status HealthStatus
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status, err = exec_.HealthCheck(ctx, containerName, HealthCheckSpec{Path: "/", Port: 8080, TimeoutSeconds: 3})
		if err == nil && (status.Healthy || status.StatusCode != 0) {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("HealthCheck error: %v", err)
	}
	if !status.Healthy {
		t.Errorf("expected the bare 302 to count as healthy (not chased), got %+v", status)
	}
	if status.StatusCode != 302 {
		t.Errorf("expected StatusCode 302, got %d", status.StatusCode)
	}
	if status.ResponseTimeMS > 2000 {
		t.Errorf("expected a fast response (no redirect chase attempted), took %dms", status.ResponseTimeMS)
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
