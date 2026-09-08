export const STATUS_COLORS: Record<string, "green" | "red" | "yellow" | "gray"> = {
  running: "green",
  success: "green",
  healthy: "green",
  failed: "red",
  unhealthy: "red",
  error: "red",
  building: "yellow",
  healthchecking: "yellow",
  queued: "yellow",
  pending: "gray",
  stopped: "gray",
  timeout: "yellow",
  rolled_back: "yellow",
  unknown: "gray",
};

// Lower rank = more urgent -- used to pick one "worst" status to represent
// a group of deployments (e.g. a project's ledger row) with a single
// beacon rather than showing nothing until you drill in.
const STATUS_RANK: Record<string, number> = {
  failed: 0,
  unhealthy: 0,
  error: 0,
  building: 1,
  healthchecking: 1,
  queued: 1,
  pending: 1,
  timeout: 1,
  stopped: 2,
  running: 3,
  success: 3,
  healthy: 3,
  rolled_back: 3,
};

export function worstStatus(statuses: string[]): string | null {
  if (statuses.length === 0) return null;
  return statuses.reduce((worst, s) => ((STATUS_RANK[s] ?? 2) < (STATUS_RANK[worst] ?? 2) ? s : worst));
}

// Statuses that represent an in-progress transition get a pulsing beacon
// ring -- a lamp signaling "in progress," not a decorative flourish. See
// .beacon-live in styles.css. Exported so Simple mode's own hand-rolled
// pill markup (plain-language status text instead of the raw enum) can
// apply the same beacon.
export const IN_PROGRESS_STATUSES = new Set(["building", "healthchecking", "queued", "pending"]);

export function StatusPill({ status }: { status: string }) {
  const color = STATUS_COLORS[status] ?? "gray";
  return (
    <span className={`pill pill-${color}`}>
      <span className={`pill-dot ${IN_PROGRESS_STATUSES.has(status) ? "beacon-live" : ""}`} />
      {status.replace(/_/g, " ")}
    </span>
  );
}
