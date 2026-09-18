import { useEffect, useState, type FormEvent } from "react";
import { api, ApiError, type AuditEvent, type WorkspaceMember, type WorkspaceRole } from "../api";
import { Link, useRouter } from "../router";
import { Modal, useModalClose } from "../components/Modal";
import { AuditLogTable } from "../components/AuditLogTable";
import { useWorkspaces } from "../workspaceContext";
import { LedgerIcon, MangroveIcon, PlusIcon, TrashIcon, UserIcon } from "../icons";

export function WorkspacesPage() {
  const { navigate } = useRouter();
  const { workspaces, setActiveWorkspaceId, reload } = useWorkspaces();
  const [error, setError] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [membersFor, setMembersFor] = useState<{ id: number; name: string } | null>(null);
  const [activityFor, setActivityFor] = useState<{ id: number; name: string } | null>(null);

  const view = (id: number) => {
    setActiveWorkspaceId(id);
    navigate("/");
  };

  const remove = async (id: number, name: string) => {
    if (!confirm(`Delete workspace "${name}"? Its projects will be moved to the default workspace.`)) return;
    try {
      await api.del(`/api/workspaces/${id}`);
      reload();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to delete workspace");
    }
  };

  return (
    <>
      <div className="breadcrumb">
        <Link to="/">Projects</Link> / Workspaces
      </div>
      <div className="page-header">
        <div>
          <h1>Workspaces</h1>
          <p>Group projects by environment (e.g. production, staging) or team. Pick one from the sidebar's station switcher to scope Projects.</p>
        </div>
        <button className="btn btn-primary" onClick={() => setShowCreate(true)}>
          <PlusIcon /> New workspace
        </button>
      </div>

      {error && <div className="error-banner">{error}</div>}

      {workspaces.length === 0 ? (
        <div className="center-loading">
          <div className="spinner" />
        </div>
      ) : (
        <div className="card">
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
                      href="/"
                      onClick={(e) => {
                        e.preventDefault();
                        view(w.workspace.id);
                      }}
                      style={{ display: "inline-flex", alignItems: "center", gap: 7 }}
                    >
                      <MangroveIcon style={{ width: 14, height: 14, color: "var(--brass-dim)" }} />
                      {w.workspace.name}
                    </a>
                  </td>
                  <td className="mono text-dim">{w.workspace.slug}</td>
                  <td className="text-dim">{w.project_count}</td>
                  <td className="text-right">
                    {!!w.your_role && (
                      <button
                        className="btn btn-sm"
                        style={{ marginRight: 8 }}
                        onClick={() => setActivityFor({ id: w.workspace.id, name: w.workspace.name })}
                      >
                        <LedgerIcon /> Activity
                      </button>
                    )}
                    {w.your_role === "admin" && (
                      <button
                        className="btn btn-sm"
                        style={{ marginRight: 8 }}
                        onClick={() => setMembersFor({ id: w.workspace.id, name: w.workspace.name })}
                      >
                        <UserIcon /> Members
                      </button>
                    )}
                    {w.workspace.id !== 1 && w.your_role === "admin" && (
                      <button className="btn btn-sm btn-danger" onClick={() => remove(w.workspace.id, w.workspace.name)}>
                        <TrashIcon /> Delete
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
            reload();
          }}
        />
      )}

      {membersFor && (
        <ManageMembersModal workspaceId={membersFor.id} workspaceName={membersFor.name} onClose={() => setMembersFor(null)} />
      )}

      {activityFor && <ActivityModal workspaceId={activityFor.id} workspaceName={activityFor.name} onClose={() => setActivityFor(null)} />}
    </>
  );
}

// ActivityModal is viewer+ (any member can open it, matching the
// GET /api/workspaces/{id}/audit-log route's own gate) -- accountability
// for who deployed/deleted/changed access is exactly the kind of thing a
// workspace's own members should be able to see, not just its admins.
function ActivityModal({ workspaceId, workspaceName, onClose }: { workspaceId: number; workspaceName: string; onClose: () => void }) {
  const requestClose = useModalClose();
  const [events, setEvents] = useState<AuditEvent[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .get<AuditEvent[]>(`/api/workspaces/${workspaceId}/audit-log`)
      .then((e) => setEvents(e ?? []))
      .catch((e) => setError(e instanceof ApiError ? e.message : "Failed to load activity"));
  }, [workspaceId]);

  return (
    <Modal title={`Activity in "${workspaceName}"`} onClose={onClose}>
      {error && <div className="error-banner">{error}</div>}
      <AuditLogTable events={events} />
      <div className="modal-actions">
        <button type="button" className="btn" onClick={requestClose}>
          Close
        </button>
      </div>
    </Modal>
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

const ROLES: WorkspaceRole[] = ["viewer", "editor", "admin"];

// ManageMembersModal is admin-only, both server-side (RequireWorkspaceRole
// on /api/workspaces/{id}/members, see router.go) and here (WorkspacesPage
// only ever opens it for a workspace where your_role === "admin"). Adding
// a member is by exact email match against an *existing* org account --
// deliberately not a picker over the full user roster, which stays
// reserved for global owners (docs/multi-user.md).
function ManageMembersModal({ workspaceId, workspaceName, onClose }: { workspaceId: number; workspaceName: string; onClose: () => void }) {
  const requestClose = useModalClose();
  const [members, setMembers] = useState<WorkspaceMember[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<WorkspaceRole>("editor");
  const [adding, setAdding] = useState(false);

  const load = () => {
    api
      .get<WorkspaceMember[]>(`/api/workspaces/${workspaceId}/members`)
      .then((m) => setMembers(m ?? []))
      .catch((e) => setError(e instanceof ApiError ? e.message : "Failed to load members"));
  };
  useEffect(load, [workspaceId]);

  const add = async (e: FormEvent) => {
    e.preventDefault();
    if (!email.trim()) return;
    setAdding(true);
    setError(null);
    try {
      await api.post(`/api/workspaces/${workspaceId}/members`, { email: email.trim(), role });
      setEmail("");
      load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to add member");
    } finally {
      setAdding(false);
    }
  };

  const setMemberRole = async (userId: number, newRole: WorkspaceRole) => {
    try {
      await api.put(`/api/workspaces/${workspaceId}/members/${userId}`, { role: newRole });
      load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to change role");
    }
  };

  const remove = async (userId: number) => {
    try {
      await api.del(`/api/workspaces/${workspaceId}/members/${userId}`);
      load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to remove member");
    }
  };

  return (
    <Modal title={`Members of "${workspaceName}"`} onClose={onClose}>
      {error && <div className="error-banner">{error}</div>}
      {members && members.length > 0 && (
        <div className="kv-list" style={{ marginBottom: 16 }}>
          {members.map((m) => (
            <div className="kv-row" key={m.user_id}>
              <span className="kv-key">{m.email}</span>
              <select className="input mono" style={{ width: "auto" }} value={m.role} onChange={(e) => setMemberRole(m.user_id, e.target.value as WorkspaceRole)}>
                {ROLES.map((r) => (
                  <option key={r} value={r}>
                    {r}
                  </option>
                ))}
              </select>
              <button className="btn btn-sm btn-danger" onClick={() => remove(m.user_id)}>
                <TrashIcon /> Remove
              </button>
            </div>
          ))}
        </div>
      )}
      <form onSubmit={add} className="form-row" style={{ alignItems: "flex-end" }}>
        <div className="field" style={{ marginBottom: 0, flex: 1 }}>
          <label htmlFor="member-email">Add an existing account by email</label>
          <input
            id="member-email"
            className="input"
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="teammate@example.com"
          />
        </div>
        <div className="field" style={{ marginBottom: 0 }}>
          <label htmlFor="member-role">Role</label>
          <select id="member-role" className="input mono" value={role} onChange={(e) => setRole(e.target.value as WorkspaceRole)}>
            {ROLES.map((r) => (
              <option key={r} value={r}>
                {r}
              </option>
            ))}
          </select>
        </div>
        <button className="btn" type="submit" disabled={adding || !email.trim()}>
          <PlusIcon /> Add
        </button>
      </form>
      <div className="modal-actions">
        <button type="button" className="btn" onClick={requestClose}>
          Close
        </button>
      </div>
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
