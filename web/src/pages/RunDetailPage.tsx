import { useEffect, useState } from "react";
import { api, ApiError, type DeployHistory, type Deployment } from "../api";
import { Link } from "../router";
import { StatusPill } from "../components/StatusPill";
import { CodeBlock } from "../components/CodeBlock";
import { CenterLoading } from "../components/CenterLoading";
import { CheckCircleIcon, EmptyLedgerIcon, XCircleIcon } from "../icons";
import { fmtDuration, fmtWhenFull } from "../lib/format";
import { useToast } from "../components/Toast";
import { firstTime } from "../lib/milestones";

// One run, its own page -- the "run/log page" pattern from the reference:
// breadcrumb back to the deployment, a header with status/who/when/duration,
// then the run rendered as a single labeled step block ending in an
// explicit result. Mangrove's data model doesn't have discrete named build
// steps or a persisted stdout/exit-code for a *successful* run (only
// `error_message` on failure), so this shows one step -- the deploy itself
// -- rather than fabricating a multi-step breakdown or fake output.
export function RunDetailPage({
  projectId,
  deploymentId,
  historyId,
}: {
  projectId: number;
  deploymentId: number;
  historyId: number;
}) {
  const [projectName, setProjectName] = useState<string | null>(null);
  const [deployment, setDeployment] = useState<Deployment | null>(null);
  const [history, setHistory] = useState<DeployHistory[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [rollbackBusy, setRollbackBusy] = useState(false);
  const { showToast } = useToast();

  const load = () => {
    api
      .get<Deployment>(`/api/deployments/${deploymentId}`)
      .then(setDeployment)
      .catch((e) => setError(errMsg(e)));
    api
      .get<DeployHistory[]>(`/api/deployments/${deploymentId}/history`)
      .then((h) => setHistory(h ?? []))
      .catch((e) => setError(errMsg(e)));
  };

  useEffect(() => {
    load();
    const interval = setInterval(load, 4000);
    return () => clearInterval(interval);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [deploymentId]);

  useEffect(() => {
    api
      .get<{ name: string }>(`/api/projects/${projectId}`)
      .then((p) => setProjectName(p.name))
      .catch(() => {});
  }, [projectId]);

  const index = history?.findIndex((h) => h.id === historyId) ?? -1;
  const entry = index >= 0 ? history![index] : null;
  const runNumber = history && index >= 0 ? history.length - index : null;

  const rollback = async () => {
    if (!entry) return;
    setRollbackBusy(true);
    setError(null);
    try {
      await api.post(`/api/deploy-history/${entry.id}/rollback`, {});
      if (firstTime("first-rollback")) {
        showToast("First rollback complete. Good instinct, keeping a healthy version one click away.");
      }
      load();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setRollbackBusy(false);
    }
  };

  if (history === null || deployment === null) {
    return <CenterLoading />;
  }

  if (!entry) {
    return (
      <>
        <div className="breadcrumb">
          <Link to="/">Projects</Link> / <Link to={`/projects/${projectId}`}>{projectName ?? "..."}</Link> /{" "}
          <Link to={`/projects/${projectId}/deployments/${deploymentId}`}>{deployment.name}</Link> / Run
        </div>
        <div className="card empty-state">
          <EmptyLedgerIcon />
          <p>This run no longer exists -- deploy history is pruned over time.</p>
        </div>
      </>
    );
  }

  const duration = fmtDuration(entry.started_at, entry.finished_at);
  const succeeded = entry.status === "success";
  const failed = entry.status === "failed" || entry.status === "timeout";

  return (
    <>
      <div className="breadcrumb">
        <Link to="/">Projects</Link> / <Link to={`/projects/${projectId}`}>{projectName ?? "..."}</Link> /{" "}
        <Link to={`/projects/${projectId}/deployments/${deploymentId}`}>{deployment.name}</Link> / Run #{runNumber}
      </div>
      <div className="page-header">
        <div>
          <h1 className="flex gap-8" style={{ alignItems: "center" }}>
            Run #{runNumber} <StatusPill status={entry.status} />
            {entry.is_current && <span className="pill pill-gray">current</span>}
          </h1>
          <p>
            {entry.triggered_by}
            {entry.commit_sha ? ` · ${entry.commit_sha.slice(0, 8)}` : ""} &middot; {fmtWhenFull(entry.started_at)}
            {duration ? ` · ${duration}` : ""}
          </p>
        </div>
        {entry.status === "success" && !entry.is_current && (
          <button className="btn" onClick={rollback} disabled={rollbackBusy}>
            {rollbackBusy ? "Rolling back..." : "Revert to this"}
          </button>
        )}
      </div>

      {error && <div className="error-banner">{error}</div>}

      <div className="card run-step-card">
        <div className="run-step-tag">{deployment.build_strategy === "compose" ? "COMPOSE" : "DEPLOY"}</div>
        <div className="run-step-row">
          {succeeded ? (
            <CheckCircleIcon className="run-step-icon run-step-icon-success" />
          ) : failed ? (
            <XCircleIcon className="run-step-icon run-step-icon-failed" />
          ) : (
            <StatusPill status={entry.status} />
          )}
          <span className="run-step-title">{entry.commit_message || entry.git_ref || deployment.name}</span>
          <span className="mono text-dim">{deployment.build_strategy}</span>
          {duration && <span className="text-dim run-step-duration">{duration}</span>}
        </div>
        {entry.error_message ? (
          <CodeBlock tone="danger">{entry.error_message}</CodeBlock>
        ) : (
          <p className="text-dim run-step-no-output">
            {succeeded
              ? "No build output is kept for a successful run -- see the Logs tab for live container output."
              : "No output captured for this run."}
          </p>
        )}
        {entry.rollback_of_deploy_history_id && (
          <div className="field-hint">Rollback of run #{history.length - history.findIndex((h) => h.id === entry.rollback_of_deploy_history_id)}.</div>
        )}
      </div>
    </>
  );
}

function errMsg(e: unknown): string {
  return e instanceof ApiError ? e.message : "Something went wrong";
}
