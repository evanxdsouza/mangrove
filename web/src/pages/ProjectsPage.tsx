import { useEffect, useState, type FormEvent } from "react";
import { api, ApiError, type Project, type Workspace } from "../api";
import { Link, useRouter } from "../router";
import { Modal, useModalClose } from "../components/Modal";

export function ProjectsPage() {
  const { navigate } = useRouter();
  const [projects, setProjects] = useState<Project[] | null>(null);
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);

  const wsParam = new URLSearchParams(window.location.search).get("workspace");
  const activeWorkspace = wsParam ? Number(wsParam) : null;

  const load = () => {
    const q = activeWorkspace ? `?workspace_id=${activeWorkspace}` : "";
    api
      .get<Project[]>(`/api/projects${q}`)
      .then((p) => setProjects(p ?? []))
      .catch((e) => setError(e instanceof ApiError ? e.message : "Failed to load projects"));
  };

  useEffect(() => {
    api
      .get<Workspace[]>("/api/workspaces")
      .then((ws) => setWorkspaces(ws?.map((x: any) => x.workspace ?? x) ?? []))
      .catch(() => setWorkspaces([]));
  }, []);

  useEffect(load, [activeWorkspace]);

  const workspaceName = (id: number) => workspaces.find((w) => w.id === id)?.name ?? null;

  return (
    <>
      <div className="page-header">
        <div>
          <h1>Projects</h1>
          <p>Group deployments by app or site.</p>
        </div>
        <div className="flex gap-8">
          {activeWorkspace != null && (
            <button className="btn" onClick={() => navigate("/projects")}>
              All workspaces
            </button>
          )}
          <button className="btn btn-primary" onClick={() => setShowCreate(true)}>
            + New project
          </button>
        </div>
      </div>

      {error && <div className="error-banner">{error}</div>}

      {projects === null ? (
        <div className="center-loading">
          <div className="spinner" />
        </div>
      ) : projects.length === 0 ? (
        <div className="card empty-state">
          {activeWorkspace != null ? "No projects in this workspace yet." : "No projects yet. Create one to deploy your first app."}
        </div>
      ) : (
        <div className="card" style={{ padding: 0 }}>
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Slug</th>
                <th>Workspace</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {projects.map((p) => (
                <tr key={p.id} className="row-link" onClick={() => (window.location.href = `/projects/${p.id}`)}>
                  <td>
                    <Link to={`/projects/${p.id}`}>{p.name}</Link>
                  </td>
                  <td className="mono text-dim">{p.slug}</td>
                  <td className="text-dim">
                    {p.workspace_name ? (
                      <Link to={`/projects?workspace=${p.workspace_id}`}>{p.workspace_name}</Link>
                    ) : (
                      workspaceName(p.workspace_id) ?? <span className="text-faint">—</span>
                    )}
                  </td>
                  <td className="text-dim">{new Date(p.created_at).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showCreate && (
        <CreateProjectModal
          workspaces={workspaces}
          defaultWorkspaceId={activeWorkspace ?? undefined}
          onClose={() => setShowCreate(false)}
          onCreated={(id) => {
            setShowCreate(false);
            navigate(`/projects/${id}`);
          }}
        />
      )}
    </>
  );
}

function CreateProjectModal({
  workspaces,
  defaultWorkspaceId,
  onClose,
  onCreated,
}: {
  workspaces: Workspace[];
  defaultWorkspaceId?: number;
  onClose: () => void;
  onCreated: (id: number) => void;
}) {
  const requestClose = useModalClose();
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [description, setDescription] = useState("");
  const [workspaceId, setWorkspaceId] = useState<number>(defaultWorkspaceId ?? 1);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const project = await api.post<Project>("/api/projects", { name, slug, description, workspace_id: workspaceId });
      onCreated(project.id);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to create project");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal title="New project" onClose={onClose}>
      {error && <div className="error-banner">{error}</div>}
      <form onSubmit={submit}>
        <div className="field">
          <label htmlFor="project-name">Name</label>
          <input
            id="project-name"
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
          <label htmlFor="project-slug">Slug</label>
          <input
            id="project-slug"
            className="input mono"
            required
            value={slug}
            onChange={(e) => setSlug(slugify(e.target.value))}
          />
        </div>
        <div className="field">
          <label htmlFor="project-description">Description (optional)</label>
          <input id="project-description" className="input" value={description} onChange={(e) => setDescription(e.target.value)} />
        </div>
        <div className="field">
          <label htmlFor="project-workspace">Workspace</label>
          <select id="project-workspace" className="input" value={workspaceId} onChange={(e) => setWorkspaceId(Number(e.target.value))}>
            {workspaces.map((w) => (
              <option key={w.id} value={w.id}>
                {w.name}
              </option>
            ))}
          </select>
        </div>
        <div className="modal-actions">
          <button type="button" className="btn" onClick={requestClose}>
            Cancel
          </button>
          <button type="submit" className="btn btn-primary" disabled={busy}>
            {busy ? "Creating..." : "Create project"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

export function slugify(s: string): string {
  return s
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/(^-|-$)/g, "");
}
