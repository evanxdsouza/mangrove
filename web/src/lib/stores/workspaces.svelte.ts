import { api, type WorkspaceProjectCount, type WorkspaceRole } from "../api";

// The active workspace is ambient context for the whole technical-mode
// dashboard, not a one-off filter on the Projects page's URL -- selecting
// one here (from the sidebar's station switcher) is what the Projects list
// reads, so switching "which workspace am I looking at" never requires a
// detour through a separate Workspaces page. Persisted like ui-mode's
// UiMode, so the choice survives a reload instead of resetting to "all".
const STORAGE_KEY = "mangrove-active-workspace";

function readStored(): number | null {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    return raw ? Number(raw) : null;
  } catch {
    return null;
  }
}

export const workspaceState = $state<{
  workspaces: WorkspaceProjectCount[];
  activeWorkspaceId: number | null; // null = "All workspaces"
}>({ workspaces: [], activeWorkspaceId: readStored() });

export function reloadWorkspaces() {
  api
    .get<WorkspaceProjectCount[]>("/api/workspaces")
    .then((ws) => {
      workspaceState.workspaces = ws ?? [];
      // A workspace picked up from storage that no longer exists (deleted
      // elsewhere) quietly falls back to "All workspaces" instead of
      // filtering the Projects list down to nothing with no visible cause.
      const id = workspaceState.activeWorkspaceId;
      if (id != null && workspaceState.workspaces.length > 0 && !workspaceState.workspaces.some((w) => w.workspace.id === id)) {
        workspaceState.activeWorkspaceId = null;
      }
    })
    .catch(() => {
      workspaceState.workspaces = [];
    });
}

export function setActiveWorkspaceId(id: number | null) {
  workspaceState.activeWorkspaceId = id;
  try {
    if (id == null) window.localStorage.removeItem(STORAGE_KEY);
    else window.localStorage.setItem(STORAGE_KEY, String(id));
  } catch {
    // localStorage unavailable -- selection still works for this session.
  }
}

// The caller's role in a specific workspace (not necessarily the active
// one -- a project/deployment page cares about the workspace *that
// resource* lives in), read from the already-loaded list. "" covers both
// "no membership" and "workspace not known yet", so callers gating on
// `=== "admin"` fail closed the same way the API would. Cosmetic only: the
// real boundary is server-side (auth.RequireWorkspaceRole).
export function workspaceRole(workspaceId: number | null | undefined): WorkspaceRole | "" {
  if (workspaceId == null) return "";
  const found = workspaceState.workspaces.find((w) => w.workspace.id === workspaceId);
  return (found?.your_role as WorkspaceRole) || "";
}
