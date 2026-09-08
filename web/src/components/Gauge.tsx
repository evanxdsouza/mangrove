// A real analog-style arc gauge for a resource reading (CPU/memory/disk/
// load): the fraction is drawn as a filled arc over a dimmer track, with
// tick marks at the quarter points, echoing an instrument panel dial.
// Renders alongside the actual number and label, never in place of them --
// see docs/DESIGN.md's craft-floor note on gauges standing in for content.
const SIZE = 72;
const STROKE = 6;
const RADIUS = (SIZE - STROKE) / 2;
const START_ANGLE = -215; // degrees, 0 = 3 o'clock, clockwise positive
const SWEEP = 250;

function polar(cx: number, cy: number, r: number, angleDeg: number) {
  const a = (angleDeg * Math.PI) / 180;
  return { x: cx + r * Math.cos(a), y: cy + r * Math.sin(a) };
}

function arcPath(cx: number, cy: number, r: number, startDeg: number, endDeg: number): string {
  const start = polar(cx, cy, r, startDeg);
  const end = polar(cx, cy, r, endDeg);
  const largeArc = endDeg - startDeg <= 180 ? 0 : 1;
  return `M ${start.x} ${start.y} A ${r} ${r} 0 ${largeArc} 1 ${end.x} ${end.y}`;
}

export function Gauge({ fraction, tone = "default" }: { fraction: number; tone?: "default" | "warn" | "danger" }) {
  const clamped = Math.min(Math.max(fraction, 0), 1);
  const cx = SIZE / 2;
  const cy = SIZE / 2;
  const d = arcPath(cx, cy, RADIUS, START_ANGLE, START_ANGLE + SWEEP);
  const arcLength = (SWEEP * Math.PI * RADIUS) / 180;

  const ticks = [0, 0.25, 0.5, 0.75, 1].map((t) => {
    const angle = START_ANGLE + SWEEP * t;
    const outer = polar(cx, cy, RADIUS + STROKE / 2 + 1.5, angle);
    const inner = polar(cx, cy, RADIUS + STROKE / 2 + 4, angle);
    return { key: t, x1: outer.x, y1: outer.y, x2: inner.x, y2: inner.y };
  });

  return (
    <div className="gauge-figure" style={{ width: SIZE, height: SIZE }}>
      <svg width={SIZE} height={SIZE} viewBox={`0 0 ${SIZE} ${SIZE}`}>
        <path className="gauge-track" d={d} strokeWidth={STROKE} />
        <path
          className={`gauge-value ${tone !== "default" ? tone : ""}`}
          d={d}
          strokeWidth={STROKE}
          strokeDasharray={arcLength}
          strokeDashoffset={arcLength * (1 - clamped)}
        />
        <g className="gauge-ticks">
          {ticks.map((t) => (
            <line key={t.key} x1={t.x1} y1={t.y1} x2={t.x2} y2={t.y2} />
          ))}
        </g>
      </svg>
      <div className="gauge-readout">
        <span className="n" style={{ fontSize: 15 }}>
          {Math.round(clamped * 100)}
          <span style={{ fontSize: 10, opacity: 0.65 }}>%</span>
        </span>
      </div>
    </div>
  );
}
