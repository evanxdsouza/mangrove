import type { DeployHistory } from "../api";
import { DOT_COLOR } from "./DeployTimeline";
import { fmtWhen } from "../lib/format";

// A run-outcome strip above the History tab's timeline -- the same
// seismograph-tick idiom LogViewer/ResourceHistoryStrip already use
// elsewhere, applied to deploy outcomes instead of log lines or resource
// samples: one uniform-height tick per run, colored by its real status,
// oldest on the left, most recent on the right. A glance at "how has this
// been going lately" without reading the list below it.
export function DeployHistorySparkline({ history }: { history: DeployHistory[] }) {
  if (history.length < 2) return null; // nothing to trend with 0-1 runs

  // `history` arrives newest-first (DeployTimeline's own convention); show
  // oldest-to-newest left-to-right, capped to the most recent 40 so a long
  // history doesn't crush each tick to nothing.
  const recent = history.slice(0, 40).slice().reverse();

  return (
    <div className="deploy-sparkline" title="Recent deploy outcomes, oldest to newest">
      {recent.map((h) => (
        <span
          key={h.id}
          className="deploy-sparkline-tick"
          style={{ background: DOT_COLOR[h.status] ?? "var(--text-faint)" }}
          title={`${h.status.replace(/_/g, " ")} · ${fmtWhen(h.started_at)}`}
        />
      ))}
    </div>
  );
}
