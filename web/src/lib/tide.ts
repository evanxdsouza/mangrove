import { STATUS_COLORS } from "./status";

export type TideLevel = "high" | "mid" | "low" | "dry";

const LEVEL_BY_COLOR: Record<string, TideLevel> = { green: "high", yellow: "mid", gray: "low", red: "dry" };

/** Maps a raw deployment/deploy status onto the tide language: high =
    running/healthy, mid = working (building, health checking, queued,
    timeout, rolled back), low = at rest (stopped, sleeping, never
    deployed), dry = needs you (failed, unhealthy, error). Reuses the pill's
    own classification so a gauge and its pill always agree. */
export function tideLevel(status: string | null | undefined): TideLevel {
  if (!status) return "low";
  return LEVEL_BY_COLOR[STATUS_COLORS[status] ?? "gray"] ?? "low";
}
