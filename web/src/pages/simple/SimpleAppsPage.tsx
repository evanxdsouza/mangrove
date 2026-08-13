import { useEffect, useState } from "react";
import { api, ApiError, type Deployment, type Project } from "../../api";
import { useRouter } from "../../router";
import { STATUS_COLORS } from "../../components/StatusPill";
import { AddAppModal } from "../../components/simple/AddAppModal";
import { GithubDeployWizard } from "../../components/GithubDeployWizard";
import { plainStatus } from "./plainCopy";

interface FlatApp {
  deployment: Deployment;
  projectId: number;
}

// Simple mode's home page: every deployment across every project,
// flattened into one plain "your apps" list -- the project grouping
// concept from technical mode never surfaces here.
export function SimpleAppsPage() {
  const { navigate } = useRouter();
  const [apps, setApps] = useState<FlatApp[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [showAdd, setShowAdd] = useState(false);
  const [showGithub, setShowGithub] = useState(false);

  const load = () => {
    api
      .get<Project[]>("/api/projects")
      .then(async (projects) => {
        const lists = await Promise.all(
          (projects ?? []).map((p) =>
            api
              .get<Deployment[]>(`/api/projects/${p.id}/deployments`)
              .then((deps) => (deps ?? []).map((d): FlatApp => ({ deployment: d, projectId: p.id })))
              .catch(() => [] as FlatApp[]),
          ),
        );
        setApps(lists.flat());
      })
      .catch((e) => setError(e instanceof ApiError ? e.message : "Couldn't load your apps"));
  };

  useEffect(load, []);

  return (
    <>
      <div className="page-header">
        <div>
          <h1>Your apps</h1>
          <p>Everything you've set up, in one place.</p>
        </div>
        <div className="flex gap-8">
          <button className="btn" onClick={() => setShowGithub(true)}>
            + From GitHub
          </button>
          <button className="btn btn-primary" onClick={() => setShowAdd(true)}>
            + Add an app
          </button>
        </div>
      </div>

      {error && <div className="error-banner">{error}</div>}

      {apps === null ? (
        <div className="center-loading">
          <div className="spinner" />
        </div>
      ) : apps.length === 0 ? (
        <div className="card empty-state">You haven't added an app yet. Add one to get started.</div>
      ) : (
        <div className="grid grid-2">
          {apps.map(({ deployment, projectId }) => (
            <div
              key={deployment.id}
              className="card card-clickable"
              onClick={() => navigate(`/projects/${projectId}/deployments/${deployment.id}`)}
            >
              <div className="flex-between" style={{ marginBottom: 0 }}>
                <div className="card-title" style={{ margin: 0 }}>
                  {deployment.name}
                </div>
                <span className={`pill pill-${STATUS_COLORS[deployment.status] ?? "gray"}`}>
                  <span className="pill-dot" />
                  {plainStatus(deployment.status)}
                </span>
              </div>
            </div>
          ))}
        </div>
      )}

      {showAdd && (
        <AddAppModal
          onClose={() => setShowAdd(false)}
          onAdded={(projectId, deploymentId) => {
            setShowAdd(false);
            navigate(`/projects/${projectId}/deployments/${deploymentId}`);
          }}
        />
      )}

      {showGithub && (
        <GithubDeployWizard
          onClose={() => setShowGithub(false)}
          onDeployed={(projectId, deploymentId) => {
            setShowGithub(false);
            navigate(`/projects/${projectId}/deployments/${deploymentId}`);
          }}
        />
      )}
    </>
  );
}
