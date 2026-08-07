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
	Remove(ctx context.Context, containerRef string) error
	// HealthCheck performs a single HTTP probe against the container.
	HealthCheck(ctx context.Context, containerRef string, cfg HealthCheckSpec) (HealthStatus, error)
	Logs(ctx context.Context, containerRef string, opts LogOptions) (io.ReadCloser, error)
	Stats(ctx context.Context, containerRef string) (ResourceStats, error)
	// Prune removes dangling images/build cache, keeping anything whose tag
	// is listed in opts.KeepImageTags regardless of age.
	Prune(ctx context.Context, opts PruneOptions) (PruneResult, error)
}

// BuildStrategy enumerates how BuildSpec.Context should be turned into an image.
type BuildStrategy string

const (
	StrategyDockerfile BuildStrategy = "dockerfile"
	StrategyNixpacks   BuildStrategy = "nixpacks"
	StrategyCompose    BuildStrategy = "compose"
	StrategyImage       BuildStrategy = "image" // pre-built image ref, no build step
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
}

type BuildResult struct {
	ImageTag string
	ImageID  string
}

type VolumeMount struct {
	Name      string // Docker volume name
	MountPath string
}

type RunSpec struct {
	ImageRef      string
	ContainerName string
	Env           map[string]string
	Network       string
	InternalPort  int
	HostPort      *int // nil unless the service opts out of proxy-only routing
	CPULimitCores float64
	MemoryLimitMB int
	RestartPolicy string
	Volumes       []VolumeMount
	OOMScoreAdj   int
	CgroupParent  string
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
