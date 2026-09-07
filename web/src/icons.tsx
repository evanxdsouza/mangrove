import type { SVGProps } from "react";

// Authored line-icon set for the Field Station / Instrument Room world:
// one consistent stroke (1.7, round caps/joins), drawn from the same
// instrument-panel grammar (dials, ledgers, engraved plaques) rather than
// pulled from a generic icon font -- see docs/DESIGN.md.
type IconProps = SVGProps<SVGSVGElement>;

const base = {
  viewBox: "0 0 24 24",
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 1.7,
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
};

/** Compass rose -- the station/workspace mark and sidebar brand mark. */
export function CompassIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M12 6.2 13.6 11 12 12l-1.6-1z" fill="currentColor" stroke="none" />
      <path d="M12 17.8 10.4 13 12 12l1.6 1z" fill="currentColor" stroke="none" opacity="0.45" />
      <circle cx="12" cy="12" r="1.1" fill="currentColor" stroke="none" />
      <path d="M12 3v1.4M12 19.6V21M3 12h1.4M19.6 12H21" />
    </svg>
  );
}

/** Ruled ledger -- Projects. */
export function LedgerIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <rect x="4.5" y="3.5" width="15" height="17" rx="1.6" />
      <path d="M8 3.5v17M11.5 8h5M11.5 12h5M11.5 16h3.5" />
    </svg>
  );
}

/** Stacked cabinet drawers -- Storage. */
export function CabinetIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <rect x="4.5" y="3.5" width="15" height="17" rx="1.4" />
      <path d="M4.5 11h15" />
      <path d="M10.3 7h3.4M10.3 15h3.4" />
    </svg>
  );
}

/** Adjustment sliders -- Admin. */
export function DialsIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <path d="M5 6h14M5 12h14M5 18h14" />
      <circle cx="9" cy="6" r="1.9" fill="var(--bg-card, #1c160e)" />
      <circle cx="16" cy="12" r="1.9" fill="var(--bg-card, #1c160e)" />
      <circle cx="10.5" cy="18" r="1.9" fill="var(--bg-card, #1c160e)" />
    </svg>
  );
}

/** Half-circle gauge with needle -- Server health. */
export function GaugeIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <path d="M4 16a8 8 0 0 1 16 0" />
      <path d="M12 16 15.2 10.4" />
      <circle cx="12" cy="16" r="1.2" fill="currentColor" stroke="none" />
      <path d="M4 19h16" />
    </svg>
  );
}

/** Gear -- Settings. */
export function GearIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <circle cx="12" cy="12" r="3" />
      <path d="M12 3.5v2.4M12 18.1v2.4M20.5 12h-2.4M5.9 12H3.5M17.7 6.3l-1.7 1.7M8 16l-1.7 1.7M17.7 17.7 16 16M8 8 6.3 6.3" />
    </svg>
  );
}

/** Globe with meridians -- Domains. */
export function GlobeIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M3.5 12h17M12 3.5c2.8 2.3 2.8 15.7 0 17M12 3.5c-2.8 2.3-2.8 15.7 0 17" />
    </svg>
  );
}

/** Strip-chart trace -- Logs. */
export function StripChartIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <rect x="3.5" y="4" width="17" height="16" rx="1.4" />
      <path d="M6 15l2.4-4.5 2 3 2.6-6 2.4 4.8L18 10.5" />
    </svg>
  );
}

/** Prompt caret -- Terminal. */
export function TerminalIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <rect x="3.5" y="4" width="17" height="16" rx="1.4" />
      <path d="M7.5 9.5 11 12.5 7.5 15.5M13 15.5h4" />
    </svg>
  );
}

/** Upward bearing arrow -- Deploy. */
export function DeployIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <path d="M12 20V6.5" />
      <path d="M6.5 11 12 5.5 17.5 11" />
      <path d="M6 20h12" />
    </svg>
  );
}

/** Branch/fork glyph -- GitHub (avoids the trademarked mark, same idea). */
export function BranchIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <circle cx="6" cy="6" r="2.1" />
      <circle cx="6" cy="18" r="2.1" />
      <circle cx="18" cy="9" r="2.1" />
      <path d="M6 8.1V16" />
      <path d="M6 12c0-2.8 2.3-3.8 4-3.8h6" />
      <path d="M18 11.1V13" />
    </svg>
  );
}

export function PlusIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <path d="M12 5v14M5 12h14" />
    </svg>
  );
}

export function TrashIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <path d="M4.5 7h15" />
      <path d="M9 7V5.2c0-.7.6-1.2 1.3-1.2h3.4c.7 0 1.3.5 1.3 1.2V7" />
      <path d="M6.5 7l.8 12.1c.05.8.7 1.4 1.5 1.4h6.4c.8 0 1.45-.6 1.5-1.4L18 7" />
      <path d="M10.3 11v6M13.7 11v6" />
    </svg>
  );
}

export function ChevronDownIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <path d="M6 9.5 12 15l6-5.5" />
    </svg>
  );
}

/** Three-line menu -- the mobile nav toggle. */
export function MenuIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <path d="M4 6.5h16M4 12h16M4 17.5h16" />
    </svg>
  );
}

export function CloseIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <path d="M6 6l12 12M18 6 6 18" />
    </svg>
  );
}

export function UserIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <circle cx="12" cy="8.2" r="3.4" />
      <path d="M4.8 20c1-3.6 4-5.6 7.2-5.6s6.2 2 7.2 5.6" />
    </svg>
  );
}

export function AlertIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <path d="M12 3.5 21.5 19.5H2.5z" />
      <path d="M12 9.5v5" />
      <circle cx="12" cy="17.2" r="0.9" fill="currentColor" stroke="none" />
    </svg>
  );
}

/** Blank ledger page -- generic empty state. */
export function EmptyLedgerIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <rect x="4.5" y="3.5" width="15" height="17" rx="1.6" strokeDasharray="2.6 2.6" />
      <path d="M8 3.5v17" strokeDasharray="2.6 2.6" />
    </svg>
  );
}
