import { createContext, useContext, useEffect, useState, type ReactNode } from "react";

// A color theme -- swaps the --brass* instrument-accent variables in
// styles.css's per-theme [data-theme="..."] blocks. Semantic colors
// (--verdigris/--red/--ochre, --text*, --bg*) intentionally stay constant
// across themes -- DESIGN.md's "One Accent Rule" says the accent is never
// duplicated *within* an active theme, and switching between named
// single-accent presets keeps that true at every moment: only one accent
// is ever live. "brass" is the default and needs no override block since
// it matches styles.css's :root values.
export interface ThemeDef {
  id: string;
  label: string;
  accent: string;
}

export const THEMES: ThemeDef[] = [
  { id: "brass", label: "Brass", accent: "#caa057" },
  { id: "copper", label: "Copper", accent: "#c97b4a" },
  { id: "iron", label: "Iron", accent: "#8fa3b0" },
  { id: "silver", label: "Silver", accent: "#c7c2b4" },
  { id: "indigo", label: "Indigo", accent: "#8b7fd1" },
];
const DEFAULT_THEME = THEMES[0].id;
const STORAGE_KEY = "mangrove-theme";

interface ThemeState {
  theme: string;
  setTheme: (id: string) => void;
}

const ThemeContext = createContext<ThemeState | null>(null);

function readStored(): string {
  if (typeof window === "undefined") return DEFAULT_THEME;
  const stored = window.localStorage.getItem(STORAGE_KEY);
  return THEMES.some((t) => t.id === stored) ? stored! : DEFAULT_THEME;
}

// Standalone SVG markup (a literal color, no CSS vars) for the browser-tab
// favicon -- a separate document context that can't see the app's
// stylesheet, so it's regenerated as a data: URI whenever the theme
// changes, keeping the tab icon in sync with the sidebar mark. Mirrors
// MangroveIcon's path data (see icons.tsx) on the same dark rounded-square
// badge as the static web/public/favicon.svg.
function faviconMarkup(accent: string): string {
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><rect width="24" height="24" rx="5" fill="#14100a"/><g fill="none" stroke="${accent}" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="8.3" r="3.3"/><circle cx="8.3" cy="10.3" r="2.4"/><circle cx="15.7" cy="10.3" r="2.4"/><path d="M12 13v3.4"/><path d="M12 16.4c-2.6 1-4.4 2.1-5.4 4.1"/><path d="M12 16.4v4.1"/><path d="M12 16.4c2.6 1 4.4 2.1 5.4 4.1"/></g></svg>`;
}

function applyFavicon(def: ThemeDef) {
  if (typeof document === "undefined") return;
  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
  if (!link) {
    link = document.createElement("link");
    link.rel = "icon";
    link.type = "image/svg+xml";
    document.head.appendChild(link);
  }
  link.href = `data:image/svg+xml,${encodeURIComponent(faviconMarkup(def.accent))}`;
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<string>(readStored);

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
    applyFavicon(THEMES.find((t) => t.id === theme) ?? THEMES[0]);
  }, [theme]);

  const setTheme = (id: string) => {
    setThemeState(id);
    try {
      window.localStorage.setItem(STORAGE_KEY, id);
    } catch {
      // localStorage unavailable (private browsing etc.) -- theme still
      // applies for this session, just doesn't persist across reloads.
    }
  };

  return <ThemeContext.Provider value={{ theme, setTheme }}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeState {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error("useTheme must be used within a ThemeProvider");
  return ctx;
}
