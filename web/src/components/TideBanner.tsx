import { useId } from "react";
import { tideLevel, type TideLevel } from "./TideGauge";

interface Palette {
  sky: [string, string, number];
  water: [string, string];
  ridge: [string, number];
  halo: number;
  sun?: [number, number, string];
  glints?: [string, number];
}

// One scene per tide level: high tide (something running), turning
// (building / health checking), low tide (stopped, empty, never
// deployed), storm (failed or unhealthy). Colors are the canvas's own.
const SCENES: Record<TideLevel, Palette> = {
  high: { sky: ["#2E6B66", "#E7B77A", 0.75], water: ["#3C7F78", "#15423A"], ridge: ["#2B6B5F", 0.5], halo: 1, sun: [80, 13, "#FFF1CC"], glints: ["#FFF1CC", 0.5] },
  mid: { sky: ["#5C7FA3", "#F2C98B", 0.8], water: ["#6A93A8", "#2A4F5E"], ridge: ["#3E6A6A", 0.5], halo: 1, sun: [94, 14, "#FFF1CC"], glints: ["#FFF1CC", 0.55] },
  low: { sky: ["#A9BDB3", "#E4DCC6", 0.8], water: ["#B7A98A", "#7A6C52"], ridge: ["#8FA89B", 0.5], halo: 0.5 },
  dry: { sky: ["#3A3F46", "#B8735F", 0.8], water: ["#4A4C52", "#23252A"], ridge: ["#2C2F34", 0.6], halo: 0.35, sun: [86, 11, "#E7A38B"], glints: ["#E7A38B", 0.35] },
};

/** The mangrove-at-the-waterline banner art behind a project card or
    project header. Art, not data -- but it follows the same tide level as
    the project's gauge, so a storm banner always sits over a dry gauge. */
export function TideBanner({ status, level, className }: { status?: string | null; level?: TideLevel; className?: string }) {
  const lv = level ?? tideLevel(status);
  const s = SCENES[lv];
  const id = useId().replace(/:/g, "");
  const waterTop = lv === "high" ? 92 : 94;

  return (
    <svg className={`tide-banner ${className ?? ""}`} viewBox="0 0 800 132" preserveAspectRatio="xMidYMid slice" aria-hidden="true">
      <defs>
        <linearGradient id={`${id}s`} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor={s.sky[0]} />
          <stop offset={s.sky[2]} stopColor={s.sky[1]} />
        </linearGradient>
        <linearGradient id={`${id}w`} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor={s.water[0]} />
          <stop offset="1" stopColor={s.water[1]} />
        </linearGradient>
        <radialGradient id={`${id}h`}>
          <stop offset="0" stopColor="#FFE9B8" stopOpacity=".9" />
          <stop offset="1" stopColor="#FFE9B8" stopOpacity="0" />
        </radialGradient>
        <linearGradient id={`${id}v`} x1="0" y1="0" x2="1" y2="0">
          <stop offset="0" stopColor="#0A1F1A" stopOpacity=".38" />
          <stop offset=".25" stopColor="#0A1F1A" stopOpacity="0" />
          <stop offset=".75" stopColor="#0A1F1A" stopOpacity="0" />
          <stop offset="1" stopColor="#0A1F1A" stopOpacity=".38" />
        </linearGradient>
      </defs>
      <rect width="800" height="132" fill={`url(#${id}s)`} />
      <circle cx="470" cy={s.sun ? s.sun[0] - 4 : 70} r={lv === "mid" ? 70 : lv === "low" ? 46 : 60} fill={`url(#${id}h)`} opacity={s.halo} />
      {s.sun && <circle cx="470" cy={s.sun[0]} r={s.sun[1]} fill={s.sun[2]} />}
      <path d="M0 88Q90 70 180 82T360 79T540 84T800 76V100H0z" fill={s.ridge[0]} opacity={s.ridge[1]} />
      <rect y={waterTop} width="800" height={132 - waterTop} fill={`url(#${id}w)`} />
      {s.glints && (
        <g fill={s.glints[0]} opacity={s.glints[1]}>
          <rect x="448" y={waterTop + 6} width="44" height="2.5" rx="1" />
          <rect x="456" y={waterTop + 13} width="30" height="2.5" rx="1" />
          <rect x="462" y={waterTop + 20} width="18" height="2.5" rx="1" />
        </g>
      )}
      {lv === "low" && (
        <>
          <g fill="none" stroke="#6A5E46" strokeWidth="1.6" opacity=".5">
            <path d="M20 106q30-4 60 0t60 0" />
            <path d="M300 114q40-4 80 0t80 0" />
            <path d="M560 104q30-4 60 0t60 0" />
          </g>
          <ellipse cx="420" cy="116" rx="60" ry="5" fill="#9FB6AE" opacity=".8" />
        </>
      )}
      <g fill="#0F2B24" opacity={lv === "low" ? 0.9 : 1}>
        <ellipse cx="86" cy="60" rx="64" ry="24" opacity=".92" />
        <ellipse cx="140" cy="68" rx="38" ry="16" opacity=".85" />
        <ellipse cx="716" cy="64" rx="70" ry="24" opacity=".92" />
        <ellipse cx="660" cy="72" rx="34" ry="14" opacity=".85" />
        <g fill="none" stroke="#0F2B24" strokeWidth="4" strokeLinecap="round">
          <path d="M30 132C34 106 52 88 82 74" />
          <path d="M62 132C64 112 78 96 96 82" />
          <path d="M104 132C104 114 112 100 124 88" />
          <path d="M780 132C774 104 752 88 720 76" />
          <path d="M746 132C744 112 730 96 712 84" />
          <path d="M700 132C702 114 694 100 684 90" />
        </g>
      </g>
      <rect width="800" height="132" fill={`url(#${id}v)`} />
    </svg>
  );
}
