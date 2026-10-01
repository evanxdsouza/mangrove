import type { DeployHistory } from "../api";
import { Link } from "../router";
import { StatusPill } from "./StatusPill";
import { CodeBlock } from "./CodeBlock";
import { EmptyLedgerIcon } from "../icons";
import { fmtDuration, fmtWhen } from "../lib/format";

export const DOT_COLOR: Record<string, string> = {
  success: "var(--verdigris)",
  failed: "var(--red)",
  building: "var(--ochre)",
  healthchecking: "var(--ochre)",
  queued: "var(--text-faint)",
  rolled_back: "var(--ochre)",
};

export function DeployTimeline({
  history,
  onRollback,
  busyId,
  projectId,
  deploymentId,
}: {
  history: DeployHistory[];
  onRollback: (id: number) => void;
  busyId: number | null;
  // Each entry links to its own run page (RunDetailPage) -- the reference
  // dashboard's "run page" pattern, one route per run rather than only a
  // list. Optional so this component still works anywhere that context
  // isn't available.
  projectId?: number;
  deploymentId?: number;
}) {
  if (history.length === 0) {
    return (
      <div className="empty-state">
        <EmptyLedgerIcon />
        <p>No deploys yet.</p>
      </div>
    );
  }

  return (
    <div className="timeline">
      {history.map((h, i) => {
        const runNumber = history.length - i;
        const body = (
          <>
            <div className="timeline-meta">
              <StatusPill status={h.status} />
              {h.is_current && <span className="pill pill-gray">current</span>}
              <span className="mono text-dim">{h.commit_sha ? h.commit_sha.slice(0, 8) : h.triggered_by}</span>
              {h.rollback_of_deploy_history_id && (
                <span className="text-dim" style={{ fontSize: 12 }}>
                  rollback of #{h.rollback_of_deploy_history_id}
                </span>
              )}
            </div>
            <div className="timeline-time">
              {fmtWhen(h.started_at)} &middot; triggered by {h.triggered_by}
              {fmtDuration(h.started_at, h.finished_at) && <> &middot; {fmtDuration(h.started_at, h.finished_at)}</>}
            </div>
            {h.commit_message && <div className="timeline-msg">{h.commit_message}</div>}
            {h.error_message && (
              <div style={{ marginTop: 8 }}>
                <CodeBlock tone="danger">{h.error_message}</CodeBlock>
              </div>
            )}
          </>
        );
        return (
          <div className="timeline-item" key={h.id}>
            <div className="timeline-marker">
              <span className="timeline-index">{runNumber}</span>
              <div className="dot" style={{ background: DOT_COLOR[h.status] ?? "var(--text-faint)" }} />
            </div>
            {projectId != null && deploymentId != null ? (
              <Link to={`/projects/${projectId}/deployments/${deploymentId}/history/${h.id}`} className="timeline-body timeline-item-link">
                {body}
              </Link>
            ) : (
              <div className="timeline-body">{body}</div>
            )}
            {h.status === "success" && !h.is_current && (
              <button className="btn btn-sm" disabled={busyId !== null} onClick={() => onRollback(h.id)}>
                {busyId === h.id ? "Rolling back..." : "Revert to this"}
              </button>
            )}
          </div>
        );
      })}
    </div>
  );
}
