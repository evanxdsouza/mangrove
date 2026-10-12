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
