import { useEffect, useRef, useState } from "react";
import { createSpring, prefersReducedMotion } from "../lib/spring";
import { STATUS_COLORS } from "./StatusPill";

type Pose = "healthy" | "growing" | "wilted" | "dormant";

const POSE_BY_COLOR: Record<string, Pose> = {
  green: "healthy",
  yellow: "growing",
  red: "wilted",
  gray: "dormant",
};

const POSE_COLOR: Record<Pose, string> = {
  healthy: "var(--leaf-bright)",
  growing: "var(--ochre)",
  wilted: "var(--red)",
  dormant: "var(--text-faint)",
};

/** A small root-and-sprout glyph whose posture reads real status -- upright
    and full when healthy, still uncurling while building, drooping on a
    failure, bare when stopped -- so a project/deployment card carries a
    living face instead of a status pill being the only signal of health.
    Reuses StatusPill's own green/yellow/red/gray classification so a
    card's plant and its pill always agree. */
export function PlantGlyph({ status, size = 22, burstKey }: { status: string | null | undefined; size?: number; burstKey?: number }) {
  const pose: Pose = status ? POSE_BY_COLOR[STATUS_COLORS[status] ?? "gray"] ?? "dormant" : "dormant";
  const color = POSE_COLOR[pose];
  const scale = useBurst(burstKey);

  return (
    <span
      className={`plant-glyph plant-glyph-${pose}`}
      style={{ width: size, height: size, color, transform: `scale(${scale})` }}
    >
      <svg viewBox="0 0 24 24" width={size} height={size} fill="none" stroke="currentColor" strokeWidth={1.7} strokeLinecap="round" strokeLinejoin="round">
        {pose === "healthy" && (
          <>
            <path d="M12 20v-7.5" />
            <path d="M12 12.5c0-3 2-4.5 5-5 .3 3-1.4 5.6-5 5" />
            <path d="M12 14.5c0-2.6-1.8-4-4.4-4.4-.3 2.6 1.2 4.8 4.4 4.4" />
            <path d="M9 20h6" />
          </>
        )}
        {pose === "growing" && (
          <>
            <path d="M12 20v-5.5" className="plant-glyph-sway" />
            <path d="M12 14.5c0-2 1.2-3.1 3.4-3.5.2 2-.9 3.8-3.4 3.5" className="plant-glyph-sway" />
            <path d="M9 20h6" />
          </>
        )}
        {pose === "wilted" && (
          <>
            <path d="M12 20v-4.5" />
            <path d="M12 15.5c1.6-.6 3-1.8 3.6-3.8-2.2-.4-4 .6-4.6 2.4" />
            <path d="M9 20h6" />
          </>
        )}
        {pose === "dormant" && (
          <>
            <path d="M12 20v-2.5" />
            <path d="M9 20h6" />
          </>
        )}
      </svg>
    </span>
  );
}

/** A one-shot spring-driven scale pop -- the "burst of visible growth"
    after a successful deploy. burstKey changes (e.g. a deploy history id)
    trigger it; the same key never re-fires. Built on lib/spring.ts's
    critically-damped spring rather than a CSS keyframe so it can be
    interrupted/retargeted the same way Modal's open/close already is. */
function useBurst(burstKey: number | undefined): number {
  const [scale, setScale] = useState(1);
  const lastKey = useRef<number | undefined>(undefined);

  useEffect(() => {
    if (burstKey == null || burstKey === lastKey.current || prefersReducedMotion()) {
      lastKey.current = burstKey;
      return;
    }
    lastKey.current = burstKey;
    const spring = createSpring(1.5, {
      damping: 0.5,
      response: 0.5,
      onUpdate: setScale,
    });
    spring.set(1);
    return () => spring.stop();
  }, [burstKey]);

  return scale;
}
