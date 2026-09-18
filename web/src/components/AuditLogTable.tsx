import type { AuditEvent } from "../api";
import { EmptyLedgerIcon } from "../icons";

function fmtWhen(iso: string): string {
  return new Date(iso).toLocaleString(undefined, { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" });
}

// AuditLogTable is the shared rendering for both the per-workspace log
// (WorkspacesPage's Activity modal, viewer+) and the global one
// (AdminPage's Audit log card, owner-only) -- same shape either way, one
// row per audit_log entry, newest first (the API already returns it
// that way).
export function AuditLogTable({ events, showWorkspace }: { events: AuditEvent[] | null; showWorkspace?: boolean }) {
  if (events === null) {
    return <div className="text-dim">Loading...</div>;
  }
  if (events.length === 0) {
    return (
      <div className="empty-state">
        <EmptyLedgerIcon />
        <p>No activity recorded yet.</p>
      </div>
    );
  }
  return (
    <table>
      <thead>
        <tr>
          <th>When</th>
          <th>Who</th>
          <th>Action</th>
          <th>Resource</th>
          {showWorkspace && <th>Workspace</th>}
          <th>Detail</th>
        </tr>
      </thead>
      <tbody>
        {events.map((e) => (
          <tr key={e.id}>
            <td className="text-dim mono">{fmtWhen(e.created_at)}</td>
            <td>{e.actor_email}</td>
            <td className="mono">{e.action}</td>
            <td className="text-dim">
              {e.resource_type}
              {e.resource_id != null ? ` #${e.resource_id}` : ""}
            </td>
            {showWorkspace && <td className="text-dim">{e.workspace_id ?? "—"}</td>}
            <td className="text-dim">{e.detail || "—"}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
