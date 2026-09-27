// Shared formatters -- imported everywhere a timestamp or an internal
// enum/DB value reaches the UI, instead of an inline `.toLocaleString()`
// variant or a raw string per call site (see DESIGN.md's Do/Don't list:
// one date format across the app, no internal identifier shown verbatim).

/** Compact instrument-log timestamp: "Sept 27, 5:19 PM" -- no year (this
    year is assumed), no seconds (never meaningfully different from a
    poll interval). The one date format used everywhere in the dashboard. */
export function fmtWhen(iso: string | null | undefined): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

/** Full timestamp for a context where the exact moment matters more than
    scanability (e.g. a single detail row) -- still consistent formatting,
    just with the year included. */
export function fmtWhenFull(iso: string | null | undefined): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

/** "3 minutes ago" / "in 2 hours" -- for the once-glanced-at cases where
    relative time reads faster than an absolute one. */
export function fmtRelative(iso: string): string {
  const ms = new Date(iso).getTime() - Date.now();
  const abs = Math.abs(ms);
  const MIN = 60_000, HOUR = 3_600_000, DAY = 86_400_000;
  const past = ms < 0;
  let value: number, unit: string;
  if (abs < MIN) return "just now";
  if (abs < HOUR) { value = Math.round(abs / MIN); unit = "minute"; }
  else if (abs < DAY) { value = Math.round(abs / HOUR); unit = "hour"; }
  else { value = Math.round(abs / DAY); unit = "day"; }
  const plural = value === 1 ? unit : `${unit}s`;
  return past ? `${value} ${plural} ago` : `in ${value} ${plural}`;
}

/** Internal DB/enum value -> human label. Anything not in the table falls
    back to the same snake_case->"Title Case with spaces" transform so a
    future enum value never leaks verbatim, even before someone adds it
    here explicitly. */
const LABELS: Record<string, string> = {
  // Port registry allocation types (internal/portregistry)
  system: "System",
  service_public: "Public service port",
  custom_domain_port: "Custom domain port",
  // Audit log actions -- deploy triggers (deploy_history.triggered_by,
  // reused as-is for the audit action -- see auditDeploy in
  // internal/api/deployments.go)
  manual: "Deployed (manual)",
  redeploy: "Redeployed",
  promote: "Promoted",
  push: "Deployed (push)",
  deploy: "Deployed",
  // Audit log actions -- everything else (internal/api/*.go's s.audit /
  // s.auditCtxWorkspace call sites)
  delete: "Deleted",
  rollback: "Rolled back",
  scale: "Scaled",
  set_access_control: "Changed access control",
  set_secret_env_var: "Set secret",
  add_custom_domain: "Added custom domain",
  delete_custom_domain: "Removed custom domain",
  move_project: "Moved project",
  add_member: "Added member",
  remove_member: "Removed member",
  set_member_role: "Changed member role",
  create_user: "Created user",
  delete_user: "Deleted user",
  revoke_session: "Revoked session",
  // Audit log resource types
  project: "Project",
  deployment: "Deployment",
  workspace: "Workspace",
  user: "User",
  service: "Service",
  session: "Session",
  custom_domain: "Custom domain",
};

export function humanLabel(value: string | null | undefined): string {
  if (!value) return "—";
  if (LABELS[value]) return LABELS[value];
  return value
    .split("_")
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(" ");
}
