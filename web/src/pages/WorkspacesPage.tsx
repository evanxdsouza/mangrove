import { useEffect, useState, type FormEvent } from "react";
import { api, ApiError, type WorkspaceProjectCount } from "../api";
import { useRouter } from "../router";
import { Modal, useModalClose } from "../components/Modal";

export function WorkspacesPage() {
  const { navigate } = useRouter();
  const [workspaces, setWorkspaces] = useState<WorkspaceProjectCount[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);

  const load = () => {
    api
      .get<WorkspaceProjectCount[]>("/api/workspaces")
      .then((w) => setWorkspaces(w ?? []))
      .catch((e) => setError(e instanceof ApiError ? e.message : "Failed to load workspaces"));
  };

  useEffect(load, []);

  const remove = async (id: number, name: string) => {
    if (!confirm(`Delete workspace "${name}"? Its projects will be moved to the default workspace.`)) return;
    try {
      await api.del(`/api/workspaces/${id}`);
      load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to delete workspace");
    }
  };

  return (
    <>
      <div className="page-header">
        <div>
          <h1>Workspaces</h1>
          <p>Group projects by environment (e.g. production, staging) or team.</p>
        </div>
        <button className="btn btn-primary" onClick={() => setShowCreate(true)}>
          + New workspace
        </button>
      </div>

      {error && <div className="error-banner">{error}</div>}

      {workspaces === null ? (
        <div className="center-loading">
          <div className="spinner" />
        </div>
      ) : workspaces.length === 0 ? (
        <div className="card empty-state">No workspaces yet.</div>
      ) : (
        <div className="card" style={{ padding: 0 }}>
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Slug</th>
                <th>Projects</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {workspaces.map((w) => (
                <tr key={w.workspace.id}>
                  <td>
                    <a
                      href={`/projects?workspace=${w.workspace.id}`}
                      onClick={(e) => {
                        e.preventDefault();
                        navigate(`/projects?workspace=${w.workspace.id}`);
                      }}
                    >
                      {w.workspace.name}
                    </a>
                  </td>
                  <td className="mono text-dim">{w.workspace.slug}</td>
                  <td className="text-dim">{w.project_count}</td>
                  <td className="text-right">
                    {w.workspace.id !== 1 && (
                      <button className="btn btn-sm" onClick={() => remove(w.workspace.id, w.workspace.name)}>
                        Delete
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showCreate && (
        <CreateWorkspaceModal
          onClose={() => setShowCreate(false)}
          onCreated={() => {
            setShowCreate(false);
            load();
          }}
        />
      )}
    </>
  );
}

function CreateWorkspaceModal({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const requestClose = useModalClose();
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api.post("/api/workspaces", { name, slug });
      onCreated();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to create workspace");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal title="New workspace" onClose={onClose}>
      {error && <div className="error-banner">{error}</div>}
      <form onSubmit={submit}>
        <div className="field">
          <label htmlFor="ws-name">Name</label>
          <input
            id="ws-name"
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
          <label htmlFor="ws-slug">Slug</label>
          <input id="ws-slug" className="input mono" required value={slug} onChange={(e) => setSlug(slugify(e.target.value))} />
        </div>
        <div className="modal-actions">
          <button type="button" className="btn" onClick={requestClose}>
            Cancel
          </button>
          <button type="submit" className="btn btn-primary" disabled={busy}>
            {busy ? "Creating..." : "Create workspace"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function slugify(s: string): string {
  return s
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/(^-|-$)/g, "");
}
