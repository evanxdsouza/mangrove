import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { api, type WorkspaceProjectCount } from "./api";

// The active workspace is ambient context for the whole technical-mode
// dashboard, not a one-off filter on the Projects page's URL -- selecting
// one here (from the sidebar's station switcher) is what the Projects list
// reads, so switching "which workspace am I looking at" never requires a
// detour through a separate Workspaces page. Persisted like uiMode.tsx's
// UiMode, so the choice survives a reload instead of resetting to "all"
// every time.
const STORAGE_KEY = "mangrove-active-workspace";

interface WorkspaceState {
  workspaces: WorkspaceProjectCount[];
  activeWorkspaceId: number | null; // null = "All workspaces"
  setActiveWorkspaceId: (id: number | null) => void;
  reload: () => void;
}

const WorkspaceContext = createContext<WorkspaceState | null>(null);

function readStored(): number | null {
  if (typeof window === "undefined") return null;
  const raw = window.localStorage.getItem(STORAGE_KEY);
  return raw ? Number(raw) : null;
}

export function WorkspaceProvider({ children }: { children: ReactNode }) {
  const [workspaces, setWorkspaces] = useState<WorkspaceProjectCount[]>([]);
  const [activeWorkspaceId, setActiveWorkspaceIdState] = useState<number | null>(readStored);

  const reload = () => {
    api
      .get<WorkspaceProjectCount[]>("/api/workspaces")
      .then((ws) => setWorkspaces(ws ?? []))
      .catch(() => setWorkspaces([]));
  };

  useEffect(reload, []);

  // A workspace picked up from storage that no longer exists (deleted
  // elsewhere) quietly falls back to "All workspaces" instead of filtering
  // the Projects list down to nothing with no visible cause.
  useEffect(() => {
    if (activeWorkspaceId == null || workspaces.length === 0) return;
    if (!workspaces.some((w) => w.workspace.id === activeWorkspaceId)) {
      setActiveWorkspaceIdState(null);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspaces]);

  const setActiveWorkspaceId = (id: number | null) => {
    setActiveWorkspaceIdState(id);
    try {
      if (id == null) window.localStorage.removeItem(STORAGE_KEY);
      else window.localStorage.setItem(STORAGE_KEY, String(id));
    } catch {
      // localStorage unavailable (private browsing etc.) -- selection still
      // works for this session, just doesn't persist across reloads.
    }
  };

  return (
    <WorkspaceContext.Provider value={{ workspaces, activeWorkspaceId, setActiveWorkspaceId, reload }}>
      {children}
    </WorkspaceContext.Provider>
  );
}

export function useWorkspaces(): WorkspaceState {
  const ctx = useContext(WorkspaceContext);
  if (!ctx) throw new Error("useWorkspaces must be used within a WorkspaceProvider");
  return ctx;
}
