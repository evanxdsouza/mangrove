import { useEffect, useState, type FormEvent } from "react";
import { api, ApiError, type Deployment, type Project } from "../api";
import { Link, useRouter } from "../router";
import { Modal, useModalClose } from "../components/Modal";
import { useWorkspaces } from "../workspaceContext";
import { StatusPill, worstStatus } from "../components/StatusPill";
import { EmptyLedgerIcon, PlusIcon } from "../icons";

interface ProjectStatus {
  worst: string | null;
  count: number;
}

export function ProjectsPage() {
  const { navigate } = useRouter();
  const { workspaces, activeWorkspaceId, setActiveWorkspaceId, reload: reloadWorkspaces } = useWorkspaces();
  const [projects, setProjects] = useState<Project[] | null>(null);
  const [status, setStatus] = useState<Record<number, ProjectStatus>>({});
  const [error, setError] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);

  const load = () => {
    const q = activeWorkspaceId ? `?workspace_id=${activeWorkspaceId}` : "";
    api
      .get<Project[]>(`/api/projects${q}`)
      .then((p) => {
        const list = p ?? [];
        setProjects(list);
        // One request per project, same pattern SimpleAppsPage already uses
        // to flatten every deployment -- the ledger's status beacon is a
        // real read of what's actually deployed, not decoration.
        Promise.all(
          list.map((project) =>
            api
              .get<Deployment[]>(`/api/projects/${project.id}/deployments`)
              .then((deps): [number, ProjectStatus] => [
                project.id,
                { worst: worstStatus((deps ?? []).map((d) => d.status)), count: (deps ?? []).length },
              ])
              .catch((): [number, ProjectStatus] => [project.id, { worst: null, count: 0 }]),
          ),
        ).then((entries) => setStatus(Object.fromEntries(entries)));
      })
      .catch((e) => setError(e instanceof ApiError ? e.message : "Failed to load projects"));
  };

  useEffect(() => {
    load();
    const interval = setInterval(load, 4000);
    return () => clearInterval(interval);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeWorkspaceId]);

  const activeWorkspaceName = workspaces.find((w) => w.workspace.id === activeWorkspaceId)?.workspace.name ?? null;

  return (
    <>
      <div className="page-header">
        <div>
          <h1>{activeWorkspaceName ? `${activeWorkspaceName} projects` : "Projects"}</h1>
          <p>
            {activeWorkspaceName
              ? "Deployments grouped by app or site, scoped to this workspace."
              : "Group deployments by app or site, across every workspace."}
          </p>
        </div>
        <div className="flex gap-8">
          {activeWorkspaceId != null && (
            <button className="btn" onClick={() => setActiveWorkspaceId(null)}>
              All workspaces
            </button>
          )}
          <button className="btn btn-primary" onClick={() => setShowCreate(true)}>
            <PlusIcon /> New project
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
          <EmptyLedgerIcon />
          <p>{activeWorkspaceId != null ? "No projects in this workspace yet." : "No projects yet."}</p>
          <div className="field-hint">Create one to deploy your first app.</div>
        </div>
      ) : (
        <div className="card">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Status</th>
                <th>Slug</th>
                <th>Workspace</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {projects.map((p) => {
                const s = status[p.id];
                return (
                <tr key={p.id} className="row-link" onClick={() => (window.location.href = `/projects/${p.id}`)}>
                  <td>
                    <Link to={`/projects/${p.id}`}>{p.name}</Link>
                  </td>
                  <td>
                    {s == null ? (
                      <span className="text-faint mono" style={{ fontSize: 12 }}>...</span>
                    ) : s.worst == null ? (
                      <span className="text-faint">no deployments</span>
                    ) : (
                      <span className="flex gap-8" style={{ alignItems: "center" }}>
                        <StatusPill status={s.worst} />
                        {s.count > 1 && <span className="text-faint mono" style={{ fontSize: 11.5 }}>&times;{s.count}</span>}
                      </span>
                    )}
                  </td>
                  <td className="mono text-dim">{p.slug}</td>
                  <td className="text-dim">
                    {p.workspace_name ? (
                      <a
                        href={`/`}
                        onClick={(e) => {
                          e.preventDefault();
                          e.stopPropagation();
                          setActiveWorkspaceId(p.workspace_id);
                        }}
                      >
                        {p.workspace_name}
                      </a>
                    ) : (
                      <span className="text-faint">—</span>
                    )}
                  </td>
                  <td className="text-dim">{new Date(p.created_at).toLocaleString()}</td>
                </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {showCreate && (
        <CreateProjectModal
          workspaces={workspaces.map((w) => w.workspace)}
          defaultWorkspaceId={activeWorkspaceId ?? undefined}
          onClose={() => setShowCreate(false)}
          onCreated={(id) => {
            setShowCreate(false);
            reloadWorkspaces();
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
  workspaces: { id: number; name: string }[];
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
