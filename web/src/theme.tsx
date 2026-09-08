import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { mangroveLogoMarkup } from "./components/Logo";

// A color theme -- swaps the CSS custom properties in styles.css's
// per-theme [data-theme="..."] blocks, and re-colors the logo (sidebar mark
// + favicon) to match, since the logo is drawn from --accent/--green rather
// than fixed hex values. Semantic colors (--red/--yellow for errors and
// warnings, --text* for body copy) intentionally stay constant across
// themes -- only the brand/accent palette changes, so status pills keep
// meaning a user has already learned.
export interface ThemeDef {
  id: string;
  label: string;
  accent: string;
  green: string;
}

export const THEMES: ThemeDef[] = [
  { id: "ocean", label: "Ocean", accent: "#4f8cff", green: "#3ecf8e" },
  { id: "mangrove", label: "Mangrove", accent: "#3ecf8e", green: "#a8e05f" },
  { id: "tidal", label: "Tidal", accent: "#22b8cf", green: "#3ecf8e" },
  { id: "dusk", label: "Dusk", accent: "#b06bff", green: "#3ecf8e" },
  { id: "amber", label: "Amber", accent: "#ff9457", green: "#3ecf8e" },
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

function applyFavicon(def: ThemeDef) {
  if (typeof document === "undefined") return;
  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
  if (!link) {
    link = document.createElement("link");
    link.rel = "icon";
    link.type = "image/svg+xml";
    document.head.appendChild(link);
  }
  link.href = `data:image/svg+xml,${encodeURIComponent(mangroveLogoMarkup(def.accent, def.green))}`;
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
