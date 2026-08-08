// Package models holds the Go-side row types shared by the store, API, and
// orchestrator packages.
package models

import "time"

type Project struct {
	ID          int64     `json:"id"`
	WorkspaceID int64     `json:"workspace_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Deployment struct {
	ID                  int64      `json:"id"`
	ProjectID           int64      `json:"project_id"`
	Name                string     `json:"name"`
	Slug                string     `json:"slug"`
	BuildStrategy       string     `json:"build_strategy"`
	GitBranch           string     `json:"git_branch,omitempty"`
	ProjectRepoID       *int64     `json:"project_repo_id,omitempty"`
	ImageRef            string     `json:"image_ref,omitempty"`
	RootPath            string     `json:"root_path"`
	DockerfilePath      string     `json:"dockerfile_path,omitempty"`
	ComposePath         string     `json:"compose_path,omitempty"`
	StaticBuildCommand  string     `json:"static_build_command,omitempty"`
	StaticOutputDir     string     `json:"static_output_dir,omitempty"`
	AutoDeployOnPush    bool       `json:"auto_deploy_on_push"`
	IsPublic            bool       `json:"is_public"`
	PasswordProtected   bool       `json:"password_protected"`
	ImageRetentionCount int        `json:"image_retention_count"`
	Status              string     `json:"status"`
	NodeID              int64      `json:"node_id"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	LastDeployedAt      *time.Time `json:"last_deployed_at,omitempty"`
}

type Service struct {
	ID                   int64   `json:"id"`
	DeploymentID         int64   `json:"deployment_id"`
	Name                 string  `json:"name"`
	IsPrimary            bool    `json:"is_primary"`
	ImageTagCurrent      string  `json:"image_tag_current,omitempty"`
	ContainerIDCurrent   string  `json:"container_id_current,omitempty"`
	ContainerName        string  `json:"container_name"`
	InternalPort         int     `json:"internal_port"`
	HostPort             *int    `json:"host_port,omitempty"`
	IsInternalOnly       bool    `json:"is_internal_only"`
	CPULimitCores        float64 `json:"cpu_limit_cores"`
	MemoryLimitMB        int     `json:"memory_limit_mb"`
	RestartPolicy        string  `json:"restart_policy"`
	HealthCheckPath      string  `json:"health_check_path,omitempty"`
	HealthCheckIntervalS int     `json:"health_check_interval_s"`
	HealthCheckTimeoutS  int     `json:"health_check_timeout_s"`
	// Command overrides the image's default CMD when non-empty (e.g.
	// Redis's --requirepass) -- see executor.RunSpec.Command.
	Command   []string  `json:"command,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type DeployHistory struct {
	ID                        int64      `json:"id"`
	DeploymentID              int64      `json:"deployment_id"`
	TriggeredBy               string     `json:"triggered_by"`
	CommitSHA                 string     `json:"commit_sha,omitempty"`
	CommitMessage             string     `json:"commit_message,omitempty"`
	GitRef                    string     `json:"git_ref,omitempty"`
	Status                    string     `json:"status"`
	StartedAt                 time.Time  `json:"started_at"`
	FinishedAt                *time.Time `json:"finished_at,omitempty"`
	IsCurrent                 bool       `json:"is_current"`
	RollbackOfDeployHistoryID *int64     `json:"rollback_of_deploy_history_id,omitempty"`
	ErrorMessage              string     `json:"error_message,omitempty"`
}

type DeployArtifact struct {
	ID              int64  `json:"id"`
	DeployHistoryID int64  `json:"deploy_history_id"`
	ServiceID       int64  `json:"service_id"`
	ImageTag        string `json:"image_tag"`
	ImageID         string `json:"image_id,omitempty"`
	BuildLogPath    string `json:"build_log_path,omitempty"`
	// OutputPath is set instead of ImageTag/ImageID for a static-strategy
	// deploy: the host directory Caddy's file_server root points at.
	OutputPath string `json:"output_path,omitempty"`
}

type EnvVar struct {
	ID        int64  `json:"id"`
	ServiceID int64  `json:"service_id"`
	KeyName   string `json:"key_name"`
	IsSecret  bool   `json:"is_secret"`
	Value     string `json:"value,omitempty"` // decrypted for API responses only when explicitly requested
}

type Volume struct {
	ID               int64  `json:"id"`
	DeploymentID     int64  `json:"deployment_id"`
	ServiceID        *int64 `json:"service_id,omitempty"`
	Name             string `json:"name"`
	DockerVolumeName string `json:"docker_volume_name"`
	MountPath        string `json:"mount_path"`
	Driver           string `json:"driver"`
	SizeEstimateMB   *int   `json:"size_estimate_mb,omitempty"`
}
