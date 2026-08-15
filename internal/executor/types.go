// Package executor defines the seam between Mangrove's orchestration logic
// and wherever a deployment actually runs. Today the only implementation
// (DockerExecutor) drives the local Docker daemon; a future remote worker
// (SSH tunnel or gRPC agent) can satisfy the same interface without any
// caller changes. Nothing in this package or its callers may assume a
// shared filesystem with the caller — see ContextSource.
package executor

import (
	"context"
	"io"
	"time"
)

// Executor performs the build/run/observe lifecycle for a single service.
type Executor interface {
	// Build produces a tagged image from spec, streaming build output to logs.
	Build(ctx context.Context, spec BuildSpec, logs io.Writer) (BuildResult, error)
	// Run starts a new container from an already-built image.
	Run(ctx context.Context, spec RunSpec) (RunResult, error)
	Stop(ctx context.Context, containerRef string, timeout time.Duration) error
	// Restart stops and starts an already-created container in place (same
	// container ID, same volumes) -- unlike the blue/green swap Deploy()
	// performs, this reuses the existing container rather than creating a
	// new one from a (possibly rebuilt) image. Also the way to start a
	// container previously stopped via Stop: `docker restart` on an
	// exited container starts it.
	Restart(ctx context.Context, containerRef string, timeout time.Duration) error
	Remove(ctx context.Context, containerRef string) error
	// Exec runs a one-off command inside an already-running container (e.g.
	// a database migration) and returns its combined output and exit code.
	// A non-zero exit is reported via ExecResult.ExitCode, not as a Go
	// error -- the command ran, it just failed; a returned error means the
	// command could not be run at all (container gone, docker exec itself
	// failed to start, etc).
	Exec(ctx context.Context, containerRef string, cmd []string) (ExecResult, error)
	// HealthCheck performs a single HTTP probe against the container.
	HealthCheck(ctx context.Context, containerRef string, cfg HealthCheckSpec) (HealthStatus, error)
	Logs(ctx context.Context, containerRef string, opts LogOptions) (io.ReadCloser, error)
	Stats(ctx context.Context, containerRef string) (ResourceStats, error)
	// Prune removes dangling images/build cache, keeping anything whose tag
	// is listed in opts.KeepImageTags regardless of age.
	Prune(ctx context.Context, opts PruneOptions) (PruneResult, error)
	// RemoveImage deletes a specific tagged image outright, unlike Prune
	// (which only ever sweeps dangling/untagged layers). Used when tearing
	// down a service/deployment/project, where every image tag it ever
	// built must go, not just the ones that happen to be dangling.
	// Best-effort by convention -- callers log rather than fail a delete
	// over it, since an image already gone shouldn't block the rest of a
	// teardown.
	RemoveImage(ctx context.Context, ref string) error
	// RemoveVolume deletes a named Docker volume. Best-effort by
	// convention -- callers log rather than fail a delete over it, since a
	// volume already gone (or still referenced by something outside
	// Mangrove's view) shouldn't block tearing down the rest of a project.
	RemoveVolume(ctx context.Context, name string) error
	// ContainerAddr returns "internal-ip:port" for an already-running
	// container -- used to re-push a Caddy route (e.g. after an
	// access-control toggle) without needing a fresh Run().
	ContainerAddr(ctx context.Context, containerRef string, port int) (string, error)
}

// BuildStrategy enumerates how BuildSpec.Context should be turned into an image.
type BuildStrategy string

const (
	StrategyDockerfile BuildStrategy = "dockerfile"
	StrategyNixpacks   BuildStrategy = "nixpacks"
	StrategyCompose    BuildStrategy = "compose"
	StrategyImage      BuildStrategy = "image"  // pre-built image ref, no build step
	StrategyStatic     BuildStrategy = "static" // build (optional) + serve a directory via Caddy, no container
)

// ContextSource describes where to fetch a build's source from. It is
// deliberately never a bare host path the orchestrator hands to the
// executor — a git ref or a tarball stream are both things a remote
// executor could equally consume over SSH/gRPC. Each Executor
// implementation is responsible for materializing this into its own local
// working directory.
type ContextSource struct {
	GitURL    string    // https://github.com/owner/repo.git
	GitRef    string    // branch, tag, or commit SHA
	AuthToken string    // decrypted PAT; passed at call time, never persisted by the executor
	Tarball   io.Reader // alternative to GitURL: a pre-fetched build context stream
}

type BuildSpec struct {
	Strategy       BuildStrategy
	Context        ContextSource
	RootPath       string // subdirectory within the fetched context to build from
	DockerfilePath string // relative to RootPath; strategy == dockerfile
	ComposePath    string // relative to RootPath; strategy == compose
	ImageTag       string
	ImageRef       string // strategy == image: use this ref directly, no build
	BuildArgs      map[string]string

	// StaticBuildCommand, when set, is run in a nixpacks-provisioned builder
	// container (e.g. "npm run build"); strategy == static only. Left empty
	// when the repo is already pre-built HTML/CSS/JS, in which case
	// StaticOutputDir is copied straight out of the fetched context with no
	// build container at all.
	StaticBuildCommand string
	// StaticOutputDir is the directory holding the built assets: relative to
	// the builder's /app when StaticBuildCommand is set, otherwise relative
	// to RootPath. Empty means RootPath/the builder root itself.
	StaticOutputDir string
	// StaticOutputName is a filesystem-safe unique identifier for this
	// build's output directory (e.g. "<slug>-<deployHistoryID>"); strategy
	// == static only.
	StaticOutputName string
}

type BuildResult struct {
	ImageTag string
	ImageID  string
	// OutputPath is set instead of ImageTag/ImageID for strategy == static:
	// the absolute host directory the built (or copied) static assets now
	// live in, for Caddy's file_server to serve directly.
	OutputPath string
}

type VolumeMount struct {
	Name      string // Docker volume name
	MountPath string
}

// FileMount is a small inline file the executor writes to a host path at
// container-start time and bind-mounts into the container read-only (e.g. a
// template deployment's post-baked-init SQL file, or a config file the
// image expects to be present). Unlike a named VolumeMount there's no store
// row for it: it exists only for the lifetime of the container it was
// created for, and only for deployments that carried it in their RunSpec.
// The executor materializes Content into its own temporary storage; callers
// never hand it a host path (see ContextSource for why).
type FileMount struct {
	Path    string // absolute path inside the container
	Content []byte
}

type RunSpec struct {
	ImageRef      string
	ContainerName string
	Env           map[string]string
	Network       string
	// NetworkAlias, when set, is attached via `docker run --network-alias`
	// so other containers on the same network can reach this service by a
	// name that stays stable across the blue/green swap's per-deploy
	// ContainerName (which embeds the deploy_history id and so changes
	// every deploy). Used for template-linked dependencies (e.g. WordPress
	// resolving its MySQL sibling deployment's address).
	NetworkAlias  string
	InternalPort  int
	HostPort      *int // nil unless the service opts out of proxy-only routing
	CPULimitCores float64
	MemoryLimitMB int
	RestartPolicy string
	Volumes       []VolumeMount
	// Files, when set, are written to host temp files and bind-mounted into
	// the container read-only before it starts. Used by template installs to
	// seed a container with content it needs before first boot (e.g. a
	// Postgres entrypoint-initdb.d script) without going through the store.
	Files []FileMount
	// Command, when set, overrides the image's default CMD (e.g. Redis's
	// `--requirepass`). Passed as exec-form args after the image ref, not
	// through a shell, unless an entry itself invokes one (e.g. "sh", "-c").
	Command      []string
	OOMScoreAdj  int
	CgroupParent string
}

type RunResult struct {
	ContainerID   string
	ContainerAddr string // internal network address:port, for pre-swap health checks
}

type HealthCheckSpec struct {
	Path           string
	Port           int
	TimeoutSeconds int
}

type HealthStatus struct {
	Healthy        bool
	StatusCode     int
	ResponseTimeMS int64
	Error          string
}

// ExecResult is the outcome of a one-off command run via Executor.Exec.
type ExecResult struct {
	Output   string // combined stdout+stderr
	ExitCode int
}

type LogOptions struct {
	Follow bool
	Tail   string // e.g. "200", or "all"
}

type ResourceStats struct {
	CPUPercent float64
	MemUsageMB float64
	MemLimitMB float64
}

type PruneOptions struct {
	KeepImageTags []string
}

type PruneResult struct {
	ImagesRemoved    int
	SpaceReclaimedMB int64
}
