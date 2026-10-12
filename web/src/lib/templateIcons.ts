// Per-template icons for the template gallery -- authored in the same
// 1.7px stroke / round-cap grammar as icons.ts, one recognizable glyph
// per template (Ghost's ghost, Postgres' elephant, Redis's cube stack)
// instead of every template being plain text in a list. A key not listed
// here falls back to its category's icon in CATEGORY_ICON, so a future
// template never regresses to no icon at all.
// Rendered with components/TemplateIcon.svelte.
const GLYPHS: Record<string, string> = {
  Ghost: `<path d="M6 20V11a6 6 0 0 1 12 0v9l-2.2-1.8L14 20l-2-1.8L10 20l-1.8-1.8z" /> <circle cx="9.7" cy="11.3" r="0.9" fill="currentColor" stroke="none" /> <circle cx="14.3" cy="11.3" r="0.9" fill="currentColor" stroke="none" />`,
  WordPressMark: `<circle cx="12" cy="12" r="8.5" /> <path d="M5.5 9.5 9 17l2-5-1.3-3.2M18.5 9.5 15 18l-1.8-4.5M13.3 9h3.4M6.6 9h3" />`,
  Shield: `<path d="M12 3.5 19 6v5.5c0 4.5-3 7.2-7 9-4-1.8-7-4.5-7-9V6z" /> <circle cx="12" cy="11" r="2" /> <path d="M12 13v2.6" />`,
  BearFace: `<circle cx="7.2" cy="6.6" r="2" /> <circle cx="16.8" cy="6.6" r="2" /> <circle cx="12" cy="13" r="7" /> <circle cx="9" cy="12.3" r="0.9" fill="currentColor" stroke="none" /> <circle cx="15" cy="12.3" r="0.9" fill="currentColor" stroke="none" /> <path d="M10.3 16c.6.6 2.8.6 3.4 0" />`,
  NodeNetwork: `<circle cx="5.5" cy="7" r="2.3" /> <circle cx="5.5" cy="17" r="2.3" /> <circle cx="18.5" cy="12" r="2.3" /> <path d="M7.6 8 16.6 11M7.6 16 16.6 13" />`,
  BarsUp: `<path d="M4 20V13M9.5 20V9M15 20v-7M20 20V6" /> <path d="M3.5 20.5h17" />`,
  SheetGrid: `<rect x="3.5" y="4.5" width="17" height="15" rx="1.4" /> <path d="M3.5 10h17M3.5 15h17M10 4.5v15M15.5 4.5v15" />`,
  Pocket: `<path d="M4.5 6h15v10.5a3.5 3.5 0 0 1-3.5 3.5h-8A3.5 3.5 0 0 1 4.5 16.5z" /> <path d="M4.5 6 12 12l7.5-6" />`,
  TeaCup: `<path d="M5 10h11v4.5a4 4 0 0 1-4 4H9a4 4 0 0 1-4-4z" /> <path d="M16 11.5h1.8a2 2 0 0 1 0 4H16" /> <path d="M9 6.2c-.6-.9-.2-1.5.2-2M12.5 6.2c-.6-.9-.2-1.5.2-2" />`,
  Bolt: `<path d="M13 3 6 13.5h5L11 21l7-10.5h-5z" />`,
  Elephant: `<path d="M4 15c0-4.5 3-8 8-8s7 3 7 6.5c0 1.7-1 2.8-2.5 2.8" /> <path d="M16.5 16.3c1-.2 2 .4 2 1.6 0 1-.8 1.6-1.8 1.6" /> <path d="M6.5 13.5V19M9.5 13.8V19" /> <circle cx="8" cy="10" r="0.9" fill="currentColor" stroke="none" />`,
  Dolphin: `<path d="M4 16c3-6 9-10 16-9-1 2-2.6 3-4.3 3.4C17.5 12 19 14 19 16.5c-2.5 1-5-.2-6-2-2 1.8-5.5 2.3-9-1" /> <circle cx="14.3" cy="9.4" r="0.7" fill="currentColor" stroke="none" />`,
  Leaf: `<path d="M6 19c-1-7 2.5-13.5 12-14.5C19 13.5 13.5 18 6 19Z" /> <path d="M7 18c3-4 6-7 10.5-12.5" />`,
  CubeStack: `<path d="M12 3.5 19 7.2 12 11 5 7.2z" /> <path d="M5 12 12 15.7 19 12M5 16.8 12 20.5l7-3.7" />`,
  Wing: `<path d="M4 18c4-1 6-4 6.5-8.5C13 12 13.5 16 20 17c-3.5.8-7 .3-9-1.6-.6 2-2.3 3.3-7 2.6Z" />`,
};

export const TEMPLATE_ICONS: Record<string, string> = {
  ghost: GLYPHS.Ghost,
  wordpress: GLYPHS.WordPressMark,
  vaultwarden: GLYPHS.Shield,
  "uptime-kuma": GLYPHS.BearFace,
  n8n: GLYPHS.NodeNetwork,
  umami: GLYPHS.BarsUp,
  nocodb: GLYPHS.SheetGrid,
  pocketbase: GLYPHS.Pocket,
  gitea: GLYPHS.TeaCup,
  supabase: GLYPHS.Bolt,
  postgres: GLYPHS.Elephant,
  mysql: GLYPHS.Dolphin,
  mongodb: GLYPHS.Leaf,
  redis: GLYPHS.CubeStack,
  nephthys: GLYPHS.Wing,
};

const CATEGORY_ICON: Record<string, string> = {
  cms: GLYPHS.SheetGrid,
  database: GLYPHS.CubeStack,
  automation: GLYPHS.NodeNetwork,
  monitoring: GLYPHS.BarsUp,
  analytics: GLYPHS.BarsUp,
  backend: GLYPHS.Bolt,
  security: GLYPHS.Shield,
};

export function templateIconMarkup(templateKey: string, category: string): string {
  return TEMPLATE_ICONS[templateKey] ?? CATEGORY_ICON[category] ?? GLYPHS.SheetGrid;
}

// A hand-configured "image" deployment (not installed from a template) can
// still carry a recognizable icon when its own image reference happens to
// match one of Mangrove's own curated template keys (e.g. `image_ref:
// "postgres:16"` matches the "postgres" key that already has an elephant
// glyph) -- an honest, narrow heuristic, not a guess: it only ever returns
// a key this file already has a *real*, deliberately-authored icon for,
// never a fabricated/generic match. Returns null (render the TideGauge alone)
// for anything else, including well-known non-template images like nginx
// that have no authored glyph here.
export function imageIconKey(imageRef: string | undefined | null): string | null {
  if (!imageRef) return null;
  const withoutDigest = imageRef.split("@")[0];
  const withoutTag = withoutDigest.split(":")[0];
  const segments = withoutTag.split("/");
  const base = segments[segments.length - 1].toLowerCase();
  return TEMPLATE_ICONS[base] ? base : null;
}
