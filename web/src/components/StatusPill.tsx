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

export function StatusPill({ status }: { status: string }) {
  const color = STATUS_COLORS[status] ?? "gray";
  return (
    <span className={`pill pill-${color}`}>
      <span className="pill-dot" />
      {status.replace(/_/g, " ")}
    </span>
  );
}
