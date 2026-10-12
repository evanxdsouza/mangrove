import { useEffect, useId, useRef, useState } from "react";
import { createSpring, prefersReducedMotion } from "../lib/spring";
import { IN_PROGRESS_STATUSES, STATUS_COLORS } from "./StatusPill";

export type TideLevel = "high" | "mid" | "low" | "dry";

const LEVEL_BY_COLOR: Record<string, TideLevel> = {
  green: "high",
  yellow: "mid",
  gray: "low",
  red: "dry",
};

/** Maps a raw deployment/deploy status onto the tide language: high =
    running/healthy, mid = working (building, health checking, queued,
    timeout, rolled back), low = at rest (stopped, sleeping, never
    deployed), dry = needs you (failed, unhealthy, error). Reuses
    StatusPill's own classification so a gauge and its pill always agree. */
export function tideLevel(status: string | null | undefined): TideLevel {
  if (!status) return "low";
  return LEVEL_BY_COLOR[STATUS_COLORS[status] ?? "gray"] ?? "low";
}

// Water height (y of the wave crest in the 14x30 viewBox) and fill per level.
const WATER: Record<Exclude<TideLevel, "dry">, { y: number; fill: string }> = {
  high: { y: 8, fill: "var(--tide-high)" },
  mid: { y: 17, fill: "var(--tide-mid)" },
  low: { y: 25, fill: "var(--tide-low)" },
};

const SIZES = { sm: [12, 26], md: [14, 30], lg: [22, 48] } as const;

const LEVEL_LABEL: Record<TideLevel, string> = {
  high: "high",
  mid: "working",
  low: "at rest",
  dry: "needs attention",
};

/** The tide gauge -- Intertidal's one status mark, used on every project,
    deployment, search result, and activity row. A small capsule whose
    water line reads real state; a dry, red-rimmed gauge is the one that
    needs you. In-progress statuses get a slow wave on the water line. */
export function TideGauge({
  status,
  size = "md",
  burstKey,
}: {
  status: string | null | undefined;
  size?: keyof typeof SIZES;
  burstKey?: number;
}) {
  const level = tideLevel(status);
  const [w, h] = SIZES[size];
  const clipId = useId();
  const scale = useBurst(burstKey);
  const live = status != null && IN_PROGRESS_STATUSES.has(status);
  const label = `Tide gauge: ${LEVEL_LABEL[level]}${status ? `, ${status.replace(/_/g, " ")}` : ""}`;

  return (
    <svg
      className={`tide-gauge tide-gauge-${level} ${live ? "tide-gauge-live" : ""}`}
      width={w}
      height={h}
      viewBox="0 0 14 30"
      role="img"
      aria-label={label}
      style={scale !== 1 ? { transform: `scale(${scale})` } : undefined}
    >
      <defs>
        <clipPath id={clipId}>
          <rect x="1" y="1" width="12" height="28" rx="6" />
        </clipPath>
      </defs>
      <rect x="1" y="1" width="12" height="28" rx="6" fill="var(--bg-card)" stroke={level === "dry" ? "var(--red)" : "var(--tide-rim)"} />
      {level === "dry" ? (
        <path d="M3 25h8" stroke="var(--red)" strokeWidth={1.6} strokeLinecap="round" />
      ) : (
        <g clipPath={`url(#${clipId})`}>
          <path
            className="tide-gauge-water"
            d={`M-14 ${WATER[level].y}q3.5-3 7 0t7 0t7 0t7 0t7 0V30H-14z`}
            fill={WATER[level].fill}
          />
        </g>
      )}
    </svg>
  );
}

/** A one-shot spring-driven scale pop after a successful deploy. burstKey
    changes (e.g. a deploy history id) trigger it; the same key never
    re-fires. Built on lib/spring.ts so it can be interrupted the same way
    Modal's open/close already is. */
function useBurst(burstKey: number | undefined): number {
  const [scale, setScale] = useState(1);
  const lastKey = useRef<number | undefined>(undefined);

  useEffect(() => {
    if (burstKey == null || burstKey === lastKey.current || prefersReducedMotion()) {
      lastKey.current = burstKey;
      return;
    }
    lastKey.current = burstKey;
    const spring = createSpring(1.4, {
      damping: 0.5,
      response: 0.5,
      onUpdate: setScale,
    });
    spring.set(1);
    return () => spring.stop();
  }, [burstKey]);

  return scale;
}
