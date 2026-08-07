import { useEffect, useState, type FormEvent } from "react";
import { api, ApiError } from "../api";

interface ResourceBudget {
  memory_allocated_mb: number;
  memory_ceiling_mb: number;
  running_containers: number;
  disk_total_gb: number;
  disk_used_gb: number;
}

interface PortEntry {
  id: number;
  port: number;
  status: string;
  allocation_type: string;
  service_id?: number;
  note: string;
}

interface SessionEntry {
  id: number;
  user_email: string;
  created_at: string;
  expires_at: string;
  ip_address: string;
  user_agent: string;
}

interface NodeEntry {
  id: number;
  name: string;
  kind: string;
  address?: string;
  status: string;
  is_default: boolean;
}

interface NotificationEntry {
  id: number;
  type: string;
  channel: string;
  recipient: string;
  subject: string;
  status: string;
  sent_at: string;
}

interface GithubPATEntry {
  id: number;
  label: string;
  created_at: string;
  last_used_at?: string;
}

export function AdminPage() {
  const [budget, setBudget] = useState<ResourceBudget | null>(null);
  const [ports, setPorts] = useState<PortEntry[] | null>(null);
  const [sessions, setSessions] = useState<SessionEntry[] | null>(null);
  const [nodes, setNodes] = useState<NodeEntry[] | null>(null);
  const [notifications, setNotifications] = useState<NotificationEntry[] | null>(null);
  const [pats, setPats] = useState<GithubPATEntry[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pruneResult, setPruneResult] = useState<string | null>(null);

  const load = () => {
    api.get<ResourceBudget>("/api/admin/resource-budget").then(setBudget).catch((e) => setError(errMsg(e)));
    api.get<PortEntry[]>("/api/admin/ports").then((p) => setPorts(p ?? [])).catch((e) => setError(errMsg(e)));
    api.get<SessionEntry[]>("/api/admin/sessions").then((s) => setSessions(s ?? [])).catch((e) => setError(errMsg(e)));
    api.get<NodeEntry[]>("/api/admin/nodes").then((n) => setNodes(n ?? [])).catch((e) => setError(errMsg(e)));
    api
      .get<NotificationEntry[]>("/api/admin/notifications")
      .then((n) => setNotifications(n ?? []))
      .catch((e) => setError(errMsg(e)));
    api.get<GithubPATEntry[]>("/api/github/pats").then((p) => setPats(p ?? [])).catch((e) => setError(errMsg(e)));
  };

  useEffect(load, []);

  const revokeSession = async (id: number) => {
    await api.del(`/api/admin/sessions/${id}`);
    load();
  };

  const releasePort = async (port: number) => {
    await api.del(`/api/admin/ports/${port}`);
    load();
  };

  const deletePat = async (id: number) => {
    await api.del(`/api/github/pats/${id}`);
    load();
  };

  const runPrune = async () => {
    setPruneResult(null);
    try {
      const res = await api.post<{ images_removed: number; space_reclaimed_mb: number }>("/api/admin/prune");
      setPruneResult(`Removed ${res.images_removed} image(s), reclaimed ~${res.space_reclaimed_mb}MB.`);
      load();
    } catch (e) {
      setError(errMsg(e));
    }
  };

  return (
    <>
      <div className="page-header">
        <div>
          <h1>Admin</h1>
          <p>System-wide operations across every project.</p>
        </div>
      </div>

      {error && <div className="error-banner">{error}</div>}

      <div className="card">
        <div className="card-title">Resource budget</div>
        {budget ? (
          <div className="grid grid-3">
            <StatTile
              label="Memory allocated"
              value={`${budget.memory_allocated_mb} / ${budget.memory_ceiling_mb} MB`}
              fraction={budget.memory_allocated_mb / Math.max(budget.memory_ceiling_mb, 1)}
            />
            <div className="stat-tile">
              <div className="stat-value">{budget.running_containers}</div>
              <div className="stat-label">Running containers</div>
            </div>
            {budget.disk_total_gb > 0 && (
              <StatTile
                label="Disk used"
                value={`${budget.disk_used_gb.toFixed(1)} / ${budget.disk_total_gb.toFixed(1)} GB`}
                fraction={budget.disk_used_gb / Math.max(budget.disk_total_gb, 1)}
              />
            )}
          </div>
        ) : (
          <div className="text-dim">Loading...</div>
        )}
      </div>

      <div className="grid grid-2">
        <div className="card">
          <div className="flex-between" style={{ marginBottom: 14 }}>
            <div className="card-title" style={{ margin: 0 }}>
              Port registry
            </div>
          </div>
          {ports === null ? (
            <div className="text-dim">Loading...</div>
          ) : ports.length === 0 ? (
            <div className="text-dim">No ports allocated yet.</div>
          ) : (
            <table>
              <thead>
                <tr>
                  <th>Port</th>
                  <th>Type</th>
                  <th>Note</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {ports.map((p) => (
                  <tr key={p.id}>
                    <td className="mono">{p.port}</td>
                    <td className="text-dim">{p.allocation_type}</td>
                    <td className="text-dim">{p.note || "—"}</td>
                    <td>
                      {p.allocation_type !== "system" && (
                        <button className="btn btn-sm btn-danger" onClick={() => releasePort(p.port)}>
                          Release
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
          <ReservePortForm onReserved={load} />
        </div>

        <div className="card">
          <div className="card-title">Active sessions</div>
          {sessions === null ? (
            <div className="text-dim">Loading...</div>
          ) : sessions.length === 0 ? (
            <div className="text-dim">No active sessions.</div>
          ) : (
            <table>
              <thead>
                <tr>
                  <th>User</th>
                  <th>IP</th>
                  <th>Created</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {sessions.map((s) => (
                  <tr key={s.id}>
                    <td>{s.user_email}</td>
                    <td className="mono text-dim">{s.ip_address}</td>
                    <td className="text-dim">{new Date(s.created_at).toLocaleString()}</td>
                    <td>
                      <button className="btn btn-sm btn-danger" onClick={() => revokeSession(s.id)}>
                        Revoke
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>

      <div className="grid grid-2">
        <div className="card">
          <div className="card-title">Nodes</div>
          {nodes === null ? (
            <div className="text-dim">Loading...</div>
          ) : (
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Kind</th>
                  <th>Status</th>
                </tr>
              </thead>
              <tbody>
                {nodes.map((n) => (
                  <tr key={n.id}>
                    <td>
                      {n.name} {n.is_default && <span className="pill pill-gray">default</span>}
                    </td>
                    <td className="text-dim">{n.kind}</td>
                    <td>
                      <span className="pill pill-green">
                        <span className="pill-dot" />
                        {n.status}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
          <div className="field-hint" style={{ marginTop: 12 }}>
            Single-node only for now. A future remote worker would appear here.
          </div>
        </div>

        <div className="card">
          <div className="card-title">Image / build cache pruning</div>
          <p className="text-dim" style={{ marginTop: 0 }}>
            Runs automatically on a schedule; trigger it manually here too.
          </p>
          <button className="btn" onClick={runPrune}>
            Prune now
          </button>
          {pruneResult && <div className="field-hint">{pruneResult}</div>}
        </div>
      </div>

      <div className="card">
        <div className="card-title">GitHub Personal Access Tokens</div>
        <p className="text-dim" style={{ marginTop: 0 }}>
          Used to clone private repos for git-backed deployments. Stored encrypted; the token itself is never shown again after creation.
        </p>
        {pats === null ? (
          <div className="text-dim">Loading...</div>
        ) : pats.length === 0 ? (
          <div className="text-dim">No tokens added yet.</div>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Label</th>
                <th>Added</th>
                <th>Last used</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {pats.map((p) => (
                <tr key={p.id}>
                  <td>{p.label}</td>
                  <td className="text-dim">{new Date(p.created_at).toLocaleString()}</td>
                  <td className="text-dim">{p.last_used_at ? new Date(p.last_used_at).toLocaleString() : "never"}</td>
                  <td>
                    <button className="btn btn-sm btn-danger" onClick={() => deletePat(p.id)}>
                      Remove
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        <AddPatForm onAdded={load} />
      </div>

      <div className="card">
        <div className="card-title">Notifications</div>
        {notifications === null ? (
          <div className="text-dim">Loading...</div>
        ) : notifications.length === 0 ? (
          <div className="text-dim">No notifications sent yet.</div>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Type</th>
                <th>Recipient</th>
                <th>Status</th>
                <th>Sent</th>
              </tr>
            </thead>
            <tbody>
              {notifications.map((n) => (
                <tr key={n.id}>
                  <td>{n.subject}</td>
                  <td className="text-dim">{n.recipient}</td>
                  <td>
                    <span className={`pill ${n.status === "sent" ? "pill-green" : "pill-red"}`}>
                      <span className="pill-dot" />
                      {n.status}
                    </span>
                  </td>
                  <td className="text-dim">{new Date(n.sent_at).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </>
  );
}

function StatTile({ label, value, fraction }: { label: string; value: string; fraction: number }) {
  const cls = fraction > 0.9 ? "danger" : fraction > 0.7 ? "warn" : "";
  return (
    <div className="stat-tile">
      <div className="stat-value" style={{ fontSize: 18 }}>
        {value}
      </div>
      <div className="stat-label">{label}</div>
      <div className="meter">
        <div className={`meter-fill ${cls}`} style={{ width: `${Math.min(fraction * 100, 100)}%` }} />
      </div>
    </div>
  );
}

function AddPatForm({ onAdded }: { onAdded: () => void }) {
  const [label, setLabel] = useState("");
  const [token, setToken] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!label || !token) return;
    setBusy(true);
    setError(null);
    try {
      await api.post("/api/github/pats", { label, token });
      setLabel("");
      setToken("");
      onAdded();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to add token");
    } finally {
      setBusy(false);
    }
  };

  return (
    <form onSubmit={submit} className="form-row" style={{ marginTop: 14, alignItems: "flex-end" }}>
      {error && <div className="error-banner">{error}</div>}
      <div className="field" style={{ marginBottom: 0 }}>
        <label htmlFor="pat-label">Label</label>
        <input id="pat-label" className="input" value={label} onChange={(e) => setLabel(e.target.value)} placeholder="personal" />
      </div>
      <div className="field" style={{ marginBottom: 0 }}>
        <label htmlFor="pat-token">Token</label>
        <input
          id="pat-token"
          className="input mono"
          type="password"
          value={token}
          onChange={(e) => setToken(e.target.value)}
          placeholder="ghp_..."
        />
      </div>
      <button className="btn btn-sm" type="submit" disabled={busy || !label || !token}>
        Add
      </button>
    </form>
  );
}

function ReservePortForm({ onReserved }: { onReserved: () => void }) {
  const [port, setPort] = useState("");
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!port) return;
    setBusy(true);
    try {
      await api.post("/api/admin/ports", { port: Number(port), note });
      setPort("");
      setNote("");
      onReserved();
    } finally {
      setBusy(false);
    }
  };

  return (
    <form onSubmit={submit} className="form-row" style={{ marginTop: 14, alignItems: "flex-end" }}>
      <div className="field" style={{ marginBottom: 0 }}>
        <label htmlFor="reserve-port">Reserve port</label>
        <input
          id="reserve-port"
          className="input mono"
          type="number"
          value={port}
          onChange={(e) => setPort(e.target.value)}
          placeholder="8080"
        />
      </div>
      <div className="field" style={{ marginBottom: 0 }}>
        <label htmlFor="reserve-port-note">Note</label>
        <input
          id="reserve-port-note"
          className="input"
          value={note}
          onChange={(e) => setNote(e.target.value)}
          placeholder="used by X outside Mangrove"
        />
      </div>
      <button className="btn btn-sm" type="submit" disabled={busy || !port}>
        Reserve
      </button>
    </form>
  );
}

function errMsg(e: unknown): string {
  return e instanceof ApiError ? e.message : "Something went wrong";
}
