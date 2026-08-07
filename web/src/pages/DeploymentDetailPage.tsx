import { useEffect, useState } from "react";
import { api, ApiError, type Deployment, type DeployHistory, type HealthCheckEntry, type Service } from "../api";
import { Link } from "../router";
import { StatusPill } from "../components/StatusPill";
import { DeployTimeline } from "../components/DeployTimeline";
import { LogViewer } from "../components/LogViewer";
import { EnvVarsEditor } from "../components/EnvVarsEditor";

type Tab = "overview" | "history" | "logs" | "env";

export function DeploymentDetailPage({ projectId, deploymentId }: { projectId: number; deploymentId: number }) {
  const [deployment, setDeployment] = useState<Deployment | null>(null);
  const [services, setServices] = useState<Service[]>([]);
  const [history, setHistory] = useState<DeployHistory[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [tab, setTab] = useState<Tab>("overview");
  const [deploying, setDeploying] = useState(false);
  const [rollbackBusyId, setRollbackBusyId] = useState<number | null>(null);
  const [selectedServiceId, setSelectedServiceId] = useState<number | null>(null);

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
        <button className="btn btn-primary" onClick={deploy} disabled={deploying}>
          {deploying ? "Deploying..." : "Deploy"}
        </button>
      </div>

      {error && <div className="error-banner">{error}</div>}

      <div className="tabs">
        {(["overview", "history", "logs", "env"] as Tab[]).map((t) => (
          <div key={t} className={`tab ${tab === t ? "active" : ""}`} onClick={() => setTab(t)}>
            {t[0].toUpperCase() + t.slice(1)}
          </div>
        ))}
      </div>

      {tab === "overview" && <OverviewTab services={services} />}

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
    </>
  );
}

function OverviewTab({ services }: { services: Service[] }) {
  if (services.length === 0) {
    return <div className="card empty-state">No services yet.</div>;
  }
  return (
    <div className="grid grid-2">
      {services.map((s) => (
        <ServiceCard key={s.id} service={s} />
      ))}
    </div>
  );
}

function ServiceCard({ service }: { service: Service }) {
  const [health, setHealth] = useState<HealthCheckEntry[] | null>(null);

  useEffect(() => {
    api
      .get<HealthCheckEntry[]>(`/api/services/${service.id}/health?limit=1`)
      .then((h) => setHealth(h ?? []))
      .catch(() => setHealth([]));
  }, [service.id]);

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
        <div className="kv-row">
          <span className="kv-key">Image</span>
          <span className="kv-value">{service.image_tag_current ?? "—"}</span>
        </div>
        <div className="kv-row">
          <span className="kv-key">Internal port</span>
          <span className="kv-value">{service.internal_port || "—"}</span>
        </div>
        <div className="kv-row">
          <span className="kv-key">Host port</span>
          <span className="kv-value">{service.host_port ?? (service.is_internal_only ? "internal only" : "not yet assigned")}</span>
        </div>
        <div className="kv-row">
          <span className="kv-key">Resources</span>
          <span className="kv-value">
            {service.cpu_limit_cores} CPU / {service.memory_limit_mb}MB
          </span>
        </div>
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
      </div>
    </div>
  );
}

function errMsg(e: unknown): string {
  return e instanceof ApiError ? e.message : "Something went wrong";
}
