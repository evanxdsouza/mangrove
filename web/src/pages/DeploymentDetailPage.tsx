import { useEffect, useState } from "react";
import { api, ApiError, type Deployment, type DeployHistory, type HealthCheckEntry, type Service } from "../api";
import { Link } from "../router";
import { StatusPill } from "../components/StatusPill";
import { DeployTimeline } from "../components/DeployTimeline";
import { LogViewer } from "../components/LogViewer";
import { EnvVarsEditor } from "../components/EnvVarsEditor";
import { ConfirmModal } from "../components/ConfirmModal";
import { RunCommandCard } from "../components/RunCommandCard";
import { useIsOwner } from "../userContext";

type Tab = "overview" | "history" | "logs" | "env";

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
        {/* A static site has no container to stream logs from -- Caddy serves the built files directly. */}
        {(["overview", "history", "logs", "env"] as Tab[])
          .filter((t) => t !== "logs" || deployment?.build_strategy !== "static")
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
          <AutoDeployCard projectId={projectId} deploymentId={deploymentId} deployment={deployment} onSaved={load} />
          <BuildConfigCard deploymentId={deploymentId} deployment={deployment} onSaved={load} />
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
    </div>
  );
}

const BUILD_STRATEGIES = ["dockerfile", "nixpacks", "compose", "image", "static"] as const;

// BuildConfigCard is the only edit path for a deployment's build_strategy
// and related fields (dockerfile/compose path, static build command +
// output dir, image ref) after creation -- the "Deploy from GitHub" wizard
// only sets these once, at creation time, from its own best-guess
// auto-detect. A common miss: a Vite/CRA frontend (which always has a
// package.json for its build tooling, even though it ships no server)
// gets auto-detected as a generic nixpacks app, and every build then fails
// with "no start command could be found" since there was never going to be
// one. This card is how that gets corrected without deleting and
// recreating the deployment. git_branch is deliberately not editable here
// -- that's AutoDeployCard's field -- so saving this card can't clobber it.
function BuildConfigCard({
  deploymentId,
  deployment,
  onSaved,
}: {
  deploymentId: number;
  deployment: Deployment | null;
  onSaved: () => void;
}) {
  const isOwner = useIsOwner();
  const [strategy, setStrategy] = useState<Deployment["build_strategy"]>("nixpacks");
  const [rootPath, setRootPath] = useState(".");
  const [dockerfilePath, setDockerfilePath] = useState("");
  const [composePath, setComposePath] = useState("");
  const [imageRef, setImageRef] = useState("");
  const [staticBuildCommand, setStaticBuildCommand] = useState("");
  const [staticOutputDir, setStaticOutputDir] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    if (deployment) {
      setStrategy(deployment.build_strategy);
      setRootPath(deployment.root_path || ".");
      setDockerfilePath(deployment.dockerfile_path ?? "");
      setComposePath(deployment.compose_path ?? "");
      setImageRef(deployment.image_ref ?? "");
      setStaticBuildCommand(deployment.static_build_command ?? "");
      setStaticOutputDir(deployment.static_output_dir ?? "");
    }
  }, [deployment]);

  const save = async () => {
    setBusy(true);
    setError(null);
    setSaved(false);
    try {
      await api.post(`/api/deployments/${deploymentId}/build-config`, {
        build_strategy: strategy,
        git_branch: deployment?.git_branch ?? "",
        root_path: rootPath,
        dockerfile_path: dockerfilePath,
        compose_path: composePath,
        image_ref: imageRef,
        static_build_command: staticBuildCommand,
        static_output_dir: staticOutputDir,
      });
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
      <div className="card-title">Build configuration</div>
      <p className="text-dim" style={{ marginTop: 0 }}>
        Only takes effect on the next deploy or redeploy -- changing this never touches what's currently running.
      </p>
      {error && <div className="error-banner">{error}</div>}
      {!isOwner && <div className="field-hint">Only an owner can change build configuration.</div>}
      <div className="form-row">
        <div className="field">
          <label htmlFor="build-strategy">Strategy</label>
          <select
            id="build-strategy"
            className="input"
            disabled={!isOwner}
            value={strategy}
            onChange={(e) => setStrategy(e.target.value as Deployment["build_strategy"])}
          >
            {BUILD_STRATEGIES.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
        </div>
        <div className="field">
          <label htmlFor="build-root-path">Root path</label>
          <input
            id="build-root-path"
            className="input mono"
            disabled={!isOwner}
            value={rootPath}
            onChange={(e) => setRootPath(e.target.value)}
          />
        </div>
      </div>
      {strategy === "dockerfile" && (
        <div className="field">
          <label htmlFor="build-dockerfile-path">Dockerfile path</label>
          <input
            id="build-dockerfile-path"
            className="input mono"
            disabled={!isOwner}
            value={dockerfilePath}
            onChange={(e) => setDockerfilePath(e.target.value)}
            placeholder="Dockerfile"
          />
        </div>
      )}
      {strategy === "compose" && (
        <div className="field">
          <label htmlFor="build-compose-path">Compose file path</label>
          <input
            id="build-compose-path"
            className="input mono"
            disabled={!isOwner}
            value={composePath}
            onChange={(e) => setComposePath(e.target.value)}
            placeholder="docker-compose.yml"
          />
        </div>
      )}
      {strategy === "image" && (
        <div className="field">
          <label htmlFor="build-image-ref">Image ref</label>
          <input
            id="build-image-ref"
            className="input mono"
            disabled={!isOwner}
            value={imageRef}
            onChange={(e) => setImageRef(e.target.value)}
            placeholder="registry.example.com/app:latest"
          />
        </div>
      )}
      {strategy === "static" && (
        <div className="form-row">
          <div className="field">
            <label htmlFor="build-static-command">Build command</label>
            <input
              id="build-static-command"
              className="input mono"
              disabled={!isOwner}
              value={staticBuildCommand}
              onChange={(e) => setStaticBuildCommand(e.target.value)}
              placeholder="npm run build"
            />
          </div>
          <div className="field">
            <label htmlFor="build-static-output">Output directory</label>
            <input
              id="build-static-output"
              className="input mono"
              disabled={!isOwner}
              value={staticOutputDir}
              onChange={(e) => setStaticOutputDir(e.target.value)}
              placeholder="dist"
            />
          </div>
        </div>
      )}
      {isOwner && (
        <button className="btn btn-sm" onClick={save} disabled={busy}>
          {busy ? "Saving..." : "Save"}
        </button>
      )}
      {saved && <div className="field-hint">Saved. Deploy or redeploy to build with the new configuration.</div>}
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
