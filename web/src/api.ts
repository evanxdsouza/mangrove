export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const resp = await fetch(path, {
    method,
    headers: body !== undefined ? { "Content-Type": "application/json" } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
    credentials: "same-origin",
  });

  if (resp.status === 204) {
    return undefined as T;
  }

  const text = await resp.text();
  const data = text ? JSON.parse(text) : undefined;

  if (!resp.ok) {
    const message = (data && typeof data === "object" && "error" in data) ? String((data as any).error) : resp.statusText;
    throw new ApiError(resp.status, message);
  }
  return data as T;
}

export const api = {
  get: <T,>(path: string) => request<T>("GET", path),
  post: <T,>(path: string, body?: unknown) => request<T>("POST", path, body ?? {}),
  put: <T,>(path: string, body?: unknown) => request<T>("PUT", path, body ?? {}),
  del: <T,>(path: string) => request<T>("DELETE", path),
};

// ---- Types mirroring internal/models and API response shapes ----

export interface Project {
  id: number;
  workspace_id: number;
  name: string;
  slug: string;
  description?: string;
  workspace_name?: string;
  workspace_slug?: string;
  created_at: string;
  updated_at: string;
}

export interface Workspace {
  id: number;
  org_id: number;
  name: string;
  slug: string;
  created_at: string;
}

export interface WorkspaceProjectCount {
  workspace: Workspace;
  project_count: number;
}

export interface Deployment {
  id: number;
  project_id: number;
  name: string;
  slug: string;
  build_strategy: "dockerfile" | "nixpacks" | "compose" | "image" | "static";
  git_branch?: string;
  image_ref?: string;
  root_path: string;
  dockerfile_path?: string;
  compose_path?: string;
  static_build_command?: string;
  static_output_dir?: string;
  auto_deploy_on_push: boolean;
  is_public: boolean;
  password_protected: boolean;
  image_retention_count: number;
  replicas: number;
  environment: "production" | "staging" | "preview";
  promotes_to_deployment_id?: number;
  pr_previews_enabled: boolean;
  pr_number?: number;
  status: "pending" | "building" | "running" | "stopped" | "failed";
  node_id: number;
  created_at: string;
  updated_at: string;
  last_deployed_at?: string;
}

export interface Service {
  id: number;
  deployment_id: number;
  name: string;
  is_primary: boolean;
  image_tag_current?: string;
  container_id_current?: string;
  replica_container_ids?: string[];
  container_name: string;
  internal_port: number;
  host_port?: number;
  is_internal_only: boolean;
  cpu_limit_cores: number;
  memory_limit_mb: number;
  restart_policy: string;
  health_check_path?: string;
  health_check_interval_s: number;
  health_check_timeout_s: number;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface DeployHistory {
  id: number;
  deployment_id: number;
  triggered_by: string;
  commit_sha?: string;
  commit_message?: string;
  git_ref?: string;
  status: string;
  started_at: string;
  finished_at?: string;
  is_current: boolean;
  rollback_of_deploy_history_id?: number;
  error_message?: string;
}

export interface WebhookEvent {
  id: number;
  event_type: string;
  signature_valid: boolean;
  status: "received" | "verified" | "rejected" | "processed" | "ignored_no_match";
  received_at: string;
  deploy_history_id?: number;
}

export interface HealthCheckEntry {
  status: string;
  checked_at: string;
  response_time_ms: number;
  http_status?: number;
  error_message?: string;
}

export interface EnvVarEntry {
  key: string;
  is_secret: boolean;
  value?: string;
}

export interface AuthStatus {
  setup_required: boolean;
}

export interface CurrentUser {
  id: number;
  email?: string;
  role: "owner" | "member";
}

export interface TemplatePromptedEnvVar {
  key: string;
  label?: string;
  required?: boolean;
}

export interface TemplateDeploymentSummary {
  slug_suffix: string;
  name_suffix: string;
  build_strategy: string;
  image_ref?: string;
  git_url?: string;
  memory_limit_mb: number;
  cpu_limit_cores: number;
  force_internal_only: boolean;
  prompted_env?: TemplatePromptedEnvVar[];
}

export interface TemplateSummary {
  key: string;
  name: string;
  description: string;
  category: string;
  total_memory_mb: number;
  deployments: TemplateDeploymentSummary[];
}

export interface GithubCredential {
  id: number;
  label: string;
  source: "pat" | "oauth";
  github_login?: string;
  created_at: string;
  last_used_at?: string;
}

export interface GithubRepo {
  owner: string;
  name: string;
  private: boolean;
  default_branch: string;
  description?: string;
  pushed_at?: string;
}

export interface DetectedEnvVar {
  key: string;
  secret: boolean;
}

export interface DetectionResult {
  build_strategy: "dockerfile" | "nixpacks" | "compose" | "static";
  dockerfile_path?: string;
  compose_path?: string;
  env_vars?: DetectedEnvVar[];
  suggested_port?: number;
}

export interface CustomDomain {
  id: number;
  deployment_id: number;
  hostname: string;
  verification_token: string;
  verified: boolean;
  created_at: string;
}

export interface TemplateInstallResult {
  template_key: string;
  deployments: {
    deployment_id: number;
    slug: string;
    name: string;
    deploy_error?: string;
  }[];
  credentials?: Record<string, string>;
}

// Storage/NAS: see internal/mountd (the privileged drive-mounting helper)
// and internal/orchestrator/storage.go.
export interface Drive {
  uuid: string;
  device: string;
  label?: string;
  filesystem?: string;
  size_bytes: number;
  removable: boolean;
  mounted: boolean;
  mount_path?: string;
}

export interface DrivesResponse {
  helper_available: boolean;
  drives: Drive[];
}

export interface NASShareInfo {
  project_id: number;
  deployment_id: number;
  deployment_name: string;
  deployment_slug: string;
  service_id: number;
  status: string;
  share_name: string;
  direct_publish_port: number;
  host_mount_source: string;
}
