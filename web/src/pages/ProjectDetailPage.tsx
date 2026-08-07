import { useEffect, useState, type FormEvent } from "react";
import { api, ApiError, type Deployment, type Project } from "../api";
import { Link } from "../router";
import { Modal } from "../components/Modal";
import { StatusPill } from "../components/StatusPill";
import { slugify } from "./ProjectsPage";

export function ProjectDetailPage({ projectId }: { projectId: number }) {
  const [project, setProject] = useState<Project | null>(null);
  const [deployments, setDeployments] = useState<Deployment[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);

  const load = () => {
    api.get<Project>(`/api/projects/${projectId}`).then(setProject).catch((e) => setError(errMsg(e)));
    api
      .get<Deployment[]>(`/api/projects/${projectId}/deployments`)
      .then((d) => setDeployments(d ?? []))
      .catch((e) => setError(errMsg(e)));
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
        <button className="btn btn-primary" onClick={() => setShowCreate(true)}>
          + New deployment
        </button>
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

      {showCreate && (
        <CreateDeploymentModal
          projectId={projectId}
          onClose={() => setShowCreate(false)}
          onCreated={(id) => {
            window.location.href = `/projects/${projectId}/deployments/${id}`;
          }}
        />
      )}
    </>
  );
}

function errMsg(e: unknown): string {
  return e instanceof ApiError ? e.message : "Something went wrong";
}

type Strategy = "dockerfile" | "nixpacks" | "compose" | "image";

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
        image_retention_count: 5,
      };
      if (strategy !== "compose") {
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
          </>
        )}

        {strategy !== "compose" && (
          <>
            <div className="card-title" style={{ marginTop: 18 }}>
              Service
            </div>
            <div className="form-row">
              <div className="field">
                <label htmlFor="dep-service-name">Service name</label>
                <input
                  id="dep-service-name"
                  className="input mono"
                  value={serviceName}
                  onChange={(e) => setServiceName(e.target.value)}
                />
              </div>
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
            </div>
            <div className="form-row">
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
            <div className="field">
              <label style={{ display: "flex", alignItems: "center", gap: 8 }}>
                <input type="checkbox" checked={internalOnly} onChange={(e) => setInternalOnly(e.target.checked)} />
                Internal only (never expose a public port)
              </label>
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
