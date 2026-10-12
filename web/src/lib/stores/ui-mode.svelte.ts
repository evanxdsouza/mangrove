// Two fully separate front-end flows share one backend: "technical" is the
// existing dashboard (projects/deployments/build strategies/env vars),
// "simple" is a parallel plain-language flow for non-technical users
// (apps, no jargon) built entirely on the same API. Persisted so the
// choice survives a reload rather than resetting to technical every time.
export type UiMode = "technical" | "simple";
const STORAGE_KEY = "mangrove-ui-mode";

function readStored(): UiMode {
  try {
    return window.localStorage.getItem(STORAGE_KEY) === "simple" ? "simple" : "technical";
  } catch {
    return "technical";
  }
}

export const uiMode = $state<{ mode: UiMode }>({ mode: readStored() });

export function setMode(next: UiMode) {
  uiMode.mode = next;
  try {
    window.localStorage.setItem(STORAGE_KEY, next);
  } catch {
    // localStorage unavailable (private browsing etc.) -- mode still works
    // for this session, just doesn't persist across reloads.
  }
}
