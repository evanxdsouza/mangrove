import type { DeployHistory } from "../api";
import { StatusPill } from "./StatusPill";

const DOT_COLOR: Record<string, string> = {
  success: "var(--green)",
  failed: "var(--red)",
  building: "var(--yellow)",
  healthchecking: "var(--yellow)",
  queued: "var(--text-faint)",
  rolled_back: "var(--yellow)",
};

export function DeployTimeline({
  history,
  onRollback,
  busyId,
}: {
  history: DeployHistory[];
  onRollback: (id: number) => void;
  busyId: number | null;
}) {
  if (history.length === 0) {
    return <div className="empty-state">No deploys yet.</div>;
  }

  return (
    <div className="timeline">
      {history.map((h) => (
        <div className="timeline-item" key={h.id}>
          <div className="timeline-marker">
            <div className="dot" style={{ background: DOT_COLOR[h.status] ?? "var(--text-faint)" }} />
          </div>
          <div className="timeline-body">
            <div className="timeline-meta">
              <StatusPill status={h.status} />
              {h.is_current && <span className="pill pill-gray">current</span>}
              <span className="mono text-dim">{h.commit_sha ? h.commit_sha.slice(0, 8) : h.triggered_by}</span>
              {h.rollback_of_deploy_history_id && (
                <span className="text-faint" style={{ fontSize: 12 }}>
                  rollback of #{h.rollback_of_deploy_history_id}
                </span>
              )}
            </div>
            <div className="timeline-time">
              {new Date(h.started_at).toLocaleString()} &middot; triggered by {h.triggered_by}
            </div>
            {h.commit_message && <div className="timeline-msg">{h.commit_message}</div>}
            {h.error_message && <div className="timeline-msg" style={{ color: "var(--red)" }}>{h.error_message}</div>}
          </div>
          {h.status === "success" && !h.is_current && (
            <button className="btn btn-sm" disabled={busyId !== null} onClick={() => onRollback(h.id)}>
              {busyId === h.id ? "Rolling back..." : "Revert to this"}
            </button>
          )}
        </div>
      ))}
    </div>
  );
}
