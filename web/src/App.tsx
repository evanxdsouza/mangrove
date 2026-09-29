import { useEffect, useState, useCallback, type ReactElement } from "react";
import { api, ApiError, type AuthStatus, type CurrentUser } from "./api";
import { Router, useRouter, matchPath } from "./router";
import { Layout } from "./components/Layout";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { LoginPage } from "./pages/LoginPage";
import { ProjectsPage } from "./pages/ProjectsPage";
import { WorkspacesPage } from "./pages/WorkspacesPage";
import { ProjectDetailPage } from "./pages/ProjectDetailPage";
import { DeploymentDetailPage } from "./pages/DeploymentDetailPage";
import { RunDetailPage } from "./pages/RunDetailPage";
import { AdminPage } from "./pages/AdminPage";
import { SettingsPage } from "./pages/SettingsPage";
import { ServerHealthPage } from "./pages/ServerHealthPage";
import { StoragePage } from "./pages/StoragePage";
import { SimpleAppsPage } from "./pages/simple/SimpleAppsPage";
import { SimpleAppDetailPage } from "./pages/simple/SimpleAppDetailPage";
import { NotFoundPage } from "./pages/NotFoundPage";
import { UserProvider } from "./userContext";
import { UiModeProvider, useUiMode } from "./uiMode";
import { WorkspaceProvider } from "./workspaceContext";
import { ThemeProvider } from "./theme";
import { ToastProvider } from "./components/Toast";

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
  const { mode } = useUiMode();

  let body: ReactElement;
  let projectParams = matchPath("/projects/:projectId", path);
  let deploymentParams = matchPath("/projects/:projectId/deployments/:deploymentId", path);
  let runParams = matchPath("/projects/:projectId/deployments/:deploymentId/history/:historyId", path);

  if (runParams) {
    const projectId = Number(runParams.projectId);
    const deploymentId = Number(runParams.deploymentId);
    // Deploy history is a technical-mode concept -- simple mode's flattened
    // app view has no history tab to link a run out of, so a stray link
    // here just lands on the app's own page, same as /workspaces bouncing
    // to the apps list.
    body =
      mode === "simple" ? (
        <SimpleAppDetailPage deploymentId={deploymentId} />
      ) : (
        <RunDetailPage projectId={projectId} deploymentId={deploymentId} historyId={Number(runParams.historyId)} />
      );
  } else if (deploymentParams) {
    const projectId = Number(deploymentParams.projectId);
    const deploymentId = Number(deploymentParams.deploymentId);
    body =
      mode === "simple" ? (
        <SimpleAppDetailPage deploymentId={deploymentId} />
      ) : (
        <DeploymentDetailPage projectId={projectId} deploymentId={deploymentId} />
      );
  } else if (projectParams) {
    // Simple mode has no notion of a "project" grouping page -- an app is
    // a deployment, reached straight from the apps list. A stray link
    // into a bare project page (e.g. an old bookmark) just bounces to
    // the apps list rather than rendering a page simple mode has no
    // equivalent for.
    body = mode === "simple" ? <SimpleAppsPage /> : <ProjectDetailPage projectId={Number(projectParams.projectId)} />;
  } else if (path === "/workspaces") {
    // Workspaces are a technical grouping concept; simple mode has no
    // equivalent page, so a direct navigation bounces to the apps list.
    body = mode === "simple" ? <SimpleAppsPage /> : <WorkspacesPage />;
  } else if (path === "/admin") {
    // Admin stays technical-only regardless of mode (see Layout, which
    // already hides its nav entry point in simple mode) -- someone who
    // navigates here directly still gets the real admin page rather than
    // a dead end.
    body = <AdminPage />;
  } else if (path === "/server-health") {
    // Whole-host status: CPU/RAM/disk/load of the machine, not just
    // Mangrove containers. Technical-only for the same reason as /admin.
    body = <ServerHealthPage />;
  } else if (path === "/storage") {
    // Storage/NAS sharing is host-level (mounting drives) and
    // owner-gated on the API itself -- StoragePage shows its own
    // "owner-only" message for a member, same as /admin's routes would
    // 403. Technical-only, same reasoning as /admin.
    body = <StoragePage />;
  } else if (path === "/settings") {
    // Account settings (change password) are relevant regardless of mode
    // or role -- every user has an account and a password to rotate.
    body = <SettingsPage />;
  } else if (path === "/") {
    body = mode === "simple" ? <SimpleAppsPage /> : <ProjectsPage />;
  } else {
    // Anything else is a genuinely unmatched path -- this used to fall
    // through to the Projects/Your-apps list unconditionally, which made
    // a bad link (a stale bookmark, a typo, a removed route) look like it
    // silently worked instead of saying it didn't.
    body = <NotFoundPage path={path} />;
  }

  return (
    <UserProvider user={user}>
      <WorkspaceProvider>
        <Layout user={user} onLogout={onLogout}>
          <ErrorBoundary key={path}>{body}</ErrorBoundary>
        </Layout>
      </WorkspaceProvider>
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
      <ThemeProvider>
        <ToastProvider>
          <UiModeProvider>
            <AppInner />
          </UiModeProvider>
        </ToastProvider>
      </ThemeProvider>
    </Router>
  );
}

export { ApiError };
