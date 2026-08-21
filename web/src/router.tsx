import { createContext, useContext, useEffect, useState, type CSSProperties, type ReactNode } from "react";

// A minimal client-side router -- deliberately hand-rolled instead of
// pulling in react-router-dom. The dashboard only has a handful of views
// (login, projects, project detail, deployment detail, admin), and
// react-router-dom's current major version carries an open CSRF advisory
// in a code path (RSC/server-action mode) this SPA never exercises;
// avoiding the dependency avoids the audit noise entirely rather than
// pinning to a stale patched version.

interface RouterState {
  path: string;
  navigate: (to: string) => void;
}

const RouterContext = createContext<RouterState | null>(null);

export function Router({ children }: { children: ReactNode }) {
  const [path, setPath] = useState(window.location.pathname);

  useEffect(() => {
    const onPopState = () => setPath(window.location.pathname);
    window.addEventListener("popstate", onPopState);
    return () => window.removeEventListener("popstate", onPopState);
  }, []);

  const navigate = (to: string) => {
    window.history.pushState({}, "", to);
    setPath(to);
  };

  return <RouterContext.Provider value={{ path, navigate }}>{children}</RouterContext.Provider>;
}

export function useRouter(): RouterState {
  const ctx = useContext(RouterContext);
  if (!ctx) throw new Error("useRouter must be used within a Router");
  return ctx;
}

export function Link({
  to,
  children,
  className,
  style,
}: {
  to: string;
  children: ReactNode;
  className?: string;
  style?: CSSProperties;
}) {
  const { navigate } = useRouter();
  return (
    <a
      href={to}
      className={className}
      style={style}
      onClick={(e) => {
        e.preventDefault();
        navigate(to);
      }}
    >
      {children}
    </a>
  );
}

/** Matches "/projects/:id" style patterns against the current path. */
export function matchPath(pattern: string, path: string): Record<string, string> | null {
  const patternParts = pattern.split("/").filter(Boolean);
  const pathParts = path.split("/").filter(Boolean);
  if (patternParts.length !== pathParts.length) return null;

  const params: Record<string, string> = {};
  for (let i = 0; i < patternParts.length; i++) {
    const p = patternParts[i];
    if (p.startsWith(":")) {
      params[p.slice(1)] = decodeURIComponent(pathParts[i]);
    } else if (p !== pathParts[i]) {
      return null;
    }
  }
  return params;
}
