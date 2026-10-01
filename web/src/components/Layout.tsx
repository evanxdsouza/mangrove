import { useEffect, useRef, useState, type ReactNode } from "react";
import { Link, useRouter } from "../router";
import { api, type CurrentUser, type Deployment, type Project } from "../api";
import { useUiMode } from "../uiMode";
import { useWorkspaces } from "../workspaceContext";
import {
  CabinetIcon,
  ChevronDownIcon,
  CloseIcon,
  DeployIcon,
  DialsIcon,
  GaugeIcon,
  GearIcon,
  LedgerIcon,
  MangroveIcon,
  MenuIcon,
  SearchIcon,
  UserIcon,
} from "../icons";

const SIDEBAR_COLLAPSE_KEY = "mangrove-sidebar-collapsed";

function readStoredCollapsed(): boolean {
  if (typeof window === "undefined") return false;
  try {
    return window.localStorage.getItem(SIDEBAR_COLLAPSE_KEY) === "1";
  } catch {
    return false;
  }
}

export function Layout({ user, onLogout, children }: { user: CurrentUser; onLogout: () => void; children: ReactNode }) {
  const { path } = useRouter();
  const { mode, setMode } = useUiMode();
  const [mobileOpen, setMobileOpen] = useState(false);
  const [collapsed, setCollapsedState] = useState(readStoredCollapsed);
  // Explicit membership, not "everything else" -- the old inverse check
  // (any path that isn't one of the other known pages) meant an unmatched
  // path, e.g. a 404, still lit up "Projects" as the active nav item.
  const onProjects = path === "/" || path.startsWith("/projects/");
  const simple = mode === "simple";
  const isOwner = user.role === "owner";
  const critical = useHostCritical(isOwner);

  const setCollapsed = (next: boolean) => {
    setCollapsedState(next);
    try {
      window.localStorage.setItem(SIDEBAR_COLLAPSE_KEY, next ? "1" : "0");
    } catch {
      // localStorage unavailable -- still works for this session
    }
  };

  // A navigation closes the mobile drawer it was reached through, same as
  // any mobile off-canvas menu.
  useEffect(() => setMobileOpen(false), [path]);

  return (
    <div className="app-shell">
      <aside className={`sidebar ${mobileOpen ? "mobile-open" : ""} ${collapsed ? "collapsed" : ""}`}>
        <div className="sidebar-topbar">
          <div className="sidebar-brand">
            <span style={{ display: "inline-flex" }} title={critical ? "Host resources critical -- see Admin" : undefined}>
              <MangroveIcon className={`sidebar-brand-mark ${critical ? "sidebar-brand-mark-critical" : ""}`} />
            </span>
            <span className="sidebar-brand-label">Mangrove</span>
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
          {!simple && <StationSwitcher collapsed={collapsed} />}
          {!simple && <SidebarSearch collapsed={collapsed} onRequestExpand={() => setCollapsed(false)} />}

          {!simple && <div className="nav-section-label">Workspace</div>}
          <Link to="/" className={`nav-link ${onProjects ? "active" : ""}`} title={collapsed ? (simple ? "Your apps" : "Projects") : undefined}>
            <LedgerIcon />
            <span className="nav-link-label">{simple ? "Your apps" : "Projects"}</span>
          </Link>

          {/* Admin (users, ports, tokens, pruning, server health) stays
              technical-only -- it's out of scope for a non-technical user by
              definition, so simple mode just doesn't offer an entry point. */}
          {!simple && (
            <>
              <div className="nav-section-label">Host</div>
              <Link to="/admin" className={`nav-link ${path === "/admin" ? "active" : ""}`} title={collapsed ? "Admin" : undefined}>
                <DialsIcon />
                <span className="nav-link-label">Admin</span>
              </Link>
              <Link
                to="/server-health"
                className={`nav-link ${path === "/server-health" ? "active" : ""}`}
                title={collapsed ? "Server health" : undefined}
              >
                <GaugeIcon />
                <span className="nav-link-label">Server health</span>
              </Link>
              {/* Storage/NAS sharing mounts host drives and creates SMB
                  shares with plaintext credentials -- system-level,
                  owner-only on the API itself (see
                  internal/api/router.go's /storage group), so the nav
                  entry point matches: hidden for a member the same way
                  Admin is hidden in simple mode, just gated on role
                  instead of mode. */}
              {isOwner && (
                <Link to="/storage" className={`nav-link ${path === "/storage" ? "active" : ""}`} title={collapsed ? "Storage" : undefined}>
                  <CabinetIcon />
                  <span className="nav-link-label">Storage</span>
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
            <div className="sidebar-user" title={collapsed ? (user.email ?? `user #${user.id}`) : undefined}>
              <UserIcon />
              <span>{user.email ?? `user #${user.id}`}</span>
            </div>
            <Link
              to="/settings"
              className={`nav-link ${path === "/settings" ? "active" : ""}`}
              style={{ marginBottom: 8 }}
              title={collapsed ? "Settings" : undefined}
            >
              <GearIcon />
              <span className="nav-link-label">Settings</span>
            </Link>
            <button className="btn btn-sm" onClick={onLogout} style={{ width: "100%" }} title={collapsed ? "Log out" : undefined}>
              {collapsed ? <UserIcon style={{ width: 14, height: 14 }} /> : "Log out"}
            </button>
            {!simple && (
              <button
                type="button"
                className="sidebar-collapse-toggle"
                aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
                title={collapsed ? "Expand sidebar" : "Collapse sidebar"}
                onClick={() => setCollapsed(!collapsed)}
              >
                <ChevronDownIcon />
                <span className="sidebar-collapse-toggle-label">Collapse</span>
              </button>
            )}
          </div>
        </div>
      </aside>
      <main className="main">{children}</main>
    </div>
  );
}

// The sidebar mark reacts to real host state instead of being a static
// mark: an owner-only poll of the same resource budget Admin already
// shows, tinting the mark to signal "something needs you" without
// requiring a visit to Admin first. Members never see this (the endpoint
// is owner-only server-side anyway -- see docs/multi-user.md).
// A 5-minute interval, not a tight poll: ComputeResourceBudget's disk-usage
// scan is genuinely expensive (observed ~7s on a modestly-sized data dir in
// manual testing), so this mirrors the backend's own resource_sampler
// cadence rather than adding a second, more frequent caller of the same
// costly endpoint from every page in the app.
function useHostCritical(isOwner: boolean): boolean {
  const [critical, setCritical] = useState(false);

  useEffect(() => {
    if (!isOwner) return;
    let cancelled = false;
    const check = () => {
      api
        .get<{ memory_used_mb: number; memory_ceiling_mb: number; disk_used_gb: number; disk_total_gb: number }>(
          "/api/admin/resource-budget",
        )
        .then((b) => {
          if (cancelled) return;
          const memFrac = b.memory_used_mb / Math.max(b.memory_ceiling_mb, 1);
          const diskFrac = b.disk_total_gb > 0 ? b.disk_used_gb / Math.max(b.disk_total_gb, 1) : 0;
          setCritical(memFrac > 0.9 || diskFrac > 0.9);
        })
        .catch(() => {});
    };
    check();
    const interval = setInterval(check, 300_000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [isOwner]);

  return critical;
}

// Jump straight to a project by name/slug from anywhere in the technical
// dashboard, without detouring through the list first -- Ctrl/Cmd+K
// focuses it from any page, matching the muscle memory of every other
// command-palette-style search. Client-side only: one /api/projects call
// (every workspace, not scoped to the active station) cached for the
// component's lifetime, no new endpoint.
interface SearchMatch {
  kind: "project" | "deployment";
  id: number;
  name: string;
  slug: string;
  projectId: number;
  projectName?: string; // only set for a deployment match
}

function SidebarSearch({ collapsed, onRequestExpand }: { collapsed: boolean; onRequestExpand: () => void }) {
  const { navigate } = useRouter();
  const [projects, setProjects] = useState<Project[] | null>(null);
  // Deployments are fetched lazily, the first time a query actually needs
  // them -- an eager fetch-per-project on every sidebar mount would be a
  // real N+1 cost most visits never use (most sidebar renders never open
  // search at all).
  const [deploymentsByProject, setDeploymentsByProject] = useState<Record<number, Deployment[]> | null>(null);
  const [loadingDeployments, setLoadingDeployments] = useState(false);
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(0);
  // When Ctrl/Cmd+K fires while the sidebar is collapsed (icon-only), the
  // input isn't focusable until the sidebar re-expands -- this flag defers
  // the actual focus() call to the effect below, which fires once the
  // expand has actually re-rendered the real input into the DOM.
  const [focusPending, setFocusPending] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    api
      .get<Project[]>("/api/projects")
      .then((p) => setProjects(p ?? []))
      .catch(() => setProjects([]));
  }, []);

  const ensureDeploymentsLoaded = () => {
    if (deploymentsByProject != null || loadingDeployments || projects == null) return;
    setLoadingDeployments(true);
    Promise.all(
      projects.map((p) =>
        api
          .get<Deployment[]>(`/api/projects/${p.id}/deployments`)
          .then((d): [number, Deployment[]] => [p.id, d ?? []])
          .catch((): [number, Deployment[]] => [p.id, []]),
      ),
    ).then((entries) => {
      setDeploymentsByProject(Object.fromEntries(entries));
      setLoadingDeployments(false);
    });
  };

  useEffect(() => {
    if (!collapsed && focusPending) {
      inputRef.current?.focus();
      inputRef.current?.select();
      setFocusPending(false);
    }
  }, [collapsed, focusPending]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        if (collapsed) {
          onRequestExpand();
          setFocusPending(true);
        } else {
          inputRef.current?.focus();
          inputRef.current?.select();
        }
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [collapsed, onRequestExpand]);

  useEffect(() => {
    if (!open) return;
    const onDocClick = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDocClick);
    return () => document.removeEventListener("mousedown", onDocClick);
  }, [open]);

  const q = query.trim().toLowerCase();
  const projectNameById = new Map((projects ?? []).map((p) => [p.id, p.name]));
  const matches: SearchMatch[] = q
    ? [
        ...(projects ?? [])
          .filter((p) => p.name.toLowerCase().includes(q) || p.slug.toLowerCase().includes(q))
          .map((p): SearchMatch => ({ kind: "project", id: p.id, name: p.name, slug: p.slug, projectId: p.id })),
        ...Object.entries(deploymentsByProject ?? {}).flatMap(([projectId, deps]) =>
          deps
            .filter((d) => d.name.toLowerCase().includes(q) || d.slug.toLowerCase().includes(q))
            .map(
              (d): SearchMatch => ({
                kind: "deployment",
                id: d.id,
                name: d.name,
                slug: d.slug,
                projectId: Number(projectId),
                projectName: projectNameById.get(Number(projectId)),
              }),
            ),
        ),
      ].slice(0, 8)
    : [];

  const go = (m: SearchMatch) => {
    setQuery("");
    setOpen(false);
    inputRef.current?.blur();
    navigate(m.kind === "project" ? `/projects/${m.projectId}` : `/projects/${m.projectId}/deployments/${m.id}`);
  };

  if (collapsed) {
    return (
      <button
        type="button"
        className="nav-link sidebar-search-collapsed-btn"
        title="Search (Ctrl/Cmd+K)"
        onClick={() => {
          onRequestExpand();
          setFocusPending(true);
        }}
      >
        <SearchIcon />
      </button>
    );
  }

  return (
    <div className="sidebar-search" ref={rootRef}>
      <SearchIcon className="sidebar-search-icon" />
      <input
        ref={inputRef}
        className="sidebar-search-input"
        placeholder="Search projects and deployments"
        aria-label="Search projects and deployments"
        value={query}
        onChange={(e) => {
          setQuery(e.target.value);
          setActiveIndex(0);
          setOpen(true);
          if (e.target.value.trim()) ensureDeploymentsLoaded();
        }}
        onFocus={() => {
          if (query) {
            setOpen(true);
            ensureDeploymentsLoaded();
          }
        }}
        onKeyDown={(e) => {
          if (e.key === "Escape") {
            setOpen(false);
            inputRef.current?.blur();
          } else if (e.key === "ArrowDown" && matches.length > 0) {
            e.preventDefault();
            setActiveIndex((i) => (i + 1) % matches.length);
          } else if (e.key === "ArrowUp" && matches.length > 0) {
            e.preventDefault();
            setActiveIndex((i) => (i - 1 + matches.length) % matches.length);
          } else if (e.key === "Enter" && matches[activeIndex]) {
            go(matches[activeIndex]);
          }
        }}
      />
      {!query && <span className="sidebar-search-kbd">&#8984;K</span>}
      {open && query && (
        <div className="sidebar-search-panel">
          {matches.length === 0 ? (
            <div className="sidebar-search-empty">
              {loadingDeployments ? "Searching..." : <>No projects or deployments match &ldquo;{query}&rdquo;.</>}
            </div>
          ) : (
            matches.map((m, i) => (
              <button
                key={`${m.kind}-${m.id}`}
                type="button"
                className={`sidebar-search-item ${i === activeIndex ? "active" : ""}`}
                onMouseEnter={() => setActiveIndex(i)}
                onClick={() => go(m)}
              >
                {m.kind === "project" ? (
                  <LedgerIcon style={{ width: 14, height: 14, flexShrink: 0, color: "var(--text-faint)" }} />
                ) : (
                  <DeployIcon style={{ width: 14, height: 14, flexShrink: 0, color: "var(--text-faint)" }} />
                )}
                {m.name}
                <span className="sidebar-search-item-slug">{m.kind === "deployment" ? m.projectName : m.slug}</span>
              </button>
            ))
          )}
        </div>
      )}
    </div>
  );
}

// The station switcher fixes "workspaces isn't where it's supposed to
// be": rather than a flat, orphaned nav item that detours to its own page,
// the active workspace is ambient context pinned at the top of the
// sidebar, always visible, changeable in one click from anywhere in the
// technical dashboard -- see workspaceContext.tsx.
function StationSwitcher({ collapsed }: { collapsed: boolean }) {
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
        title={collapsed ? label : undefined}
        onClick={() => setOpen((v) => !v)}
      >
        <MangroveIcon className="station-switcher-icon" />
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
