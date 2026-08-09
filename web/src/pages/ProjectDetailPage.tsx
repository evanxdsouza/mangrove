import { useEffect, useState, type FormEvent } from "react";
import { api, ApiError, type Deployment, type Project } from "../api";
import { Link } from "../router";
import { Modal } from "../components/Modal";
import { ConfirmModal } from "../components/ConfirmModal";
import { StatusPill } from "../components/StatusPill";
import { TemplateGalleryModal } from "../components/TemplateGalleryModal";
import { slugify } from "./ProjectsPage";
import { useIsOwner } from "../userContext";

interface ProjectRepoInfo {
  id: number;
  repo_owner: string;
  repo_name: string;
  default_branch: string;
  webhook_path: string;
}

export function ProjectDetailPage({ projectId }: { projectId: number }) {
  const isOwner = useIsOwner();
  const [project, setProject] = useState<Project | null>(null);
  const [deployments, setDeployments] = useState<Deployment[] | null>(null);
  const [repo, setRepo] = useState<ProjectRepoInfo | null | undefined>(undefined); // undefined = not loaded yet, null = none linked
  const [error, setError] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [showLinkRepo, setShowLinkRepo] = useState(false);
  const [showTemplates, setShowTemplates] = useState(false);
  const [showDelete, setShowDelete] = useState(false);

  const load = () => {
    api.get<Project>(`/api/projects/${projectId}`).then(setProject).catch((e) => setError(errMsg(e)));
    api
      .get<Deployment[]>(`/api/projects/${projectId}/deployments`)
      .then((d) => setDeployments(d ?? []))
      .catch((e) => setError(errMsg(e)));
    api
      .get<ProjectRepoInfo>(`/api/projects/${projectId}/repo`)
      .then(setRepo)
      .catch(() => setRepo(null));
  };

  useEffect(load, [projectId]);

  return (
    <>
      <div className="breadcrumb">
        <Link to="/">Projects</Link> / {project?.name ?? "..."}
      </div>
      <div className="page-header">
        <div>
          <h1>{project?.name ?? "Loading..."}</h1>
          {project?.description && <p>{project.description}</p>}
        </div>
        <div className="flex gap-8">
          <button className="btn" onClick={() => setShowTemplates(true)}>
            Deploy from template
          </button>
          <button className="btn btn-primary" onClick={() => setShowCreate(true)}>
            + New deployment
          </button>
          {isOwner && (
            <button className="btn btn-danger" onClick={() => setShowDelete(true)}>
              Delete project
            </button>
          )}
        </div>
      </div>

      {error && <div className="error-banner">{error}</div>}

      {deployments === null ? (
        <div className="center-loading">
          <div className="spinner" />
        </div>
      ) : deployments.length === 0 ? (
        <div className="card empty-state">No deployments yet in this project.</div>
      ) : (
        <div className="card" style={{ padding: 0 }}>
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Strategy</th>
                <th>Status</th>
                <th>Visibility</th>
                <th>Last deployed</th>
              </tr>
            </thead>
            <tbody>
              {deployments.map((d) => (
                <tr
                  key={d.id}
                  className="row-link"
                  onClick={() => (window.location.href = `/projects/${projectId}/deployments/${d.id}`)}
                >
                  <td>
                    <Link to={`/projects/${projectId}/deployments/${d.id}`}>{d.name}</Link>
                  </td>
                  <td className="mono text-dim">{d.build_strategy}</td>
                  <td>
                    <StatusPill status={d.status} />
                  </td>
                  <td className="text-dim">
                    {d.is_public ? (d.password_protected ? "Password-protected" : "Public") : "Internal only"}
                  </td>
                  <td className="text-dim">{d.last_deployed_at ? new Date(d.last_deployed_at).toLocaleString() : "never"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <div className="card">
        <div className="flex-between" style={{ marginBottom: repo ? 14 : 0 }}>
          <div className="card-title" style={{ margin: 0 }}>
            GitHub
          </div>
          {!repo && (
            <button className="btn btn-sm" onClick={() => setShowLinkRepo(true)}>
              Connect a repo
            </button>
          )}
        </div>
        {repo && (
          <div className="kv-list">
            <div className="kv-row">
              <span className="kv-key">Repo</span>
              <span className="kv-value">
                {repo.repo_owner}/{repo.repo_name}
              </span>
            </div>
            <div className="kv-row">
              <span className="kv-key">Default branch</span>
              <span className="kv-value">{repo.default_branch}</span>
            </div>
            <div className="kv-row">
              <span className="kv-key">Webhook path</span>
              <span className="kv-value">{repo.webhook_path}</span>
            </div>
            <div className="field-hint">
              Set each deployment's auto-deploy branch from its own page. Paste the webhook URL and secret (shown once,
              at link time) into this repo's GitHub settings &rarr; Webhooks.
            </div>
          </div>
        )}
      </div>

      {showCreate && (
        <CreateDeploymentModal
          projectId={projectId}
          onClose={() => setShowCreate(false)}
          onCreated={(id) => {
            window.location.href = `/projects/${projectId}/deployments/${id}`;
          }}
        />
      )}

      {showLinkRepo && (
        <LinkRepoModal
          projectId={projectId}
          onClose={() => setShowLinkRepo(false)}
          onLinked={() => {
            setShowLinkRepo(false);
            load();
          }}
        />
      )}

      {showTemplates && (
        <TemplateGalleryModal
          projectId={projectId}
          onClose={() => setShowTemplates(false)}
          onInstalled={() => {
            setShowTemplates(false);
            load();
          }}
        />
      )}

      {showDelete && (
        <ConfirmModal
          title="Delete project"
          body={`This permanently deletes "${project?.name ?? "this project"}" and every deployment in it -- stopping containers, removing volumes, and releasing ports. This cannot be undone.`}
          confirmLabel="Delete project"
          onClose={() => setShowDelete(false)}
          onConfirm={async () => {
            await api.del(`/api/projects/${projectId}`);
            window.location.href = "/";
          }}
        />
      )}
    </>
  );
}

interface GithubPATOption {
  id: number;
  label: string;
}

function LinkRepoModal({ projectId, onClose, onLinked }: { projectId: number; onClose: () => void; onLinked: () => void }) {
  const [pats, setPats] = useState<GithubPATOption[]>([]);
  const [patId, setPatId] = useState<number | "">("");
  const [owner, setOwner] = useState("");
  const [name, setName] = useState("");
  const [branch, setBranch] = useState("main");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<{ webhook_path: string; webhook_secret: string } | null>(null);

  useEffect(() => {
    api.get<GithubPATOption[]>("/api/github/pats").then((p) => setPats(p ?? [])).catch(() => setPats([]));
  }, []);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!patId) {
      setError("Add a GitHub Personal Access Token in Admin first.");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const res = await api.post<{ webhook_path: string; webhook_secret: string }>(`/api/projects/${projectId}/repo`, {
        github_pat_id: patId,
        repo_owner: owner,
        repo_name: name,
        default_branch: branch,
      });
      setResult(res);
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  };

  if (result) {
    return (
      <Modal title="Repo connected" onClose={onLinked}>
        <p>Paste these into the repo's GitHub settings &rarr; Webhooks &rarr; Add webhook. The secret is shown only once.</p>
        <div className="field">
          <label>Payload URL</label>
          <input className="input mono" readOnly value={`https://<your-host>${result.webhook_path}`} onFocus={(e) => e.target.select()} />
        </div>
        <div className="field">
          <label>Secret</label>
          <input className="input mono" readOnly value={result.webhook_secret} onFocus={(e) => e.target.select()} />
        </div>
        <div className="field-hint">Content type: application/json. Events: just the push event.</div>
        <div className="modal-actions">
          <button className="btn btn-primary" onClick={onLinked}>
            Done
          </button>
        </div>
      </Modal>
    );
  }

  return (
    <Modal title="Connect a GitHub repo" onClose={onClose}>
      {error && <div className="error-banner">{error}</div>}
      {pats.length === 0 && (
        <div className="error-banner">No GitHub tokens configured yet -- add one in Admin first.</div>
      )}
      <form onSubmit={submit}>
        <div className="field">
          <label htmlFor="repo-pat">Personal access token</label>
          <select id="repo-pat" className="input" value={patId} onChange={(e) => setPatId(Number(e.target.value))}>
            <option value="">Select a token...</option>
            {pats.map((p) => (
              <option key={p.id} value={p.id}>
                {p.label}
              </option>
            ))}
          </select>
        </div>
        <div className="form-row">
          <div className="field">
            <label htmlFor="repo-owner">Owner</label>
            <input id="repo-owner" className="input mono" required value={owner} onChange={(e) => setOwner(e.target.value)} />
          </div>
          <div className="field">
            <label htmlFor="repo-name">Repository</label>
            <input id="repo-name" className="input mono" required value={name} onChange={(e) => setName(e.target.value)} />
          </div>
        </div>
        <div className="field">
          <label htmlFor="repo-branch">Default branch</label>
          <input id="repo-branch" className="input mono" value={branch} onChange={(e) => setBranch(e.target.value)} />
        </div>
        <div className="modal-actions">
          <button type="button" className="btn" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className="btn btn-primary" disabled={busy}>
            {busy ? "Connecting..." : "Connect"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function errMsg(e: unknown): string {
  return e instanceof ApiError ? e.message : "Something went wrong";
}

type Strategy = "dockerfile" | "nixpacks" | "compose" | "image" | "static";

function CreateDeploymentModal({
  projectId,
  onClose,
  onCreated,
}: {
  projectId: number;
  onClose: () => void;
  onCreated: (id: number) => void;
}) {
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [strategy, setStrategy] = useState<Strategy>("dockerfile");
  const [gitBranch, setGitBranch] = useState("main");
  const [imageRef, setImageRef] = useState("");
  const [dockerfilePath, setDockerfilePath] = useState("");
  const [composePath, setComposePath] = useState("docker-compose.yml");
  const [staticBuildCommand, setStaticBuildCommand] = useState("");
  const [staticOutputDir, setStaticOutputDir] = useState("dist");
  const [rootPath, setRootPath] = useState(".");
  const [serviceName, setServiceName] = useState("web");
  const [internalPort, setInternalPort] = useState(3000);
  // Internal-only is the explicit default per the plan -- exposing a public
  // port is something the user opts into, not an accidental default.
  const [internalOnly, setInternalOnly] = useState(true);
  const [memoryMB, setMemoryMB] = useState(256);
  const [cpuCores, setCpuCores] = useState(0.5);
  const [healthPath, setHealthPath] = useState("/");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const payload: Record<string, unknown> = {
        name,
        slug,
        build_strategy: strategy,
        git_branch: gitBranch,
        image_ref: imageRef,
        root_path: rootPath,
        dockerfile_path: dockerfilePath,
        compose_path: composePath,
        static_build_command: staticBuildCommand,
        static_output_dir: staticOutputDir,
        image_retention_count: 5,
      };
      if (strategy === "static") {
        // No container ever runs for a static site -- just the service
        // name (required by the backend) and whether it gets a public port.
        payload.service = { name: serviceName, is_internal_only: internalOnly };
      } else if (strategy !== "compose") {
        payload.service = {
          name: serviceName,
          internal_port: internalPort,
          is_internal_only: internalOnly,
          cpu_limit_cores: cpuCores,
          memory_limit_mb: memoryMB,
          health_check_path: healthPath,
        };
      }
      const dep = await api.post<Deployment>(`/api/projects/${projectId}/deployments`, payload);
      onCreated(dep.id);
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal title="New deployment" onClose={onClose}>
      {error && <div className="error-banner">{error}</div>}
      <form onSubmit={submit}>
        <div className="form-row">
          <div className="field">
            <label htmlFor="dep-name">Name</label>
            <input
              id="dep-name"
              className="input"
              required
              value={name}
              onChange={(e) => {
                setName(e.target.value);
                if (!slug) setSlug(slugify(e.target.value));
              }}
            />
          </div>
          <div className="field">
            <label htmlFor="dep-slug">Slug</label>
            <input
              id="dep-slug"
              className="input mono"
              required
              value={slug}
              onChange={(e) => setSlug(slugify(e.target.value))}
            />
          </div>
        </div>

        <div className="field">
          <label htmlFor="dep-strategy">Build strategy</label>
          <select id="dep-strategy" className="input" value={strategy} onChange={(e) => setStrategy(e.target.value as Strategy)}>
            <option value="dockerfile">Dockerfile</option>
            <option value="nixpacks">Nixpacks (no Dockerfile needed)</option>
            <option value="compose">Docker Compose (multi-service)</option>
            <option value="image">Existing image</option>
            <option value="static">Static site (build + serve files, no container)</option>
          </select>
        </div>

        {strategy === "image" ? (
          <div className="field">
            <label htmlFor="dep-image-ref">Image reference</label>
            <input
              id="dep-image-ref"
              className="input mono"
              placeholder="nginx:alpine"
              value={imageRef}
              onChange={(e) => setImageRef(e.target.value)}
            />
          </div>
        ) : (
          <>
            <div className="form-row">
              <div className="field">
                <label htmlFor="dep-branch">Branch</label>
                <input id="dep-branch" className="input mono" value={gitBranch} onChange={(e) => setGitBranch(e.target.value)} />
              </div>
              <div className="field">
                <label htmlFor="dep-root-path">Root path</label>
                <input id="dep-root-path" className="input mono" value={rootPath} onChange={(e) => setRootPath(e.target.value)} />
              </div>
            </div>
            {strategy === "dockerfile" && (
              <div className="field">
                <label htmlFor="dep-dockerfile-path">Dockerfile path (optional)</label>
                <input
                  id="dep-dockerfile-path"
                  className="input mono"
                  placeholder="Dockerfile"
                  value={dockerfilePath}
                  onChange={(e) => setDockerfilePath(e.target.value)}
                />
              </div>
            )}
            {strategy === "compose" && (
              <div className="field">
                <label htmlFor="dep-compose-path">Compose file path</label>
                <input
                  id="dep-compose-path"
                  className="input mono"
                  value={composePath}
                  onChange={(e) => setComposePath(e.target.value)}
                />
              </div>
            )}
            {strategy === "static" && (
              <>
                <div className="field">
                  <label htmlFor="dep-static-build-command">Build command (optional)</label>
                  <input
                    id="dep-static-build-command"
                    className="input mono"
                    placeholder="npm run build -- leave blank if the repo is already pre-built HTML/CSS/JS"
                    value={staticBuildCommand}
                    onChange={(e) => setStaticBuildCommand(e.target.value)}
                  />
                </div>
                <div className="field">
                  <label htmlFor="dep-static-output-dir">Output directory</label>
                  <input
                    id="dep-static-output-dir"
                    className="input mono"
                    placeholder="dist"
                    value={staticOutputDir}
                    onChange={(e) => setStaticOutputDir(e.target.value)}
                  />
                  <div className="field-hint">
                    Relative to the build's working directory (or to root path above, if there's no build command).
                  </div>
                </div>
              </>
            )}
          </>
        )}

        {strategy !== "compose" && (
          <>
            <div className="card-title" style={{ marginTop: 18 }}>
              Service
            </div>
            <div className="field">
              <label htmlFor="dep-service-name">Service name</label>
              <input
                id="dep-service-name"
                className="input mono"
                value={serviceName}
                onChange={(e) => setServiceName(e.target.value)}
              />
            </div>
            {strategy !== "static" && (
              <>
                <div className="form-row">
                  <div className="field">
                    <label htmlFor="dep-internal-port">Internal port</label>
                    <input
                      id="dep-internal-port"
                      className="input"
                      type="number"
                      value={internalPort}
                      onChange={(e) => setInternalPort(Number(e.target.value))}
                    />
                  </div>
                  <div className="field">
                    <label htmlFor="dep-memory">Memory limit (MB)</label>
                    <input
                      id="dep-memory"
                      className="input"
                      type="number"
                      value={memoryMB}
                      onChange={(e) => setMemoryMB(Number(e.target.value))}
                    />
                  </div>
                </div>
                <div className="form-row">
                  <div className="field">
                    <label htmlFor="dep-cpu">CPU cores</label>
                    <input
                      id="dep-cpu"
                      className="input"
                      type="number"
                      step="0.1"
                      value={cpuCores}
                      onChange={(e) => setCpuCores(Number(e.target.value))}
                    />
                  </div>
                </div>
                <div className="field">
                  <label htmlFor="dep-health-path">Health check path (optional)</label>
                  <input id="dep-health-path" className="input mono" value={healthPath} onChange={(e) => setHealthPath(e.target.value)} />
                </div>
              </>
            )}
            <div className="field">
              <label style={{ display: "flex", alignItems: "center", gap: 8 }}>
                <input type="checkbox" checked={internalOnly} onChange={(e) => setInternalOnly(e.target.checked)} />
                Internal only (never expose a public port)
              </label>
              {strategy === "static" && (
                <div className="field-hint">
                  A static site has no container to run or health-check -- Caddy serves the built files directly once this is off.
                </div>
              )}
              <div className="field-hint">This is the explicit default choice Mangrove asks for at deploy time -- flip it off to expose the app publicly.</div>
            </div>
          </>
        )}

        <div className="modal-actions">
          <button type="button" className="btn" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className="btn btn-primary" disabled={busy}>
            {busy ? "Creating..." : "Create deployment"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
