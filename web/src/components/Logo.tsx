import { useId } from "react";

// Mangroves are defined by their above-water prop roots -- a canopy over a
// trunk that splits into arching roots reads as "mangrove" at a glance,
// unlike the previous mark (an abstract gradient blob whose masked outline
// read as a compass needle to users, despite nothing in the code calling it
// that). Colored via CSS custom properties so it re-colors automatically
// with the active theme (see theme.tsx); mangroveLogoMarkup below renders
// the same shapes with literal colors for the browser-tab favicon, which
// can't inherit the app's stylesheet.
const CANOPY = [
  { cx: 16, cy: 10, r: 6.5 },
  { cx: 10.5, cy: 13, r: 4.5 },
  { cx: 21.5, cy: 13, r: 4.5 },
];
const TRUNK = { x: 14.6, y: 14, width: 2.8, height: 7, rx: 1.4 };
const ROOTS = ["M16 19.5 C 11 21.5, 8 23.5, 5.5 29", "M16 19.5 C 16 23, 16 26, 16 29", "M16 19.5 C 21 21.5, 24 23.5, 26.5 29"];

export function Logo({ size = 22 }: { size?: number }) {
  const gradId = useId();
  return (
    <svg width={size} height={size} viewBox="0 0 32 32" fill="none" aria-hidden="true" style={{ flexShrink: 0 }}>
      <defs>
        <linearGradient id={gradId} x1="4" y1="4" x2="28" y2="29" gradientUnits="userSpaceOnUse">
          <stop offset="0" stopColor="var(--accent)" />
          <stop offset="1" stopColor="var(--green)" />
        </linearGradient>
      </defs>
      {CANOPY.map((c, i) => (
        <circle key={i} cx={c.cx} cy={c.cy} r={c.r} fill={`url(#${gradId})`} />
      ))}
      <rect x={TRUNK.x} y={TRUNK.y} width={TRUNK.width} height={TRUNK.height} rx={TRUNK.rx} fill={`url(#${gradId})`} />
      {ROOTS.map((d, i) => (
        <path key={i} d={d} stroke={`url(#${gradId})`} strokeWidth={2.2} strokeLinecap="round" fill="none" />
      ))}
    </svg>
  );
}

// Standalone SVG markup (literal colors, no CSS vars) for the browser-tab
// favicon -- a separate document context that can't see the app's
// stylesheet, so theme.tsx swaps this in as a data: URI whenever the theme
// changes, keeping the tab icon in sync with the in-app logo.
export function mangroveLogoMarkup(fromColor: string, toColor: string): string {
  const canopy = CANOPY.map((c) => `<circle cx="${c.cx}" cy="${c.cy}" r="${c.r}" fill="url(#g)"/>`).join("");
  const trunk = `<rect x="${TRUNK.x}" y="${TRUNK.y}" width="${TRUNK.width}" height="${TRUNK.height}" rx="${TRUNK.rx}" fill="url(#g)"/>`;
  const roots = ROOTS.map((d) => `<path d="${d}" stroke="url(#g)" stroke-width="2.2" stroke-linecap="round" fill="none"/>`).join("");
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><defs><linearGradient id="g" x1="4" y1="4" x2="28" y2="29" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="${fromColor}"/><stop offset="1" stop-color="${toColor}"/></linearGradient></defs>${canopy}${trunk}${roots}</svg>`;
}
