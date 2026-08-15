import type { ReactNode } from "react";
import { Link, useRouter } from "../router";
import type { CurrentUser } from "../api";
import { useUiMode } from "../uiMode";

export function Layout({ user, onLogout, children }: { user: CurrentUser; onLogout: () => void; children: ReactNode }) {
  const { path } = useRouter();
  const { mode, setMode } = useUiMode();
  const onAdmin = path === "/admin" || path === "/server-health";
  const simple = mode === "simple";

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="sidebar-brand">
          <span className="sidebar-brand-mark" />
          Mangrove
        </div>
        <Link to="/" className={`nav-link ${!onAdmin ? "active" : ""}`}>
          {simple ? "Your apps" : "Projects"}
        </Link>
        {!simple && (
          <Link to="/workspaces" className={`nav-link ${path === "/workspaces" ? "active" : ""}`}>
            Workspaces
          </Link>
        )}
        {/* Admin (users, ports, tokens, pruning, server health) stays
            technical-only -- it's out of scope for a non-technical user by
            definition, so simple mode just doesn't offer an entry point. */}
        {!simple && (
          <Link to="/admin" className={`nav-link ${onAdmin ? "active" : ""}`}>
            Admin
          </Link>
        )}
        {!simple && (
          <Link to="/server-health" className={`nav-link ${path === "/server-health" ? "active" : ""}`}>
            Server health
          </Link>
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
          <div style={{ margin: "10px 0 8px" }}>{user.email ?? `user #${user.id}`}</div>
          <button className="btn btn-sm" onClick={onLogout} style={{ width: "100%" }}>
            Log out
          </button>
        </div>
      </aside>
      <main className="main">{children}</main>
    </div>
  );
}
