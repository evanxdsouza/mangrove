import type { ReactNode } from "react";
import { Link, useRouter } from "../router";
import type { CurrentUser } from "../api";

export function Layout({ user, onLogout, children }: { user: CurrentUser; onLogout: () => void; children: ReactNode }) {
  const { path } = useRouter();
  const onAdmin = path === "/admin";

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="sidebar-brand">
          <span className="sidebar-brand-mark" />
          Mangrove
        </div>
        <Link to="/" className={`nav-link ${!onAdmin ? "active" : ""}`}>
          Projects
        </Link>
        <Link to="/admin" className={`nav-link ${onAdmin ? "active" : ""}`}>
          Admin
        </Link>
        <div className="sidebar-footer">
          <div style={{ marginBottom: 8 }}>{user.email ?? `user #${user.id}`}</div>
          <button className="btn btn-sm" onClick={onLogout} style={{ width: "100%" }}>
            Log out
          </button>
        </div>
      </aside>
      <main className="main">{children}</main>
    </div>
  );
}
