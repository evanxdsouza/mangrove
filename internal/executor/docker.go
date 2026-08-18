package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// DockerExecutor drives the local Docker daemon via the `docker` CLI.
// Shelling out (rather than the Docker Go SDK) keeps the dependency graph
// and binary size small on a disk-constrained box, at the cost of parsing
// CLI output instead of typed API responses — an acceptable trade for a
// single-node v1 whose real complexity lives in orchestration, not in the
// Docker client itself.
type DockerExecutor struct {
	// NetworkName is the private bridge every managed container joins.
	NetworkName string
	// StaticSitesDir is the host directory static-strategy builds are
	// copied into, one subdirectory per BuildSpec.StaticOutputName. Caddy's
	// file_server reads directly from here — see proxy.PutFileServerRoute.
	StaticSitesDir string
}

// NewDockerExecutor returns an executor bound to the given Docker network,
// creating the network if it does not already exist.
func NewDockerExecutor(ctx context.Context, networkName, staticSitesDir string) (*DockerExecutor, error) {
	e := &DockerExecutor{NetworkName: networkName, StaticSitesDir: staticSitesDir}
	if err := e.ensureNetwork(ctx); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *DockerExecutor) ensureNetwork(ctx context.Context) error {
	check := exec.CommandContext(ctx, "docker", "network", "inspect", e.NetworkName)
	if err := check.Run(); err == nil {
		return nil
	}
	create := exec.CommandContext(ctx, "docker", "network", "create", e.NetworkName)
	out, err := create.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create network %s: %w: %s", e.NetworkName, err, out)
	}
	return nil
}

func (e *DockerExecutor) Build(ctx context.Context, spec BuildSpec, logs io.Writer) (BuildResult, error) {
	if spec.Strategy == StrategyImage {
		return BuildResult{ImageTag: spec.ImageRef}, nil
	}

	dir, cleanup, err := materialize(ctx, spec.Context)
	if err != nil {
		return BuildResult{}, err
	}
	defer cleanup()

	buildDir := dir
	if spec.RootPath != "" && spec.RootPath != "." {
		buildDir = filepath.Join(dir, spec.RootPath)
	}

	switch spec.Strategy {
	case StrategyDockerfile:
		return e.buildDockerfile(ctx, buildDir, spec, logs)
	case StrategyNixpacks:
		return e.buildNixpacks(ctx, buildDir, spec, logs)
	case StrategyStatic:
		return e.buildStatic(ctx, buildDir, spec, logs)
	default:
		return BuildResult{}, fmt.Errorf("unsupported build strategy for Build(): %s (compose stacks use BuildCompose)", spec.Strategy)
	}
}

// buildStatic produces a static-strategy deploy's output directory under
// e.StaticSitesDir. With no build command, it's a plain filesystem copy of
// StaticOutputDir out of the already-fetched context (the "pre-built
// HTML/CSS/JS" case). With a build command, a nixpacks-provisioned builder
// image runs it, the built directory is `docker cp`'d out, and the builder
// image is removed immediately afterward -- unlike app images, it's never
// needed again (no rollback path re-runs a build), and disk is scarce
// enough on a 16GB box that it isn't worth keeping around even briefly.
func (e *DockerExecutor) buildStatic(ctx context.Context, buildDir string, spec BuildSpec, logs io.Writer) (BuildResult, error) {
	if e.StaticSitesDir == "" {
		return BuildResult{}, fmt.Errorf("static build: executor has no StaticSitesDir configured")
	}
	if spec.StaticOutputName == "" {
		return BuildResult{}, fmt.Errorf("static build: BuildSpec.StaticOutputName is required")
	}
	destDir := filepath.Join(e.StaticSitesDir, spec.StaticOutputName)
	if err := os.RemoveAll(destDir); err != nil {
		return BuildResult{}, fmt.Errorf("clear static output dir: %w", err)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return BuildResult{}, fmt.Errorf("create static output dir: %w", err)
	}

	if spec.StaticBuildCommand == "" {
		srcDir := buildDir
		if spec.StaticOutputDir != "" && spec.StaticOutputDir != "." {
			srcDir = filepath.Join(buildDir, spec.StaticOutputDir)
		}
		out, err := exec.CommandContext(ctx, "cp", "-a", srcDir+"/.", destDir).CombinedOutput()
		if err != nil {
			return BuildResult{}, fmt.Errorf("copy static output: %w: %s", err, out)
		}
		return BuildResult{OutputPath: destDir}, nil
	}

	builderImage := "mangrove-static-build-" + spec.StaticOutputName
	nixArgs := []string{
		"build", buildDir, "--name", builderImage,
		"--env", "NIXPACKS_BUILD_CMD=" + spec.StaticBuildCommand,
		// The static build never runs the image's start phase -- only its
		// build output is extracted -- but nixpacks still wants a start
		// command to complete plan detection for repos with no obvious one
		// (e.g. plain static HTML with no package.json).
		"--env", "NIXPACKS_START_CMD=true",
	}
	// Same EOL-Node-18 pin buildNixpacks applies below -- this is a
	// separate nixpacks invocation (the static build's own builder image),
	// so it needs its own copy of the fix, not just the same constant.
	nodeVersion, explicit := spec.BuildArgs["NIXPACKS_NODE_VERSION"]
	if !explicit {
		nodeVersion = defaultNixpacksNodeVersion
	}
	nixArgs = append(nixArgs, "--env", "NIXPACKS_NODE_VERSION="+nodeVersion)
	for k, v := range spec.BuildArgs {
		if k == "NIXPACKS_NODE_VERSION" {
			continue
		}
		nixArgs = append(nixArgs, "--env", fmt.Sprintf("%s=%s", k, v))
	}
	if err := runStreaming(ctx, "nixpacks", nixArgs, logs); err != nil {
		return BuildResult{}, fmt.Errorf("nixpacks build: %w", err)
	}
	defer exec.Command("docker", "rmi", "-f", builderImage).Run()

	createOut, err := exec.CommandContext(ctx, "docker", "create", builderImage).Output()
	if err != nil {
		return BuildResult{}, fmt.Errorf("create builder container: %w", err)
	}
	containerID := strings.TrimSpace(string(createOut))
	defer exec.Command("docker", "rm", "-f", containerID).Run()

	outputDir := spec.StaticOutputDir
	if outputDir == "" {
		outputDir = "."
	}
	srcInContainer := path.Join("/app", outputDir)
	if out, err := exec.CommandContext(ctx, "docker", "cp", containerID+":"+srcInContainer+"/.", destDir).CombinedOutput(); err != nil {
		return BuildResult{}, fmt.Errorf("docker cp build output: %w: %s", err, out)
	}

	return BuildResult{OutputPath: destDir}, nil
}

func (e *DockerExecutor) buildDockerfile(ctx context.Context, buildDir string, spec BuildSpec, logs io.Writer) (BuildResult, error) {
	dockerfile := spec.DockerfilePath
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}
	args := []string{"build", "-t", spec.ImageTag, "-f", filepath.Join(buildDir, dockerfile)}
	for k, v := range spec.BuildArgs {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
	}
	args = append(args, buildDir)

	if err := runStreaming(ctx, "docker", args, logs); err != nil {
		return BuildResult{}, fmt.Errorf("docker build: %w", err)
	}
	imageID, err := e.inspectImageID(ctx, spec.ImageTag)
	if err != nil {
		return BuildResult{}, err
	}
	return BuildResult{ImageTag: spec.ImageTag, ImageID: imageID}, nil
}

// defaultNixpacksNodeVersion is the Node.js version nixpacks builds default
// to when the repo pins no version of its own. nixpacks 1.41.0 (the last
// release; the project is in maintenance mode) defaults to Node 18, which
// has reached End-Of-Life and been removed from the nixpkgs snapshot that
// CLI version pins -- so a "no Dockerfile" deploy of an unpinned Node repo
// fails at the nix-env step. A deployment can still pin its own version by
// setting NIXPACKS_NODE_VERSION in BuildArgs, which wins over this default.
const defaultNixpacksNodeVersion = "22"

// buildNixpacks shells out to the `nixpacks` CLI (a documented host
// dependency, installed alongside Docker) rather than reimplementing its
// language-detection/build-plan logic.
func (e *DockerExecutor) buildNixpacks(ctx context.Context, buildDir string, spec BuildSpec, logs io.Writer) (BuildResult, error) {
	args := []string{"build", buildDir, "--name", spec.ImageTag}
	// nixpacks reads NIXPACKS_NODE_VERSION from --env flags (build-time
	// vars), not the invoking process's environment -- see the plan-detection
	// verification. Pin a supported LTS unless the deployment overrides it.
	nodeVersion, explicit := spec.BuildArgs["NIXPACKS_NODE_VERSION"]
	if !explicit {
		nodeVersion = defaultNixpacksNodeVersion
	}
	args = append(args, "--env", "NIXPACKS_NODE_VERSION="+nodeVersion)
	for k, v := range spec.BuildArgs {
		if k == "NIXPACKS_NODE_VERSION" {
			continue
		}
		// BuildArgs reach the nixpacks build as --env (the CLI's flag for
		// build-time variables), not Docker's --build-arg, which nixpacks
		// doesn't accept.
		args = append(args, "--env", fmt.Sprintf("%s=%s", k, v))
	}

	if err := runStreaming(ctx, "nixpacks", args, logs); err != nil {
		return BuildResult{}, fmt.Errorf("nixpacks build: %w", err)
	}
	imageID, err := e.inspectImageID(ctx, spec.ImageTag)
	if err != nil {
		return BuildResult{}, err
	}
	return BuildResult{ImageTag: spec.ImageTag, ImageID: imageID}, nil
}

func (e *DockerExecutor) inspectImageID(ctx context.Context, ref string) (string, error) {
	out, err := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.Id}}", ref).Output()
	if err != nil {
		return "", fmt.Errorf("inspect image %s: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (e *DockerExecutor) Run(ctx context.Context, spec RunSpec) (RunResult, error) {
	replicas := spec.Replicas
	if replicas < 1 {
		replicas = 1
	}

	// Pull explicitly, before `docker run`, and only if the image isn't
	// already local: when `docker run` has to pull an image itself, it
	// interleaves multi-line pull-progress output with the single
	// container-ID line the code below expects as the *entire* output,
	// corrupting containerID into a useless blob (and, in turn, silently
	// defeating the orphan-container cleanup on a later failure, since it
	// tries to remove that blob instead of the real ID). Skipping the pull
	// when already-local also matters for locally-built images
	// (git/dockerfile/nixpacks strategies): those tags only ever exist on
	// this host, never in any registry, so `docker pull` on them would
	// just fail.
	if err := exec.CommandContext(ctx, "docker", "image", "inspect", spec.ImageRef).Run(); err != nil {
		out, pullErr := exec.CommandContext(ctx, "docker", "pull", spec.ImageRef).CombinedOutput()
		if pullErr != nil {
			return RunResult{}, fmt.Errorf("docker pull %s: %w: %s", spec.ImageRef, pullErr, bytes.TrimSpace(out))
		}
	}

	// File mounts are materialized once and shared across every replica --
	// the same host files are bind-mounted into each container.
	fileMounts, err := e.materializeFiles(ctx, spec)
	if err != nil {
		return RunResult{}, fmt.Errorf("materialize file mounts: %w", err)
	}

	result := RunResult{}
	var started []string // container IDs started so far, for cleanup on a mid-set failure
	for i := 0; i < replicas; i++ {
		containerName := spec.ContainerName
		if i > 0 {
			containerName = fmt.Sprintf("%s-%d", spec.ContainerName, i)
		}
		id, addr, err := e.runOne(ctx, spec, containerName, fileMounts)
		if err != nil {
			for _, cid := range started {
				_, _ = exec.CommandContext(ctx, "docker", "rm", "-f", cid).CombinedOutput()
			}
			return RunResult{}, err
		}
		started = append(started, id)
		rr := ReplicaResult{ContainerID: id, ContainerAddr: addr}
		if i == 0 {
			result.ContainerID = id
			result.ContainerAddr = addr
		} else {
			result.Replicas = append(result.Replicas, rr)
		}
	}
	return result, nil
}

// runOne starts a single container from spec with the given name and
// returns its ID and (if InternalPort > 0) its internal network address.
func (e *DockerExecutor) runOne(ctx context.Context, spec RunSpec, containerName string, fileMounts []materializedFile) (string, string, error) {
	args := []string{"run", "-d", "--name", containerName, "--network", e.effectiveNetwork(spec)}
	if spec.NetworkAlias != "" {
		args = append(args, "--network-alias", spec.NetworkAlias)
	}

	if spec.MemoryLimitMB > 0 {
		args = append(args, "--memory", fmt.Sprintf("%dm", spec.MemoryLimitMB), "--memory-swap", fmt.Sprintf("%dm", spec.MemoryLimitMB))
	}
	if spec.CPULimitCores > 0 {
		args = append(args, "--cpus", strconv.FormatFloat(spec.CPULimitCores, 'f', -1, 64))
	}
	if spec.RestartPolicy != "" {
		args = append(args, "--restart", spec.RestartPolicy)
	}
	if spec.CgroupParent != "" {
		args = append(args, "--cgroup-parent", spec.CgroupParent)
	}
	if spec.OOMScoreAdj != 0 {
		args = append(args, "--oom-score-adj", strconv.Itoa(spec.OOMScoreAdj))
	}
	if spec.HostPort != nil && spec.InternalPort > 0 {
		args = append(args, "-p", fmt.Sprintf("127.0.0.1:%d:%d", *spec.HostPort, spec.InternalPort))
	}
	for k, v := range spec.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	for _, vol := range spec.Volumes {
		args = append(args, "-v", fmt.Sprintf("%s:%s", vol.Name, vol.MountPath))
	}
	for _, fm := range fileMounts {
		args = append(args, "-v", fmt.Sprintf("%s:%s:ro", fm.hostPath, fm.mountPath))
	}
	args = append(args, spec.ImageRef)
	args = append(args, spec.Command...)

	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("docker run: %w: %s", err, out)
	}
	// `docker run -d` prints the container ID as its own last line; take
	// only that (not the whole trimmed output) as a safety net in case
	// something -- an image already pulled above hitting a registry
	// mirror notice, a Docker CLI deprecation warning, etc. -- still adds
	// another line ahead of it.
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	containerID := strings.TrimSpace(lines[len(lines)-1])

	addr := ""
	if spec.InternalPort > 0 {
		ip, err := e.containerIPWithRetry(ctx, containerID, spec.Network)
		if err != nil {
			// The container above was already created and started by
			// `docker run`; don't leak it just because we couldn't
			// determine its address -- the caller only sees this error,
			// never the containerID, so it has no way to clean this up
			// itself.
			_, _ = exec.CommandContext(ctx, "docker", "rm", "-f", containerID).CombinedOutput()
			return "", "", err
		}
		addr = net.JoinHostPort(ip, strconv.Itoa(spec.InternalPort))
	}

	return containerID, addr, nil
}

type materializedFile struct {
	hostPath  string
	mountPath string
}

// materializeFiles writes every FileMount's content to a fresh host temp
// file so Run() can bind-mount it into the container read-only. The temp
// files stay on disk for the container's lifetime -- Docker keeps the
// bind-mount open for as long as the container exists, so deleting them at
// start would break the running container. They're tiny and swept by the OS
// temp cleaner; the cost of correctness here is a few stray files under
// /tmp rather than a container that starts missing its init SQL.
func (e *DockerExecutor) materializeFiles(ctx context.Context, spec RunSpec) ([]materializedFile, error) {
	if len(spec.Files) == 0 {
		return nil, nil
	}
	dir, err := os.MkdirTemp("", "mangrove-files-*")
	if err != nil {
		return nil, err
	}
	out := make([]materializedFile, 0, len(spec.Files))
	for _, f := range spec.Files {
		if f.Path == "" || len(f.Content) == 0 {
			continue
		}
		// Flatten the container path into a filesystem-safe unique name so
		// two mounts targeting different paths can't collide in one temp dir.
		name := strings.Trim(strings.ReplaceAll(strings.TrimPrefix(f.Path, "/"), "/", "_"), "_.")
		if name == "" {
			name = "file"
		}
		hostPath := filepath.Join(dir, name)
		if err := os.WriteFile(hostPath, f.Content, 0o644); err != nil {
			return nil, err
		}
		out = append(out, materializedFile{hostPath: hostPath, mountPath: f.Path})
	}
	return out, nil
}

func (e *DockerExecutor) effectiveNetwork(spec RunSpec) string {
	if spec.Network != "" {
		return spec.Network
	}
	return e.NetworkName
}

func (e *DockerExecutor) containerIP(ctx context.Context, containerID, network string) (string, error) {
	if network == "" {
		network = e.NetworkName
	}
	// Network names commonly contain hyphens, which Go's text/template
	// can't address via dotted field syntax — use `index` instead.
	format := fmt.Sprintf(`{{index .NetworkSettings.Networks %q "IPAddress"}}`, network)
	out, err := exec.CommandContext(ctx, "docker", "inspect", "--format", format, containerID).CombinedOutput()
	if err != nil {
		// CombinedOutput (not Output): exec.ExitError's own Error() text is
		// just "exit status 1" -- the actual reason docker gave lives in
		// this command's stderr, and without it here this failure is
		// undiagnosable from the caller's error string alone.
		return "", fmt.Errorf("inspect container IP: %w: %s", err, bytes.TrimSpace(out))
	}
	ip := strings.TrimSpace(string(out))
	if ip == "" {
		return "", fmt.Errorf("container %s has no address on network %s", containerID, network)
	}
	return ip, nil
}

// containerIPWithRetry retries containerIP briefly. Run is the only caller
// that hits this immediately after `docker run -d` returns; the container's
// endpoint on spec.Network can very occasionally not be visible to `docker
// inspect` yet in that exact window, which otherwise surfaces as a
// leaked-but-running container plus a failed deploy.
func (e *DockerExecutor) containerIPWithRetry(ctx context.Context, containerID, network string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(200 * time.Millisecond):
			}
		}
		ip, err := e.containerIP(ctx, containerID, network)
		if err == nil {
			return ip, nil
		}
		lastErr = err
	}
	return "", lastErr
}

func (e *DockerExecutor) ContainerAddr(ctx context.Context, containerRef string, port int) (string, error) {
	ip, err := e.containerIP(ctx, containerRef, e.NetworkName)
	if err != nil {
		return "", err
	}
	return net.JoinHostPort(ip, strconv.Itoa(port)), nil
}

func (e *DockerExecutor) Stop(ctx context.Context, containerRef string, timeout time.Duration) error {
	secs := int(timeout.Seconds())
	if secs <= 0 {
		secs = 10
	}
	out, err := exec.CommandContext(ctx, "docker", "stop", "-t", strconv.Itoa(secs), containerRef).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker stop: %w: %s", err, out)
	}
	return nil
}

// Restart stops and starts a container in place -- including one already
// stopped, since `docker restart` on an exited container just starts it.
func (e *DockerExecutor) Restart(ctx context.Context, containerRef string, timeout time.Duration) error {
	secs := int(timeout.Seconds())
	if secs <= 0 {
		secs = 10
	}
	out, err := exec.CommandContext(ctx, "docker", "restart", "-t", strconv.Itoa(secs), containerRef).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker restart: %w: %s", err, out)
	}
	return nil
}

func (e *DockerExecutor) Remove(ctx context.Context, containerRef string) error {
	out, err := exec.CommandContext(ctx, "docker", "rm", "-f", containerRef).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker rm: %w: %s", err, out)
	}
	return nil
}

// Exec runs cmd inside containerRef via `docker exec`. A non-zero exit
// status is surfaced through ExecResult, not as a Go error -- only a
// failure to invoke `docker exec` at all (container gone, daemon
// unreachable, etc) is returned as an error.
func (e *DockerExecutor) Exec(ctx context.Context, containerRef string, cmd []string) (ExecResult, error) {
	args := append([]string{"exec", containerRef}, cmd...)
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return ExecResult{Output: string(out), ExitCode: exitErr.ExitCode()}, nil
		}
		return ExecResult{}, fmt.Errorf("docker exec: %w: %s", err, out)
	}
	return ExecResult{Output: string(out), ExitCode: 0}, nil
}

func (e *DockerExecutor) HealthCheck(ctx context.Context, containerRef string, cfg HealthCheckSpec) (HealthStatus, error) {
	network := e.NetworkName
	ip, err := e.containerIP(ctx, containerRef, network)
	if err != nil {
		return HealthStatus{Healthy: false, Error: err.Error()}, nil
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	// Redirects are not followed: a bare 3xx already counts as healthy below
	// (many apps redirect / -> /login, or upgrade to their configured
	// public https:// hostname, which won't resolve from inside the
	// container network at all -- following it would turn a perfectly
	// healthy container into a health-check failure).
	client := &http.Client{
		Timeout:       timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}
	path := cfg.Path
	if path == "" {
		path = "/"
	}
	url := fmt.Sprintf("http://%s%s", net.JoinHostPort(ip, strconv.Itoa(cfg.Port)), path)

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return HealthStatus{Healthy: false, Error: err.Error()}, nil
	}
	resp, err := client.Do(req)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return HealthStatus{Healthy: false, ResponseTimeMS: elapsed, Error: err.Error()}, nil
	}
	defer resp.Body.Close()

	healthy := resp.StatusCode >= 200 && resp.StatusCode < 400
	return HealthStatus{Healthy: healthy, StatusCode: resp.StatusCode, ResponseTimeMS: elapsed}, nil
}

// dockerLogsCloser wraps a running `docker logs` process so Close() also
// terminates the underlying command instead of leaking it.
type dockerLogsCloser struct {
	io.ReadCloser
	cmd *exec.Cmd
}

func (d *dockerLogsCloser) Close() error {
	err := d.ReadCloser.Close()
	if d.cmd.Process != nil {
		d.cmd.Process.Kill()
	}
	d.cmd.Wait()
	return err
}

func (e *DockerExecutor) Logs(ctx context.Context, containerRef string, opts LogOptions) (io.ReadCloser, error) {
	args := []string{"logs"}
	tail := opts.Tail
	if tail == "" {
		tail = "200"
	}
	args = append(args, "--tail", tail)
	if opts.Follow {
		args = append(args, "-f")
	}
	args = append(args, containerRef)

	cmd := exec.CommandContext(ctx, "docker", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = cmd.Stdout // docker logs interleaves stdout/stderr by design; keep both in one stream
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start docker logs: %w", err)
	}
	return &dockerLogsCloser{ReadCloser: stdout, cmd: cmd}, nil
}

type dockerStatsJSON struct {
	CPUPerc  string `json:"CPUPerc"`
	MemUsage string `json:"MemUsage"` // "12.3MiB / 256MiB"
}

func (e *DockerExecutor) Stats(ctx context.Context, containerRef string) (ResourceStats, error) {
	out, err := exec.CommandContext(ctx, "docker", "stats", "--no-stream", "--format", "{{json .}}", containerRef).Output()
	if err != nil {
		return ResourceStats{}, fmt.Errorf("docker stats: %w", err)
	}
	var raw dockerStatsJSON
	if err := json.Unmarshal(bytes.TrimSpace(out), &raw); err != nil {
		return ResourceStats{}, fmt.Errorf("parse docker stats output: %w", err)
	}

	cpuPct, _ := strconv.ParseFloat(strings.TrimSuffix(raw.CPUPerc, "%"), 64)
	_, limitMB := parseMemUsage(raw.MemUsage)

	// `docker stats` MemUsage is the cgroup's memory.usage_in_bytes, which
	// counts reclaimable page cache -- a freshly-written DB image can show
	// 20 MiB when only ~5 MiB is real process memory. The admin budget
	// reports actual usage, so use the cgroup's anonymous (RSS-equivalent)
	// counter instead; fall back to the stats value if we can't read it.
	usedMB, err := e.anonMemoryMB(ctx, containerRef)
	if err != nil {
		usedMB, _ = parseMemUsage(raw.MemUsage)
	}

	return ResourceStats{CPUPercent: cpuPct, MemUsageMB: usedMB, MemLimitMB: limitMB}, nil
}

// anonMemoryMB returns a container's anonymous memory usage in MB, read
// from its cgroup's memory.stat `anon` counter. On a cgroup v2 host the
// container's cgroup is derived from the process's own cgroup (read via
// /proc/<pid>/cgroup), then memory.stat is read from the mounted hierarchy.
// A container that exited between the docker inspect and the read yields an
// error, which the caller treats as "use the docker stats value instead".
func (e *DockerExecutor) anonMemoryMB(ctx context.Context, containerRef string) (float64, error) {
	pidOut, err := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.State.Pid}}", containerRef).Output()
	if err != nil {
		return 0, fmt.Errorf("inspect pid: %w", err)
	}
	pid := strings.TrimSpace(string(pidOut))
	if pid == "" || pid == "0" {
		return 0, fmt.Errorf("container has no pid")
	}

	cgroupData, err := os.ReadFile(fmt.Sprintf("/proc/%s/cgroup", pid))
	if err != nil {
		return 0, fmt.Errorf("read /proc/%s/cgroup: %w", pid, err)
	}
	// cgroup v2 line looks like: 0::/system.slice/docker-<id>.scope
	// cgroup v1 line looks like: 3:memory:/docker/<id> (controllers vary).
	var statPath string
	for _, line := range strings.Split(string(cgroupData), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}
		controllers, rel := parts[1], parts[2]
		switch {
		case controllers == "": // v2: hierarchy 0 lists no controllers
			// The v2 hierarchy is mounted directly at /sys/fs/cgroup and the
			// path is already rooted at it (e.g. /system.slice/...).
			statPath = "/sys/fs/cgroup" + rel
		case strings.Contains(controllers, "memory"):
			// v1: the memory controller has its own hierarchy mounted at
			// /sys/fs/cgroup/memory.
			statPath = "/sys/fs/cgroup/memory" + rel
		default:
			continue
		}
		break
	}
	if statPath == "" {
		return 0, fmt.Errorf("no memory cgroup found")
	}

	data, err := os.ReadFile(statPath + "/memory.stat")
	if err != nil {
		return 0, fmt.Errorf("read %s/memory.stat: %w", statPath, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "anon ") {
			fields := strings.Fields(line)
			if len(fields) == 2 {
				if b, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
					return float64(b) / (1024 * 1024), nil
				}
			}
		}
	}
	return 0, fmt.Errorf("no anon counter in memory.stat")
}

// parseMemUsage parses Docker's "12.3MiB / 256MiB" stats format into MB.
func parseMemUsage(s string) (used, limit float64) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return 0, 0
	}
	return parseSizeToMB(strings.TrimSpace(parts[0])), parseSizeToMB(strings.TrimSpace(parts[1]))
}

// parseSizeToMB parses a Docker-formatted size into MB. `docker stats` uses
// binary units (KiB/MiB/GiB); `docker ... prune` uses decimal units
// (kB/MB/GB). Suffixes are checked longest-first so "MiB" isn't mistaken
// for a trailing "B".
func parseSizeToMB(s string) float64 {
	type unit struct {
		suffix string
		mult   float64
	}
	units := []unit{
		{"TiB", 1024 * 1024}, {"GiB", 1024}, {"MiB", 1}, {"KiB", 1.0 / 1024},
		{"TB", 1000 * 1000}, {"GB", 1000}, {"MB", 1}, {"kB", 1.0 / 1000},
		{"B", 1.0 / (1024 * 1024)},
	}
	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			v, err := strconv.ParseFloat(strings.TrimSuffix(s, u.suffix), 64)
			if err != nil {
				return 0
			}
			return v * u.mult
		}
	}
	return 0
}

// RemoveVolume deletes a named Docker volume. Not an error if it's already
// gone -- "-f" no-ops rather than failing on a missing volume.
func (e *DockerExecutor) RemoveVolume(ctx context.Context, name string) error {
	out, err := exec.CommandContext(ctx, "docker", "volume", "rm", "-f", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker volume rm: %w: %s", err, out)
	}
	return nil
}

// RemoveImage deletes a specific tagged image. Not an error if it's already
// gone -- "-f" no-ops rather than failing on a missing image.
func (e *DockerExecutor) RemoveImage(ctx context.Context, ref string) error {
	out, err := exec.CommandContext(ctx, "docker", "rmi", "-f", ref).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker rmi: %w: %s", err, out)
	}
	return nil
}

func (e *DockerExecutor) Prune(ctx context.Context, opts PruneOptions) (PruneResult, error) {
	args := []string{"image", "prune", "-f"}
	for _, tag := range opts.KeepImageTags {
		args = append(args, "--filter", fmt.Sprintf("label!=mangrove.keep=%s", tag))
	}
	out, err := exec.CommandContext(ctx, "docker", args...).Output()
	if err != nil {
		return PruneResult{}, fmt.Errorf("docker image prune: %w", err)
	}
	return parsePruneOutput(string(out)), nil
}

func parsePruneOutput(out string) PruneResult {
	var result PruneResult
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "deleted:") || strings.HasPrefix(line, "Deleted Images:") {
			result.ImagesRemoved++
		}
		if strings.HasPrefix(line, "Total reclaimed space:") {
			sizeStr := strings.TrimSpace(strings.TrimPrefix(line, "Total reclaimed space:"))
			result.SpaceReclaimedMB = int64(parseSizeToMB(sizeStr))
		}
	}
	return result
}

// runStreaming runs a command, copying combined stdout/stderr to logs as it
// arrives (not buffered until exit) so a caller can show live build output.
func runStreaming(ctx context.Context, name string, args []string, logs io.Writer) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = logs
	cmd.Stderr = logs
	return cmd.Run()
}
