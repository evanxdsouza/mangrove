// An "instrument metal" -- swaps the --brass* variables in styles.css's
// per-theme [data-theme="..."] blocks. In Intertidal the metal only tints
// the sidebar's small section labels and the station eyebrow; the
// deep-green sidebar, leaf-green buttons, and tide status colors never
// change. "brass" is the default and needs no override block since it
// matches styles.css's :root values.
export interface ThemeDef {
  id: string;
  label: string;
  accent: string;
}

export const THEMES: ThemeDef[] = [
  { id: "brass", label: "Brass", accent: "#8fb0a2" },
  { id: "copper", label: "Copper", accent: "#c97b4a" },
  { id: "iron", label: "Iron", accent: "#8fa3b0" },
  { id: "silver", label: "Silver", accent: "#c7c2b4" },
  { id: "indigo", label: "Indigo", accent: "#8b7fd1" },
];
const DEFAULT_THEME = THEMES[0].id;
const STORAGE_KEY = "mangrove-theme";

function readStored(): string {
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    return THEMES.some((t) => t.id === stored) ? stored! : DEFAULT_THEME;
  } catch {
    return DEFAULT_THEME;
  }
}

export const themeState = $state<{ theme: string }>({ theme: readStored() });
document.documentElement.setAttribute("data-theme", themeState.theme);

export function setTheme(id: string) {
  themeState.theme = id;
  document.documentElement.setAttribute("data-theme", id);
  try {
    window.localStorage.setItem(STORAGE_KEY, id);
  } catch {
    // localStorage unavailable (private browsing etc.) -- theme still
    // applies for this session, just doesn't persist across reloads.
  }
}

// ---- Appearance: light, dark, or follow the system ------------------------
// Stored per browser. "system" (the default) follows prefers-color-scheme
// live. Whatever is chosen always resolves to data-color-scheme="light" or
// "dark" on <html> -- the one attribute styles.css's dark block keys on.
// app.html applies the same resolution before first paint, so a reload
// never flashes the wrong scheme.
export type Appearance = "light" | "dark" | "system";
const APPEARANCE_KEY = "mangrove-appearance";
const darkQuery = window.matchMedia("(prefers-color-scheme: dark)");

function readAppearance(): Appearance {
  try {
    const v = window.localStorage.getItem(APPEARANCE_KEY);
    return v === "light" || v === "dark" ? v : "system";
  } catch {
    return "system";
  }
}

function resolve(a: Appearance): "light" | "dark" {
  return a === "system" ? (darkQuery.matches ? "dark" : "light") : a;
}

export const appearanceState = $state<{ appearance: Appearance; scheme: "light" | "dark" }>({
  appearance: readAppearance(),
  scheme: "light",
});

function applyScheme() {
  appearanceState.scheme = resolve(appearanceState.appearance);
  document.documentElement.setAttribute("data-color-scheme", appearanceState.scheme);
}
applyScheme();
darkQuery.addEventListener("change", () => {
  if (appearanceState.appearance === "system") applyScheme();
});

export function setAppearance(a: Appearance) {
  appearanceState.appearance = a;
  applyScheme();
  try {
    if (a === "system") window.localStorage.removeItem(APPEARANCE_KEY);
    else window.localStorage.setItem(APPEARANCE_KEY, a);
  } catch {
    // localStorage unavailable -- still applies for this session.
  }
}
