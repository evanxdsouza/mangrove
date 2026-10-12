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
  sleeping: "gray",
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
  sleeping: 2,
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
//
// "pending" is deliberately excluded: for a Deployment record it means
// "created, never deployed yet" -- nothing is actually happening in the
// background, so pulsing it as if it were mid-operation was dishonest
// (Product Principle 1: every state must reflect real backend state, never
// decorative UI). It renders identically to "stopped" instead -- both are
// genuinely "no container running," just for different reasons.
export const IN_PROGRESS_STATUSES = new Set(["building", "healthchecking", "queued"]);


// Timeline/sparkline dot color for a deploy-history status.
export const DOT_COLOR: Record<string, string> = {
  success: "var(--verdigris)",
  failed: "var(--red)",
  building: "var(--ochre)",
  healthchecking: "var(--ochre)",
  queued: "var(--text-faint)",
  rolled_back: "var(--ochre)",
};
