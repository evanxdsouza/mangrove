import { useEffect, useState } from "react";
import { api, ApiError, type Deployment, type DeployHistory, type HealthCheckEntry, type Service, type WebhookEvent } from "../api";
import { Link } from "../router";
import { StatusPill } from "../components/StatusPill";
import { DeployTimeline } from "../components/DeployTimeline";
import { LogViewer } from "../components/LogViewer";
import { ServiceTerminal } from "../components/Terminal";
import { EnvVarsEditor } from "../components/EnvVarsEditor";
import { ConfirmModal } from "../components/ConfirmModal";
import { RunCommandCard } from "../components/RunCommandCard";
import { DomainsPanel } from "../components/DomainsPanel";
import { useIsOwner } from "../userContext";

type Tab = "overview" | "history" | "logs" | "terminal" | "env";

export function DeploymentDetailPage({ projectId, deploymentId }: { projectId: number; deploymentId: number }) {
  const isOwner = useIsOwner();
  const [deployment, setDeployment] = useState<Deployment | null>(null);
  const [services, setServices] = useState<Service[]>([]);
  const [history, setHistory] = useState<DeployHistory[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [tab, setTab] = useState<Tab>("overview");
  const [deploying, setDeploying] = useState(false);
  const [canceling, setCanceling] = useState(false);
  const [redeploying, setRedeploying] = useState(false);
  const [stopping, setStopping] = useState(false);
  const [restarting, setRestarting] = useState(false);
  const [rollbackBusyId, setRollbackBusyId] = useState<number | null>(null);
  const [selectedServiceId, setSelectedServiceId] = useState<number | null>(null);
  const [showDelete, setShowDelete] = useState(false);

  const load = () => {
    api.get<Deployment>(`/api/deployments/${deploymentId}`).then(setDeployment).catch((e) => setError(errMsg(e)));
    api
      .get<Service[]>(`/api/deployments/${deploymentId}/services`)
      .then((s) => {
        setServices(s ?? []);
        setSelectedServiceId((prev) => prev ?? (s && s.length > 0 ? s[0].id : null));
      })
      .catch((e) => setError(errMsg(e)));
    api
      .get<DeployHistory[]>(`/api/deployments/${deploymentId}/history`)
      .then((h) => setHistory(h ?? []))
      .catch((e) => setError(errMsg(e)));
  };

  useEffect(load, [deploymentId]);

  const deploy = async () => {
    setDeploying(true);
    setError(null);
    try {
      await api.post(`/api/deployments/${deploymentId}/deploy`, {});
      load();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setDeploying(false);
    }
  };

  const cancel = async () => {
    setCanceling(true);
    setError(null);
    try {
      await api.post(`/api/deployments/${deploymentId}/cancel`, {});
      load();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setCanceling(false);
    }
  };

  const redeploy = async () => {
    setRedeploying(true);
    setError(null);
    try {
      await api.post(`/api/deployments/${deploymentId}/redeploy`, {});
      load();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setRedeploying(false);
    }
  };

  const stop = async () => {
    setStopping(true);
    setError(null);
    try {
      await api.post(`/api/deployments/${deploymentId}/stop`, {});
      load();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setStopping(false);
    }
  };

  const restart = async () => {
    setRestarting(true);
    setError(null);
    try {
      await api.post(`/api/deployments/${deploymentId}/restart`, {});
      load();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setRestarting(false);
    }
  };

  const rollback = async (historyId: number) => {
    setRollbackBusyId(historyId);
    setError(null);
    try {
      await api.post(`/api/deploy-history/${historyId}/rollback`, {});
      load();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setRollbackBusyId(null);
    }
  };

  return (
    <>
      <div className="breadcrumb">
        <Link to="/">Projects</Link> / <Link to={`/projects/${projectId}`}>Project</Link> / {deployment?.name ?? "..."}
      </div>
      <div className="page-header">
        <div>
          <h1>{deployment?.name ?? "Loading..."}</h1>
          {deployment && (
            <p className="flex gap-8" style={{ alignItems: "center" }}>
              <StatusPill status={deployment.status} />
              <span className="mono text-dim">{deployment.build_strategy}</span>
              {deployment.environment === "staging" && <span className="pill pill-yellow">staging</span>}
              {deployment.environment === "preview" && (
                <span className="pill pill-yellow">preview{deployment.pr_number ? ` (PR #${deployment.pr_number})` : ""}</span>
              )}
            </p>
          )}
        </div>
        <div className="flex gap-8">
          {deployment?.status === "building" || deployment?.status === "pending" ? (
            <button className="btn btn-danger" onClick={cancel} disabled={canceling}>
              {canceling ? "Cancelling..." : "Cancel deploy"}
            </button>
          ) : (
            <button className="btn btn-primary" onClick={deploy} disabled={deploying}>
              {deploying ? "Deploying..." : "Deploy"}
            </button>
          )}
          <button
            className="btn"
            onClick={redeploy}
            disabled={redeploying}
            title="Rebuild and redeploy from the currently configured source (linked repo branch, or image ref)"
          >
            {redeploying ? "Redeploying..." : "Redeploy"}
          </button>
          {deployment?.build_strategy !== "static" && (
            <>
              {deployment?.status === "stopped" ? (
                <button className="btn" onClick={restart} disabled={restarting}>
                  {restarting ? "Starting..." : "Start"}
                </button>
              ) : (
                <>
                  <button className="btn" onClick={restart} disabled={restarting}>
                    {restarting ? "Restarting..." : "Restart"}
                  </button>
                  <button className="btn" onClick={stop} disabled={stopping}>
                    {stopping ? "Stopping..." : "Stop"}
                  </button>
                </>
              )}
            </>
          )}
          {isOwner && (
            <button className="btn btn-danger" onClick={() => setShowDelete(true)}>
              Delete deployment
            </button>
          )}
        </div>
      </div>

      {error && <div className="error-banner">{error}</div>}

      <div className="tabs">
        {/* A static site has no container to stream logs from, or shell into --
            Caddy serves the built files directly. */}
        {(["overview", "history", "logs", "terminal", "env"] as Tab[])
          .filter((t) => (t !== "logs" && t !== "terminal") || deployment?.build_strategy !== "static")
          .map((t) => (
          <div key={t} className={`tab ${tab === t ? "active" : ""}`} onClick={() => setTab(t)}>
            {t[0].toUpperCase() + t.slice(1)}
          </div>
        ))}
      </div>

      {tab === "overview" && (
        <>
          <OverviewTab services={services} deployment={deployment} />
          <AccessControlCard deploymentId={deploymentId} deployment={deployment} onSaved={load} />
          <DomainsPanel deploymentId={deploymentId} />
          <AutoDeployCard projectId={projectId} deploymentId={deploymentId} deployment={deployment} onSaved={load} />
          {deployment?.promotes_to_deployment_id != null ? (
            <PromoteCard projectId={projectId} deploymentId={deploymentId} deployment={deployment} />
          ) : (
            deployment && (
              <>
                <StagingCard projectId={projectId} productionDeployment={deployment} />
                <PreviewsCard projectId={projectId} productionDeployment={deployment} />
              </>
            )
          )}
        </>
      )}
      {tab === "history" && (
        <div className="card">
          <DeployTimeline history={history} onRollback={rollback} busyId={rollbackBusyId} />
        </div>
      )}

      {tab === "logs" && (
        <div className="card">
          {services.length === 0 ? (
            <div className="empty-state">No services yet.</div>
          ) : (
            <>
              {services.length > 1 && (
                <div className="field">
                  <label htmlFor="log-service-select">Service</label>
                  <select
                    id="log-service-select"
                    className="input"
                    value={selectedServiceId ?? undefined}
                    onChange={(e) => setSelectedServiceId(Number(e.target.value))}
                  >
                    {services.map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.name}
                      </option>
                    ))}
                  </select>
                </div>
              )}
              {selectedServiceId && <LogViewer serviceId={selectedServiceId} />}
            </>
          )}
        </div>
      )}

      {tab === "terminal" && (
        <div className="card">
          {services.length === 0 ? (
            <div className="empty-state">No services yet.</div>
          ) : (
            <>
              {services.length > 1 && (
                <div className="field">
                  <label htmlFor="terminal-service-select">Service</label>
                  <select
                    id="terminal-service-select"
                    className="input"
                    value={selectedServiceId ?? undefined}
                    onChange={(e) => setSelectedServiceId(Number(e.target.value))}
                  >
                    {services.map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.name}
                      </option>
                    ))}
                  </select>
                </div>
              )}
              {selectedServiceId && <ServiceTerminal key={selectedServiceId} serviceId={selectedServiceId} />}
            </>
          )}
        </div>
      )}

      {tab === "env" && (
        <div className="card">
          {services.length === 0 ? (
            <div className="empty-state">No services yet.</div>
          ) : (
            services.map((s) => (
              <div key={s.id} style={{ marginBottom: 20 }}>
                {services.length > 1 && <div className="card-title">{s.name}</div>}
                <EnvVarsEditor serviceId={s.id} />
              </div>
            ))
          )}
        </div>
      )}

      {showDelete && (
        <ConfirmModal
          title="Delete deployment"
          body={`This permanently deletes "${deployment?.name ?? "this deployment"}" -- stopping its container(s), removing volumes, and releasing its port. This cannot be undone.`}
          confirmLabel="Delete deployment"
          onClose={() => setShowDelete(false)}
          onConfirm={async () => {
            await api.del(`/api/deployments/${deploymentId}`);
            window.location.href = `/projects/${projectId}`;
          }}
        />
      )}
    </>
  );
}

interface ProjectRepoInfo {
  id: number;
  repo_owner: string;
  repo_name: string;
  webhook_registered: boolean;
}

function AccessControlCard({
  deploymentId,
  deployment,
  onSaved,
}: {
  deploymentId: number;
  deployment: Deployment | null;
  onSaved: () => void;
}) {
  const isOwner = useIsOwner();
  const [isPublic, setIsPublic] = useState(false);
  const [passwordProtected, setPasswordProtected] = useState(false);
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    if (deployment) {
      setIsPublic(deployment.is_public);
      setPasswordProtected(deployment.password_protected);
    }
  }, [deployment]);

  const save = async () => {
    setBusy(true);
    setError(null);
    setSaved(false);
    try {
      await api.post(`/api/deployments/${deploymentId}/access`, {
        is_public: isPublic,
        password_protected: passwordProtected,
        password,
      });
      setPassword("");
      setSaved(true);
      onSaved();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="card">
      <div className="card-title">Access control</div>
      <p className="text-dim" style={{ marginTop: 0 }}>
        Enforced at the Caddy proxy layer, independent of any auth the app itself has.
      </p>
      {error && <div className="error-banner">{error}</div>}
      {!isOwner && <div className="field-hint">Only an owner can change access control.</div>}
      <div className="field">
        <label style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <input type="checkbox" checked={isPublic} disabled={!isOwner} onChange={(e) => setIsPublic(e.target.checked)} />
          Public (expose on the assigned port)
        </label>
      </div>
      {isPublic && (
        <>
          <div className="field">
            <label style={{ display: "flex", alignItems: "center", gap: 8 }}>
              <input type="checkbox" checked={passwordProtected} disabled={!isOwner} onChange={(e) => setPasswordProtected(e.target.checked)} />
              Password-protected
            </label>
          </div>
          {passwordProtected && (
            <div className="field">
              <label htmlFor="access-password">Password</label>
              <input
                id="access-password"
                className="input"
                type="password"
                disabled={!isOwner}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder={deployment?.password_protected ? "Enter a new password to change it" : ""}
              />
            </div>
          )}
        </>
      )}
      {isOwner && (
        <button className="btn btn-sm" onClick={save} disabled={busy}>
          {busy ? "Saving..." : "Save"}
        </button>
      )}
      {saved && <div className="field-hint">Saved.</div>}
    </div>
  );
}

function AutoDeployCard({
  projectId,
  deploymentId,
  deployment,
  onSaved,
}: {
  projectId: number;
  deploymentId: number;
  deployment: Deployment | null;
  onSaved: () => void;
}) {
  const [repo, setRepo] = useState<ProjectRepoInfo | null>(null);
  const [branch, setBranch] = useState("");
  const [autoDeploy, setAutoDeploy] = useState(false);
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    api
      .get<ProjectRepoInfo>(`/api/projects/${projectId}/repo`)
      .then(setRepo)
      .catch(() => setRepo(null));
  }, [projectId]);

  useEffect(() => {
    if (deployment) {
      setBranch(deployment.git_branch ?? "main");
      setAutoDeploy(deployment.auto_deploy_on_push);
    }
  }, [deployment]);

  if (!repo) {
    return null; // no repo connected to this project -- nothing to configure
  }

  const save = async () => {
    setBusy(true);
    setSaved(false);
    try {
      await api.post(`/api/deployments/${deploymentId}/repo`, {
        project_repo_id: repo.id,
        branch,
        auto_deploy_on_push: autoDeploy,
      });
      setSaved(true);
      onSaved();
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="card">
      <div className="card-title">Auto-deploy</div>
      <p className="text-dim" style={{ marginTop: 0 }}>
        Deploy automatically when {repo.repo_owner}/{repo.repo_name} receives a push to the branch below.
      </p>
      <div className="form-row" style={{ alignItems: "flex-end" }}>
        <div className="field" style={{ marginBottom: 0 }}>
          <label htmlFor="autodeploy-branch">Branch</label>
          <input id="autodeploy-branch" className="input mono" value={branch} onChange={(e) => setBranch(e.target.value)} />
        </div>
        <div className="field" style={{ marginBottom: 0, flex: "0 0 auto" }}>
          <label style={{ display: "flex", alignItems: "center", gap: 6, whiteSpace: "nowrap" }}>
            <input type="checkbox" checked={autoDeploy} onChange={(e) => setAutoDeploy(e.target.checked)} />
            Auto-deploy on push
          </label>
        </div>
        <button className="btn btn-sm" onClick={save} disabled={busy}>
          {busy ? "Saving..." : "Save"}
        </button>
      </div>
      {saved && <div className="field-hint">Saved.</div>}
      <WebhookHealth projectId={projectId} repo={repo} />
    </div>
  );
}

// WebhookHealth surfaces what used to be invisible: whether GitHub's
// webhook is actually registered, and whether recent deliveries got
// through. Previously the only feedback a user had was "my push didn't
// deploy" with no way to tell whether GitHub never reached Mangrove
// (unregistered/misconfigured webhook), reached it but failed signature
// verification (stale secret), or reached it and simply matched no
// auto-deploying deployment.
function WebhookHealth({ projectId, repo }: { projectId: number; repo: ProjectRepoInfo }) {
  const [events, setEvents] = useState<WebhookEvent[] | null>(null);
  const [resyncing, setResyncing] = useState(false);
  const [resyncError, setResyncError] = useState<string | null>(null);
  const [showInstructions, setShowInstructions] = useState(false);
  const [instructions, setInstructions] = useState<{ webhook_path: string; webhook_secret: string } | null>(null);

  const loadEvents = () => {
    api
      .get<WebhookEvent[]>(`/api/projects/${projectId}/repo/webhook-events`)
      .then((e) => setEvents(e ?? []))
      .catch(() => setEvents([]));
  };

  useEffect(loadEvents, [projectId]);

  const resync = async () => {
    setResyncing(true);
    setResyncError(null);
    try {
      const result = await api.post<{ webhook_registered: boolean; error?: string }>(
        `/api/projects/${projectId}/repo/resync-webhook`,
        {},
      );
      if (!result.webhook_registered) {
        setResyncError(result.error || "Couldn't register the webhook automatically -- use the manual setup instructions below.");
      }
    } catch (e) {
      setResyncError(errMsg(e));
    } finally {
      setResyncing(false);
    }
  };

  const revealInstructions = async () => {
    try {
      const result = await api.get<{ webhook_path: string; webhook_secret: string }>(
        `/api/projects/${projectId}/repo/webhook-instructions`,
      );
      setInstructions(result);
      setShowInstructions(true);
    } catch (e) {
      setResyncError(errMsg(e));
    }
  };

  return (
    <div style={{ marginTop: 16, paddingTop: 16, borderTop: "1px solid var(--border, #2a2f3a)" }}>
      <div className="flex-between" style={{ marginBottom: 8 }}>
        <span className="text-dim" style={{ fontSize: 13 }}>
          GitHub webhook:{" "}
          {repo.webhook_registered ? (
            <span className="pill pill-green" style={{ marginLeft: 4 }}>
              registered
            </span>
          ) : (
            <span className="pill pill-yellow" style={{ marginLeft: 4 }}>
              not confirmed
            </span>
          )}
        </span>
        <div className="flex gap-8">
          <button className="btn btn-sm" onClick={revealInstructions}>
            Setup instructions
          </button>
          <button className="btn btn-sm" onClick={resync} disabled={resyncing}>
            {resyncing ? "Resyncing..." : "Resync webhook"}
          </button>
        </div>
      </div>
      {resyncError && <div className="error-banner">{resyncError}</div>}

      {showInstructions && instructions && (
        <div className="kv-list" style={{ marginBottom: 8 }}>
          <div className="kv-row">
            <span className="kv-key">Webhook URL</span>
            <span className="kv-value mono">{window.location.origin + instructions.webhook_path}</span>
          </div>
          <div className="kv-row">
            <span className="kv-key">Secret</span>
            <span className="kv-value mono">{instructions.webhook_secret}</span>
          </div>
          <div className="field-hint">
            Paste these into {repo.repo_owner}/{repo.repo_name} &rarr; Settings &rarr; Webhooks (content type: application/json,
            events: pushes, and pull requests if you want PR previews).
          </div>
        </div>
      )}

      {events && events.length > 0 && (
        <div className="kv-list">
          <div className="text-faint" style={{ fontSize: 12, marginBottom: 4 }}>
            Recent deliveries
          </div>
          {events.slice(0, 8).map((e) => (
            <div className="kv-row" key={e.id}>
              <span className="kv-key mono" style={{ fontSize: 12 }}>
                {new Date(e.received_at).toLocaleString()}
              </span>
              <span className="kv-value text-dim" style={{ fontSize: 12 }}>
                {e.event_type} &middot; {e.signature_valid ? "signed" : "bad signature"} &middot; {e.status.replace(/_/g, " ")}
              </span>
            </div>
          ))}
        </div>
      )}
      {events && events.length === 0 && (
        <div className="field-hint">No deliveries yet -- push to {repo.repo_owner}/{repo.repo_name} to see them here.</div>
      )}
    </div>
  );
}

// PromoteCard appears on a staging deployment's own page: it deploys
// production from the exact commit currently running in staging, not
// production's branch tip -- see promoteDeployment in internal/api.
function PromoteCard({
  projectId,
  deploymentId,
  deployment,
}: {
  projectId: number;
  deploymentId: number;
  deployment: Deployment;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);

  const promote = async () => {
    setBusy(true);
    setError(null);
    setDone(false);
    try {
      await api.post(`/api/deployments/${deploymentId}/promote`, {});
      setDone(true);
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="card">
      <div className="card-title">Promote to production</div>
      <p className="text-dim" style={{ marginTop: 0 }}>
        Deploys production with the exact commit currently running here -- not just production's branch tip -- so what
        you've verified in staging is exactly what ships.
      </p>
      {error && <div className="error-banner">{error}</div>}
      {done && !error && <div className="field-hint">Promotion started -- check the production deployment's history.</div>}
      <div className="modal-actions" style={{ justifyContent: "flex-start" }}>
        <Link to={`/projects/${projectId}/deployments/${deployment.promotes_to_deployment_id}`}>
          <button type="button" className="btn">
            View production deployment
          </button>
        </Link>
        <button type="button" className="btn btn-primary" onClick={promote} disabled={busy}>
          {busy ? "Promoting..." : "Promote to production"}
        </button>
      </div>
    </div>
  );
}

// StagingCard appears on a production deployment's page (one with a linked
// repo, and not itself a staging deployment): it lists any staging
// deployments already created from this one, and lets a user spin up a new
// one tracking an arbitrary branch. See createStagingDeployment.
function StagingCard({ projectId, productionDeployment }: { projectId: number; productionDeployment: Deployment }) {
  const [repo, setRepo] = useState<{ id: number } | null>(null);
  const [staging, setStaging] = useState<Deployment[] | null>(null);
  const [branch, setBranch] = useState("staging");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadStaging = () => {
    api
      .get<Deployment[]>(`/api/deployments/${productionDeployment.id}/staging`)
      .then((list) => setStaging(list ?? []))
      .catch(() => setStaging([]));
  };

  useEffect(() => {
    api
      .get<{ id: number }>(`/api/projects/${projectId}/repo`)
      .then(setRepo)
      .catch(() => setRepo(null));
    loadStaging();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectId, productionDeployment.id]);

  if (!repo) {
    return null; // no linked repo -- nothing to branch a staging environment from
  }

  const create = async () => {
    setBusy(true);
    setError(null);
    try {
      await api.post(`/api/deployments/${productionDeployment.id}/staging`, { branch });
      loadStaging();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="card">
      <div className="card-title">Staging environments</div>
      <p className="text-dim" style={{ marginTop: 0 }}>
        A staging environment tracks its own branch, auto-deploys independently, and can be promoted into this
        deployment once verified.
      </p>
      {error && <div className="error-banner">{error}</div>}

      {staging && staging.length > 0 && (
        <div className="kv-list" style={{ marginBottom: 14 }}>
          {staging.map((s) => (
            <div className="kv-row row-link" key={s.id} onClick={() => (window.location.href = `/projects/${projectId}/deployments/${s.id}`)} style={{ cursor: "pointer" }}>
              <span className="kv-key">
                {s.name} <span className="text-faint mono">({s.git_branch})</span>
              </span>
              <span className="kv-value">
                <StatusPill status={s.status} />
              </span>
            </div>
          ))}
        </div>
      )}

      <div className="form-row" style={{ alignItems: "flex-end" }}>
        <div className="field" style={{ marginBottom: 0 }}>
          <label htmlFor="staging-branch">Branch to stage</label>
          <input id="staging-branch" className="input mono" value={branch} onChange={(e) => setBranch(e.target.value)} />
        </div>
        <button className="btn btn-sm btn-primary" onClick={create} disabled={busy || !branch}>
          {busy ? "Creating..." : "+ New staging environment"}
        </button>
      </div>
    </div>
  );
}

// PreviewsCard appears on a production deployment's page (same
// prerequisites as StagingCard): a toggle to opt into per-PR preview
// deployments, and the list of currently open ones. Unlike staging, a
// preview deployment isn't created here -- it's created and torn down
// automatically by the pull_request webhook (see handlePullRequestEvent),
// which also posts/updates a bot comment with each preview's URL directly
// on the PR.
function PreviewsCard({ projectId, productionDeployment }: { projectId: number; productionDeployment: Deployment }) {
  const [repo, setRepo] = useState<{ id: number } | null>(null);
  const [previews, setPreviews] = useState<Deployment[] | null>(null);
  const [enabled, setEnabled] = useState(productionDeployment.pr_previews_enabled);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadPreviews = () => {
    api
      .get<Deployment[]>(`/api/deployments/${productionDeployment.id}/previews`)
      .then((list) => setPreviews(list ?? []))
      .catch(() => setPreviews([]));
  };

  useEffect(() => {
    api
      .get<{ id: number }>(`/api/projects/${projectId}/repo`)
      .then(setRepo)
      .catch(() => setRepo(null));
    loadPreviews();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectId, productionDeployment.id]);

  useEffect(() => {
    setEnabled(productionDeployment.pr_previews_enabled);
  }, [productionDeployment.pr_previews_enabled]);

  if (!repo) {
    return null; // no linked repo -- nothing to preview pull requests from
  }

  const toggle = async () => {
    const next = !enabled;
    setBusy(true);
    setError(null);
    try {
      await api.post(`/api/deployments/${productionDeployment.id}/pr-previews`, { enabled: next });
      setEnabled(next);
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="card">
      <div className="card-title">PR previews</div>
      <p className="text-dim" style={{ marginTop: 0 }}>
        When enabled, every open pull request against this repo gets its own preview deployment, deployed on push and
        torn down when the PR closes -- with a bot comment on the PR linking to it.
      </p>
      {error && <div className="error-banner">{error}</div>}

      {previews && previews.length > 0 && (
        <div className="kv-list" style={{ marginBottom: 14 }}>
          {previews.map((p) => (
            <div
              className="kv-row row-link"
              key={p.id}
              onClick={() => (window.location.href = `/projects/${projectId}/deployments/${p.id}`)}
              style={{ cursor: "pointer" }}
            >
              <span className="kv-key">
                PR #{p.pr_number} <span className="text-faint mono">({p.git_branch})</span>
              </span>
              <span className="kv-value">
                <StatusPill status={p.status} />
              </span>
            </div>
          ))}
        </div>
      )}

      <label className="flex gap-8" style={{ alignItems: "center", cursor: "pointer" }}>
        <input type="checkbox" checked={enabled} onChange={toggle} disabled={busy} />
        Enable PR previews for this repo
      </label>
    </div>
  );
}

function OverviewTab({ services, deployment }: { services: Service[]; deployment: Deployment | null }) {
  const isStatic = deployment?.build_strategy === "static";
  if (services.length === 0) {
    return <div className="card empty-state">No services yet.</div>;
  }
  return (
    <>
      <div className="grid grid-2">
        {services.map((s) => (
          <ServiceCard key={s.id} service={s} isStatic={isStatic} />
        ))}
      </div>
      {deployment && <ScaleCard deploymentId={deployment.id} deployment={deployment} />}
      {!isStatic &&
        services.map((s) => (
          <RunCommandCard key={s.id} serviceId={s.id} serviceName={s.name} showServiceName={services.length > 1} />
        ))}
    </>
  );
}

function ScaleCard({ deploymentId, deployment }: { deploymentId: number; deployment: Deployment }) {
  if (deployment.build_strategy === "compose" || deployment.build_strategy === "static") {
    return null;
  }
  const [value, setValue] = useState(deployment.replicas);
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const apply = async () => {
    setBusy(true);
    setSaved(false);
    setError(null);
    try {
      await api.post(`/api/deployments/${deploymentId}/scale`, { replicas: value });
      setSaved(true);
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="card">
      <div className="card-title">Replicas</div>
      <p className="text-dim" style={{ marginTop: 0 }}>
        Number of containers of this image running behind one load-balanced route. Changing this triggers a redeploy.
      </p>
      {error && <div className="error-banner">{error}</div>}
      <div className="form-row" style={{ alignItems: "flex-end" }}>
        <div className="field" style={{ marginBottom: 0 }}>
          <label htmlFor="scale-replicas">Replicas</label>
          <input
            id="scale-replicas"
            className="input"
            type="number"
            min={1}
            max={32}
            value={value}
            onChange={(e) => setValue(Math.max(1, Number(e.target.value) || 1))}
          />
        </div>
        <button className="btn btn-sm" onClick={apply} disabled={busy || value === deployment.replicas}>
          {busy ? "Scaling..." : "Scale"}
        </button>
      </div>
      {saved && <div className="field-hint">Scaling. Watch the deployment status while it redeploys.</div>}
    </div>
  );
}

function ServiceCard({ service, isStatic }: { service: Service; isStatic: boolean }) {
  const [health, setHealth] = useState<HealthCheckEntry[] | null>(null);

  useEffect(() => {
    if (isStatic) return; // no container, no health checks to fetch
    api
      .get<HealthCheckEntry[]>(`/api/services/${service.id}/health?limit=1`)
      .then((h) => setHealth(h ?? []))
      .catch(() => setHealth([]));
  }, [service.id, isStatic]);

  const latestHealth = health && health.length > 0 ? health[0] : null;

  return (
    <div className="card">
      <div className="flex-between" style={{ marginBottom: 12 }}>
        <div className="card-title" style={{ margin: 0 }}>
          {service.name}
        </div>
        <StatusPill status={service.status} />
      </div>
      <div className="kv-list">
        {isStatic ? (
          <div className="kv-row">
            <span className="kv-key">Served by</span>
            <span className="kv-value">Caddy (file_server) -- no container runs for a static site</span>
          </div>
        ) : (
          <>
            <div className="kv-row">
              <span className="kv-key">Image</span>
              <span className="kv-value">{service.image_tag_current ?? "—"}</span>
            </div>
            <div className="kv-row">
              <span className="kv-key">Internal port</span>
              <span className="kv-value">{service.internal_port || "—"}</span>
            </div>
            <div className="kv-row">
              <span className="kv-key">Resources</span>
              <span className="kv-value">
                {service.cpu_limit_cores} CPU / {service.memory_limit_mb}MB
              </span>
            </div>
            <div className="kv-row">
              <span className="kv-key">Replicas</span>
              <span className="kv-value">{(service.replica_container_ids?.length ?? 1) || 1}</span>
            </div>
          </>
        )}
        <div className="kv-row">
          <span className="kv-key">Host port</span>
          <span className="kv-value">{service.host_port ?? (service.is_internal_only ? "internal only" : "not yet assigned")}</span>
        </div>
        {!isStatic && (
          <div className="kv-row">
            <span className="kv-key">Health</span>
            <span className="kv-value">
              {latestHealth ? (
                <>
                  <StatusPill status={latestHealth.status} /> {latestHealth.response_time_ms}ms
                </>
              ) : (
                <span className="text-faint">no checks yet</span>
              )}
            </span>
          </div>
        )}
      </div>
    </div>
  );
}

function errMsg(e: unknown): string {
  return e instanceof ApiError ? e.message : "Something went wrong";
}
