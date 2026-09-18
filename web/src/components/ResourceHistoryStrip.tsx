import type { ResourceUsageSnapshot } from "../api";

// Same tone thresholds as AdminPage's StatTile/Gauge (>0.9 danger, >0.7
// warn), so a spike in the trend strip reads the same as a spike in the
// live gauge above it.
function toneColor(fraction: number): string {
  if (fraction > 0.9) return "var(--red)";
  if (fraction > 0.7) return "var(--ochre)";
  return "var(--brass-dim)";
}

function fmtWhen(iso: string): string {
  return new Date(iso).toLocaleString(undefined, { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" });
}

// ResourceHistoryStrip renders scheduler.ResourceSampler's periodic
// snapshots as the same seismograph-strip idiom LogViewer uses for a live
// log tail (.log-strip) -- one tick per sample, height from its fraction
// of ceiling/total, a native `title` tooltip per tick as the minimal real
// hover affordance for a compact trend embedded in a settings page (not a
// full analytics surface).
export function ResourceHistoryStrip({ snapshots, metric }: { snapshots: ResourceUsageSnapshot[]; metric: "memory" | "disk" }) {
  if (snapshots.length === 0) {
    return <div className="text-dim" style={{ fontSize: 12.5, padding: "8px 0" }}>Not enough history yet -- sampled every 5 minutes.</div>;
  }

  return (
    <div className="resource-strip">
      {snapshots.map((s) => {
        const fraction =
          metric === "memory" ? s.memory_used_mb / Math.max(s.memory_ceiling_mb, 1) : s.disk_used_gb / Math.max(s.disk_total_gb, 1);
        const clamped = Math.min(Math.max(fraction, 0), 1);
        const label =
          metric === "memory"
            ? `${s.memory_used_mb.toFixed(0)} / ${s.memory_ceiling_mb} MB`
            : `${s.disk_used_gb.toFixed(1)} / ${s.disk_total_gb.toFixed(1)} GB`;
        return (
          <span
            key={s.id}
            className="resource-strip-tick"
            style={{ height: `${Math.max(clamped * 100, 3)}%`, background: toneColor(clamped) }}
            title={`${fmtWhen(s.recorded_at)} -- ${label}`}
          />
        );
      })}
    </div>
  );
}
