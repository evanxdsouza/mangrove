import { useEffect, useRef, useState, type ReactNode } from "react";
import { Link, useRouter } from "../router";
import type { CurrentUser } from "../api";
import { useUiMode } from "../uiMode";
import { useWorkspaces } from "../workspaceContext";
import {
  CabinetIcon,
  ChevronDownIcon,
  CloseIcon,
  CompassIcon,
  DialsIcon,
  GaugeIcon,
  GearIcon,
  LedgerIcon,
  MenuIcon,
  UserIcon,
} from "../icons";

export function Layout({ user, onLogout, children }: { user: CurrentUser; onLogout: () => void; children: ReactNode }) {
  const { path } = useRouter();
  const { mode, setMode } = useUiMode();
  const [mobileOpen, setMobileOpen] = useState(false);
  const onAdmin = path === "/admin" || path === "/server-health";
  const simple = mode === "simple";
  const isOwner = user.role === "owner";

  // A navigation closes the mobile drawer it was reached through, same as
  // any mobile off-canvas menu.
  useEffect(() => setMobileOpen(false), [path]);

  return (
    <div className="app-shell">
      <aside className={`sidebar ${mobileOpen ? "mobile-open" : ""}`}>
        <div className="sidebar-topbar">
          <div className="sidebar-brand">
            <CompassIcon className="sidebar-brand-mark" />
            Mangrove
          </div>
          <button
            type="button"
            className="sidebar-menu-toggle"
            aria-label={mobileOpen ? "Close menu" : "Open menu"}
            aria-expanded={mobileOpen}
            onClick={() => setMobileOpen((v) => !v)}
          >
            {mobileOpen ? <CloseIcon /> : <MenuIcon />}
          </button>
        </div>

        <div className="sidebar-body">
          {!simple && <StationSwitcher />}

          <Link to="/" className={`nav-link ${!onAdmin && path !== "/workspaces" && path !== "/storage" && path !== "/settings" ? "active" : ""}`}>
            <LedgerIcon />
            {simple ? "Your apps" : "Projects"}
          </Link>

          {/* Admin (users, ports, tokens, pruning, server health) stays
              technical-only -- it's out of scope for a non-technical user by
              definition, so simple mode just doesn't offer an entry point. */}
          {!simple && (
            <>
              <div className="nav-section-label">Host</div>
              <Link to="/admin" className={`nav-link ${path === "/admin" ? "active" : ""}`}>
                <DialsIcon />
                Admin
              </Link>
              <Link to="/server-health" className={`nav-link ${path === "/server-health" ? "active" : ""}`}>
                <GaugeIcon />
                Server health
              </Link>
              {/* Storage/NAS sharing mounts host drives and creates SMB
                  shares with plaintext credentials -- system-level,
                  owner-only on the API itself (see
                  internal/api/router.go's /storage group), so the nav
                  entry point matches: hidden for a member the same way
                  Admin is hidden in simple mode, just gated on role
                  instead of mode. */}
              {isOwner && (
                <Link to="/storage" className={`nav-link ${path === "/storage" ? "active" : ""}`}>
                  <CabinetIcon />
                  Storage
                </Link>
              )}
            </>
          )}

          <div className="sidebar-footer">
            <div className="mode-toggle">
              <button
                type="button"
                className={`mode-toggle-option ${!simple ? "active" : ""}`}
                onClick={() => setMode("technical")}
                title="Full dashboard with technical detail"
              >
                Technical
              </button>
              <button
                type="button"
                className={`mode-toggle-option ${simple ? "active" : ""}`}
                onClick={() => setMode("simple")}
                title="Plain-language view for non-technical use"
              >
                Simple
              </button>
            </div>
            <div className="sidebar-user">
              <UserIcon />
              <span>{user.email ?? `user #${user.id}`}</span>
            </div>
            <Link to="/settings" className={`nav-link ${path === "/settings" ? "active" : ""}`} style={{ marginBottom: 8 }}>
              <GearIcon />
              Settings
            </Link>
            <button className="btn btn-sm" onClick={onLogout} style={{ width: "100%" }}>
              Log out
            </button>
          </div>
        </div>
      </aside>
      <main className="main">{children}</main>
    </div>
  );
}

// The station switcher fixes "workspaces isn't where it's supposed to
// be": rather than a flat, orphaned nav item that detours to its own page,
// the active workspace is ambient context pinned at the top of the
// sidebar, always visible, changeable in one click from anywhere in the
// technical dashboard -- see workspaceContext.tsx.
function StationSwitcher() {
  const { navigate, path } = useRouter();
  const { workspaces, activeWorkspaceId, setActiveWorkspaceId } = useWorkspaces();
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  const active = workspaces.find((w) => w.workspace.id === activeWorkspaceId);
  const label = active ? active.workspace.name : "All workspaces";

  useEffect(() => {
    if (!open) return;
    const onDocClick = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onDocClick);
    window.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDocClick);
      window.removeEventListener("keydown", onKey);
    };
  }, [open]);

  // Closing after a selection made from a different page (e.g. the
  // Workspaces management page) keeps the panel from staying open across
  // a navigation it itself triggered.
  useEffect(() => setOpen(false), [path]);

  const pick = (id: number | null) => {
    setActiveWorkspaceId(id);
    setOpen(false);
    navigate("/");
  };

  return (
    <div className="station-switcher" ref={rootRef}>
      <button
        type="button"
        className="station-switcher-button"
        aria-expanded={open}
        aria-haspopup="listbox"
        onClick={() => setOpen((v) => !v)}
      >
        <CompassIcon className="station-switcher-icon" />
        <span className="station-switcher-labels">
          <span className="station-switcher-eyebrow">Station</span>
          <span className="station-switcher-name">{label}</span>
        </span>
        <ChevronDownIcon className="station-switcher-chevron" />
      </button>

      {open && (
        <div className="station-switcher-panel" role="listbox">
          <button type="button" className={`station-switcher-item ${activeWorkspaceId == null ? "active" : ""}`} onClick={() => pick(null)}>
            <span className="dot" />
            All workspaces
          </button>
          {workspaces.map((w) => (
            <button
              key={w.workspace.id}
              type="button"
              className={`station-switcher-item ${activeWorkspaceId === w.workspace.id ? "active" : ""}`}
              onClick={() => pick(w.workspace.id)}
            >
              <span className="dot" />
              {w.workspace.name}
              <span className="station-switcher-item-count">{w.project_count}</span>
            </button>
          ))}
          <div className="station-switcher-divider" />
          <button
            type="button"
            className="station-switcher-manage"
            onClick={() => {
              setOpen(false);
              navigate("/workspaces");
            }}
          >
            <GearIcon />
            Manage workspaces
          </button>
        </div>
      )}
    </div>
  );
}
