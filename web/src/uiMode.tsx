import { createContext, useContext, useState, type ReactNode } from "react";

// Two fully separate front-end flows share one backend: "technical" is the
// existing dashboard (projects/deployments/build strategies/env vars),
// "simple" is a parallel plain-language flow for non-technical users
// (apps, no jargon) built entirely on the same API. Persisted so the
// choice survives a reload rather than resetting to technical every time.
export type UiMode = "technical" | "simple";
const STORAGE_KEY = "mangrove-ui-mode";

interface UiModeState {
  mode: UiMode;
  setMode: (mode: UiMode) => void;
}

const UiModeContext = createContext<UiModeState | null>(null);

function readStored(): UiMode {
  if (typeof window === "undefined") return "technical";
  return window.localStorage.getItem(STORAGE_KEY) === "simple" ? "simple" : "technical";
}

export function UiModeProvider({ children }: { children: ReactNode }) {
  const [mode, setModeState] = useState<UiMode>(readStored);

  const setMode = (next: UiMode) => {
    setModeState(next);
    try {
      window.localStorage.setItem(STORAGE_KEY, next);
    } catch {
      // localStorage unavailable (private browsing etc.) -- mode still
      // works for this session, just doesn't persist across reloads.
    }
  };

  return <UiModeContext.Provider value={{ mode, setMode }}>{children}</UiModeContext.Provider>;
}

export function useUiMode(): UiModeState {
  const ctx = useContext(UiModeContext);
  if (!ctx) throw new Error("useUiMode must be used within a UiModeProvider");
  return ctx;
}
