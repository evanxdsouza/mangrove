import { useEffect, useState, type FormEvent } from "react";
import { api, ApiError, type Project } from "../api";
import { Link, useRouter } from "../router";
import { Modal } from "../components/Modal";

export function ProjectsPage() {
  const { navigate } = useRouter();
  const [projects, setProjects] = useState<Project[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);

  const load = () => {
    api
      .get<Project[]>("/api/projects")
      .then((p) => setProjects(p ?? []))
      .catch((e) => setError(e instanceof ApiError ? e.message : "Failed to load projects"));
  };

  useEffect(load, []);

  return (
    <>
      <div className="page-header">
        <div>
          <h1>Projects</h1>
          <p>Group deployments by app or site.</p>
        </div>
        <button className="btn btn-primary" onClick={() => setShowCreate(true)}>
          + New project
        </button>
      </div>

      {error && <div className="error-banner">{error}</div>}

      {projects === null ? (
        <div className="center-loading">
          <div className="spinner" />
        </div>
      ) : projects.length === 0 ? (
        <div className="card empty-state">No projects yet. Create one to deploy your first app.</div>
      ) : (
        <div className="card" style={{ padding: 0 }}>
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Slug</th>
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
                  <td className="text-dim">{new Date(p.created_at).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showCreate && (
        <CreateProjectModal
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

function CreateProjectModal({ onClose, onCreated }: { onClose: () => void; onCreated: (id: number) => void }) {
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [description, setDescription] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const project = await api.post<Project>("/api/projects", { name, slug, description });
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
        <div className="modal-actions">
          <button type="button" className="btn" onClick={onClose}>
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
