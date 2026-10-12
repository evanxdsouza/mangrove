// Authored line-icon set for Intertidal: one consistent stroke (1.9, round
// caps/joins) on a 24px grid, drawn to match the design canvas rather than
// pulled from a generic icon font -- see DESIGN.md. Each entry is the inner
// markup of a 24x24 <svg>; render one with components/Icon.svelte.
export const ICONS = {
  "mangrove": `<path d="M4 21c0-7 3.5-10 8-10s8 3 8 10" /> <path d="M8.5 21c0-4.5 1.4-7 3.5-7s3.5 2.5 3.5 7" /> <path d="M12 11V3" />`,
  "ledger": `<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />`,
  "cabinet": `<rect x="4" y="5" width="16" height="6" rx="2" /> <rect x="4" y="13" width="16" height="6" rx="2" /> <path d="M8 8h.01M8 16h.01" />`,
  "dials": `<path d="M4 7h10M18 7h2M4 17h2M10 17h10" /> <circle cx="16" cy="7" r="2" /> <circle cx="8" cy="17" r="2" />`,
  "gauge": `<path d="M3 12h4l2-6 4 12 2-6h6" />`,
  "gear": `<circle cx="12" cy="12" r="3" /> <path d="M12 3v2.5M12 18.5V21M3 12h2.5M18.5 12H21M5.6 5.6l1.8 1.8M16.6 16.6l1.8 1.8M5.6 18.4l1.8-1.8M16.6 7.4l1.8-1.8" />`,
  "globe": `<circle cx="12" cy="12" r="9" /> <path d="M3 12h18M12 3c3 3.5 3 14 0 18M12 3c-3 3.5-3 14 0 18" />`,
  "strip-chart": `<rect x="3" y="4" width="18" height="16" rx="3" /> <path d="M6 13h2.5l1.5-3.5 2.5 6 1.5-3.5H18" />`,
  "terminal": `<path d="m5 8 4 4-4 4M12 17h7" />`,
  "deploy": `<path d="M12 19V5M6 11l6-6 6 6" />`,
  "branch": `<circle cx="6" cy="6" r="2" /> <circle cx="6" cy="18" r="2" /> <circle cx="18" cy="8" r="2" /> <path d="M6 8v8M18 10c0 5-8 3-12 6" />`,
  "layers": `<path d="m12 3 9 5-9 5-9-5z" /> <path d="m3 13 9 5 9-5" />`,
  "box": `<path d="M12 3 4 7v10l8 4 8-4V7z" /> <path d="M4 7l8 4 8-4M12 11v10" />`,
  "lock": `<rect x="5" y="11" width="14" height="9" rx="2" /> <path d="M8 11V8a4 4 0 0 1 8 0v3" />`,
  "panel": `<rect x="3" y="4" width="18" height="16" rx="3" /> <path d="M9 4v16" />`,
  "restart": `<path d="M19 12a7 7 0 1 1-2.3-5.2" /> <path d="M19 4.5V9h-4.5" />`,
  "stop": `<rect x="6" y="6" width="12" height="12" rx="2" />`,
  "arrow-right": `<path d="M5 12h14M13 6l6 6-6 6" />`,
  "plus": `<path d="M12 5v14M5 12h14" />`,
  "trash": `<path d="M4 7h16M10 11v6M14 11v6M6 7l1 12h10l1-12M9 7V4h6v3" />`,
  "chevron-down": `<path d="m7 10 5 5 5-5" />`,
  "menu": `<path d="M4 7h16M4 12h16M4 17h16" />`,
  "close": `<path d="M6 6l12 12M18 6 6 18" />`,
  "user": `<circle cx="12" cy="8.2" r="3.4" /> <path d="M4.8 20c1-3.6 4-5.6 7.2-5.6s6.2 2 7.2 5.6" />`,
  "grid": `<rect x="4" y="4" width="7" height="7" rx="1.5" /> <rect x="13" y="4" width="7" height="7" rx="1.5" /> <rect x="4" y="13" width="7" height="7" rx="1.5" /> <rect x="13" y="13" width="7" height="7" rx="1.5" />`,
  "list-view": `<path d="M8 6h12M8 12h12M8 18h12M4 6h.01M4 12h.01M4 18h.01" />`,
  "search": `<circle cx="11" cy="11" r="7" /> <path d="m20 20-3.5-3.5" />`,
  "check-circle": `<circle cx="12" cy="12" r="9" /> <path d="m8 12.3 2.8 2.8 5.2-5.6" />`,
  "x-circle": `<circle cx="12" cy="12" r="9" /> <path d="m9.2 9.2 5.6 5.6M14.8 9.2l-5.6 5.6" />`,
  "alert": `<path d="M12 4 3 20h18z" /> <path d="M12 10v4M12 17h.01" />`,
  "sun": `<circle cx="12" cy="12" r="4" /> <path d="M12 2.5v2M12 19.5v2M2.5 12h2M19.5 12h2M5.3 5.3l1.4 1.4M17.3 17.3l1.4 1.4M5.3 18.7l1.4-1.4M17.3 6.7l1.4-1.4" />`,
  "moon": `<path d="M20 14.5A8 8 0 0 1 9.5 4a8 8 0 1 0 10.5 10.5Z" />`,
  "screen": `<rect x="3" y="4" width="18" height="12" rx="2" /> <path d="M8 20h8M12 16v4" />`,
} as const;

export type IconName = keyof typeof ICONS;
