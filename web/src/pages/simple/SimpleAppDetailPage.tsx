import { useEffect, useState } from "react";
import { api, ApiError, type Deployment } from "../../api";
import { Link, useRouter } from "../../router";
import { ConfirmModal } from "../../components/ConfirmModal";
import { STATUS_COLORS } from "../../components/StatusPill";
import { useIsOwner } from "../../userContext";
import { useUiMode } from "../../uiMode";
import { plainStatus } from "./plainCopy";

// The simple-mode counterpart to DeploymentDetailPage -- no raw env var
// table, no CPU/memory numbers, no build-strategy internals. Just status,
// in plain words, and the two actions a non-technical user actually
// needs: try again if it's broken, remove it if they're done with it.
export function SimpleAppDetailPage({ deploymentId }: { deploymentId: number }) {
  const isOwner = useIsOwner();
  const { setMode } = useUiMode();
  const { navigate } = useRouter();
  const [deployment, setDeployment] = useState<Deployment | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [retrying, setRetrying] = useState(false);
  const [showDelete, setShowDelete] = useState(false);

  const load = () => {
    api.get<Deployment>(`/api/deployments/${deploymentId}`).then(setDeployment).catch((e) => setError(errMsg(e)));
  };
  useEffect(load, [deploymentId]);

  const retry = async () => {
    setRetrying(true);
    setError(null);
    try {
      await api.post(`/api/deployments/${deploymentId}/deploy`, {});
      load();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setRetrying(false);
    }
  };

  return (
    <>
      <div className="breadcrumb">
        <Link to="/">Your apps</Link> / {deployment?.name ?? "..."}
      </div>
      <div className="page-header">
        <div>
          <h1>{deployment?.name ?? "Loading..."}</h1>
          {deployment && (
            <p className="flex gap-8" style={{ alignItems: "center" }}>
              <span className={`pill pill-${STATUS_COLORS[deployment.status] ?? "gray"}`}>
                <span className="pill-dot" />
                {plainStatus(deployment.status)}
              </span>
            </p>
          )}
        </div>
        <div className="flex gap-8">
          {deployment?.status === "failed" && (
            <button className="btn btn-primary" onClick={retry} disabled={retrying}>
              {retrying ? "Trying again..." : "Try again"}
            </button>
          )}
          {isOwner && (
            <button className="btn btn-danger" onClick={() => setShowDelete(true)}>
              Remove app
            </button>
          )}
        </div>
      </div>

      {error && <div className="error-banner">{error}</div>}

      <div className="card">
        <div className="card-title">Status</div>
        <p style={{ margin: 0 }}>
          {deployment?.status === "running" && "This app is up and running."}
          {deployment?.status === "failed" && "This app is having trouble starting. Try again below, or switch to the advanced view for more detail."}
          {(deployment?.status === "pending" || deployment?.status === "building") && "This app is getting set up. This can take a minute."}
          {deployment?.status === "stopped" && "This app isn't running right now."}
        </p>
      </div>

      <button type="button" className="btn btn-sm" onClick={() => setMode("technical")}>
        Switch to advanced view
      </button>

      {showDelete && (
        <ConfirmModal
          title="Remove app"
          body={`This removes "${deployment?.name ?? "this app"}" and everything it stored. This can't be undone.`}
          confirmLabel="Remove app"
          onClose={() => setShowDelete(false)}
          onConfirm={async () => {
            await api.del(`/api/deployments/${deploymentId}`);
            navigate("/");
          }}
        />
      )}
    </>
  );
}

function errMsg(e: unknown): string {
  return e instanceof ApiError ? e.message : "Something went wrong";
}
