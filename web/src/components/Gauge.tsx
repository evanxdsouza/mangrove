// A resource tide bar for a reading (CPU/memory/disk/load/swap): the
// fraction fills a rounded track -- teal to 70%, amber above 70%, red above
// 90% -- with the percentage beside it. Renders alongside the actual number
// and label, never in place of them (DESIGN.md's craft-floor note on
// gauges standing in for content).
export function Gauge({ fraction, tone = "default" }: { fraction: number; tone?: "default" | "warn" | "danger" }) {
  const clamped = Math.min(Math.max(fraction, 0), 1);
  const pct = Math.round(clamped * 100);
  return (
    <div className="tide-bar-row">
      <div className="tide-bar" role="meter" aria-valuemin={0} aria-valuemax={100} aria-valuenow={pct} aria-label={`${pct}%`}>
        <i className={tone !== "default" ? tone : undefined} style={{ width: `${pct}%` }} />
      </div>
      <span className="tide-bar-pct">{pct}%</span>
    </div>
  );
}
