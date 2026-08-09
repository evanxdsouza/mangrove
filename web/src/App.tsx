import { useEffect, useState, useCallback, type ReactElement } from "react";
import { api, ApiError, type AuthStatus, type CurrentUser } from "./api";
import { Router, useRouter, matchPath } from "./router";
import { Layout } from "./components/Layout";
import { LoginPage } from "./pages/LoginPage";
import { ProjectsPage } from "./pages/ProjectsPage";
import { ProjectDetailPage } from "./pages/ProjectDetailPage";
import { DeploymentDetailPage } from "./pages/DeploymentDetailPage";
import { AdminPage } from "./pages/AdminPage";
import { UserProvider } from "./userContext";

type AuthState =
  | { kind: "loading" }
  | { kind: "needs-setup" }
  | { kind: "needs-login" }
  | { kind: "authenticated"; user: CurrentUser };

function useAuthState() {
  const [state, setState] = useState<AuthState>({ kind: "loading" });

  const refresh = useCallback(async () => {
    try {
      const status = await api.get<AuthStatus>("/api/auth/status");
      if (status.setup_required) {
        setState({ kind: "needs-setup" });
        return;
      }
      try {
        const me = await api.get<CurrentUser>("/api/auth/me");
        setState({ kind: "authenticated", user: me });
      } catch {
        setState({ kind: "needs-login" });
      }
    } catch (e) {
      // API unreachable -- surface as needs-login so the user sees
      // something actionable rather than an infinite spinner.
      setState({ kind: "needs-login" });
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { state, refresh };
}

function AppRoutes({ user, onLogout }: { user: CurrentUser; onLogout: () => void }) {
  const { path } = useRouter();

  let body: ReactElement;
  let projectParams = matchPath("/projects/:projectId", path);
  let deploymentParams = matchPath("/projects/:projectId/deployments/:deploymentId", path);

  if (deploymentParams) {
    body = (
      <DeploymentDetailPage
        projectId={Number(deploymentParams.projectId)}
        deploymentId={Number(deploymentParams.deploymentId)}
      />
    );
  } else if (projectParams) {
    body = <ProjectDetailPage projectId={Number(projectParams.projectId)} />;
  } else if (path === "/admin") {
    body = <AdminPage />;
  } else {
    body = <ProjectsPage />;
  }

  return (
    <UserProvider user={user}>
      <Layout user={user} onLogout={onLogout}>
        {body}
      </Layout>
    </UserProvider>
  );
}

function AppInner() {
  const { state, refresh } = useAuthState();

  if (state.kind === "loading") {
    return (
      <div className="center-loading">
        <div className="spinner" />
      </div>
    );
  }

  if (state.kind === "needs-setup" || state.kind === "needs-login") {
    return <LoginPage mode={state.kind === "needs-setup" ? "setup" : "login"} onSuccess={refresh} />;
  }

  const handleLogout = async () => {
    try {
      await api.post("/api/auth/logout");
    } catch {
      // even if the request fails, drop the client-side session state
    }
    refresh();
  };

  return <AppRoutes user={state.user} onLogout={handleLogout} />;
}

export default function App() {
  return (
    <Router>
      <AppInner />
    </Router>
  );
}

export { ApiError };
