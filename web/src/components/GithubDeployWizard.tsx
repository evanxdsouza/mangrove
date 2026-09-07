import { useEffect, useMemo, useState } from "react";
import {
  api,
  ApiError,
  type DetectedEnvVar,
  type DetectionResult,
  type GithubCredential,
  type GithubRepo,
  type Project,
} from "../api";
import { Modal, useModalClose } from "./Modal";
import { slugify } from "../pages/ProjectsPage";

type Strategy = "dockerfile" | "nixpacks" | "compose" | "static";
type Step = "connect" | "repo" | "configure" | "result";

// "Deploy from GitHub": connect (OAuth popup) -> pick a repo (private repos
// included, via the connected token) -> auto-detect a build strategy and
// prompt for whatever env vars the repo declares -> deploy. Every step
// after OAuth is just composing calls the technical flow already exposes
// (link repo, create deployment, set env vars, redeploy) -- see
// CreateDeploymentModal/LinkRepoModal in ProjectDetailPage.tsx and
// InstallTemplateForm in TemplateGalleryModal.tsx for the patterns this
// mirrors.
//
// projectId is omitted when opened from Simple mode or a fresh "add app"
// flow -- in that case a project is created behind the scenes, named after
// the chosen repo, the same way AddAppModal already does for templates.
export function GithubDeployWizard({
  projectId,
  onClose,
  onDeployed,
}: {
  projectId?: number;
  onClose: () => void;
  onDeployed: (projectId: number, deploymentId: number) => void;
}) {
  const requestClose = useModalClose();
  const [step, setStep] = useState<Step>("connect");
  const [creds, setCreds] = useState<GithubCredential[] | null>(null);
  const [credId, setCredId] = useState<number | "">("");
  const [error, setError] = useState<string | null>(null);

  const [repos, setRepos] = useState<GithubRepo[] | null>(null);
  const [filter, setFilter] = useState("");
  const [selectedRepo, setSelectedRepo] = useState<GithubRepo | null>(null);
  const [branch, setBranch] = useState("main");

  const loadCreds = () => {
    api
      .get<GithubCredential[]>("/api/github/pats")
      .then((list) => {
        const all = list ?? [];
        setCreds(all);
        if (all.length > 0) {
          setCredId((prev) => (prev === "" ? all[0].id : prev));
          setStep((s) => (s === "connect" ? "repo" : s));
        }
      })
      .catch((e) => setError(errMsg(e)));
  };

  useEffect(loadCreds, []);

  useEffect(() => {
    if (step !== "repo" || !credId) return;
    setRepos(null);
    api
      .get<GithubRepo[]>(`/api/github/repos?github_pat_id=${credId}`)
      .then((r) => setRepos(r ?? []))
      .catch((e) => setError(errMsg(e)));
  }, [step, credId]);

  const connect = () => {
    setError(null);
    const popup = window.open("/api/github/oauth/start", "mangrove-github-oauth", "width=680,height=780");
    if (!popup) {
      setError("Your browser blocked the popup -- allow popups for this site and try again.");
      return;
    }
    const handler = (ev: MessageEvent) => {
      if (ev.origin !== window.location.origin) return;
      const data = ev.data as { type?: string; ok?: boolean; error?: string } | undefined;
      if (!data || data.type !== "mangrove-github-oauth") return;
      window.removeEventListener("message", handler);
      if (data.ok) {
        loadCreds();
      } else {
        setError(data.error || "GitHub connection failed.");
      }
    };
    window.addEventListener("message", handler);
  };

  const filteredRepos = useMemo(() => {
    const list = repos ?? [];
    if (!filter.trim()) return list;
    const needle = filter.toLowerCase();
    return list.filter((r) => `${r.owner}/${r.name}`.toLowerCase().includes(needle));
  }, [repos, filter]);

  const pickRepo = (r: GithubRepo) => {
    setSelectedRepo(r);
    setBranch(r.default_branch || "main");
    setStep("configure");
  };

  if (step === "result") {
    return null; // ConfigureStep renders its own result view once deployed.
  }

  return (
    <Modal title="Deploy from GitHub" onClose={onClose}>
      {error && <div className="error-banner">{error}</div>}

      {step === "connect" && (
        <>
          <p className="text-dim" style={{ marginTop: 0 }}>
            Connect a GitHub account to pick a repo (including private ones) without pasting a token by hand.
          </p>
          <div className="modal-actions" style={{ justifyContent: "flex-start" }}>
            <button type="button" className="btn btn-primary" onClick={connect}>
              Connect GitHub
            </button>
          </div>
          {creds && creds.length === 0 && (
            <p className="field-hint">
              You can also use a pasted Personal Access Token from Admin -- if you add one there, come back and reopen this dialog.
            </p>
          )}
          <div className="modal-actions">
            <button type="button" className="btn" onClick={requestClose}>
              Cancel
            </button>
          </div>
        </>
      )}

      {step === "repo" && (
        <>
          {creds && creds.length > 1 && (
            <div className="field">
              <label htmlFor="gh-cred">GitHub account</label>
              <select
                id="gh-cred"
                className="input"
                value={credId}
                onChange={(e) => setCredId(Number(e.target.value))}
              >
                {creds.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.source === "oauth" ? `@${c.github_login}` : c.label}
                  </option>
                ))}
              </select>
            </div>
          )}
          <div className="field">
            <label htmlFor="gh-filter">Find a repo</label>
            <input
              id="gh-filter"
              className="input"
              placeholder="owner/name"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              autoFocus
            />
          </div>
          {repos === null ? (
            <div className="center-loading">
              <div className="spinner" />
            </div>
          ) : filteredRepos.length === 0 ? (
            <div className="card empty-state">No matching repos.</div>
          ) : (
            <div className="kv-list" style={{ maxHeight: 320, overflowY: "auto" }}>
              {filteredRepos.map((r) => (
                <div
                  key={`${r.owner}/${r.name}`}
                  className="kv-row row-link"
                  onClick={() => pickRepo(r)}
                  style={{ cursor: "pointer" }}
                >
                  <span className="kv-key mono">
                    {r.owner}/{r.name}
                    {r.private && <span className="text-faint"> (private)</span>}
                  </span>
                  <span className="kv-value text-dim" style={{ fontSize: 12 }}>
                    {r.description || ""}
                  </span>
                </div>
              ))}
            </div>
          )}
          <div className="modal-actions">
            <button type="button" className="btn" onClick={requestClose}>
              Cancel
            </button>
            <button type="button" className="btn" onClick={connect}>
              Connect another account
            </button>
          </div>
        </>
      )}

      {step === "configure" && selectedRepo && (
        <ConfigureStep
          projectId={projectId}
          credId={credId === "" ? undefined : credId}
          repo={selectedRepo}
          branch={branch}
          onBranchChange={setBranch}
          onBack={() => setStep("repo")}
          onDeployed={onDeployed}
        />
      )}
    </Modal>
  );
}

function ConfigureStep({
  projectId,
  credId,
  repo,
  branch,
  onBranchChange,
  onBack,
  onDeployed,
}: {
  projectId?: number;
  credId?: number;
  repo: GithubRepo;
  branch: string;
  onBranchChange: (b: string) => void;
  onBack: () => void;
  onDeployed: (projectId: number, deploymentId: number) => void;
}) {
  const [detecting, setDetecting] = useState(true);
  const [detectError, setDetectError] = useState<string | null>(null);
  const [strategy, setStrategy] = useState<Strategy>("dockerfile");
  const [dockerfilePath, setDockerfilePath] = useState("Dockerfile");
  const [composePath, setComposePath] = useState("docker-compose.yml");
  const [staticBuildCommand, setStaticBuildCommand] = useState("");
  const [staticOutputDir, setStaticOutputDir] = useState("dist");
  const [envVars, setEnvVars] = useState<DetectedEnvVar[]>([]);
  const [envValues, setEnvValues] = useState<Record<string, string>>({});

  const [appName, setAppName] = useState(repo.name);
  const [slug, setSlug] = useState(slugify(repo.name));
  const [serviceName, setServiceName] = useState("web");
  const [internalPort, setInternalPort] = useState(3000);
  const [internalOnly, setInternalOnly] = useState(true);
  const [memoryMB, setMemoryMB] = useState(256);
  const [cpuCores, setCpuCores] = useState(0.5);

  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<{ projectId: number; deploymentId: number; deployError?: string } | null>(null);

  useEffect(() => {
    setDetecting(true);
    setDetectError(null);
    api
      .post<DetectionResult>("/api/github/detect", {
        github_pat_id: credId,
        repo_owner: repo.owner,
        repo_name: repo.name,
        branch,
        root_path: ".",
      })
      .then((r) => {
        setStrategy(r.build_strategy as Strategy);
        if (r.dockerfile_path) setDockerfilePath(r.dockerfile_path);
        if (r.compose_path) setComposePath(r.compose_path);
        if (r.suggested_port) setInternalPort(r.suggested_port);
        if (r.static_build_command) setStaticBuildCommand(r.static_build_command);
        if (r.static_output_dir) setStaticOutputDir(r.static_output_dir);
        setEnvVars(r.env_vars ?? []);
      })
      .catch((e) => setDetectError(errMsg(e)))
      .finally(() => setDetecting(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [repo, branch]);

  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      let pid = projectId;
      if (!pid) {
        const project = await api.post<Project>("/api/projects", { name: appName, slug, description: "" });
        pid = project.id;
      }

      const repoLink = await api.post<{ id: number; webhook_path: string; webhook_secret: string; webhook_registered: boolean }>(
        `/api/projects/${pid}/repo`,
        { github_pat_id: credId, repo_owner: repo.owner, repo_name: repo.name, default_branch: branch },
      );

      const payload: Record<string, unknown> = {
        name: appName,
        slug,
        build_strategy: strategy,
        git_branch: branch,
        root_path: ".",
        dockerfile_path: dockerfilePath,
        compose_path: composePath,
        static_build_command: staticBuildCommand,
        static_output_dir: staticOutputDir,
        image_retention_count: 5,
      };
      if (strategy === "static") {
        payload.service = { name: serviceName, is_internal_only: internalOnly };
      } else if (strategy !== "compose") {
        payload.service = {
          name: serviceName,
          internal_port: internalPort,
          is_internal_only: internalOnly,
          cpu_limit_cores: cpuCores,
          memory_limit_mb: memoryMB,
          health_check_path: "/",
        };
      }
      const dep = await api.post<{ id: number }>(`/api/projects/${pid}/deployments`, payload);

      await api.post(`/api/deployments/${dep.id}/repo`, {
        project_repo_id: repoLink.id,
        branch,
        auto_deploy_on_push: true,
      });

      if (strategy !== "compose" && envVars.length > 0) {
        const services = await api.get<{ id: number }[]>(`/api/deployments/${dep.id}/services`);
        const serviceId = services?.[0]?.id;
        if (serviceId) {
          for (const ev of envVars) {
            const value = envValues[ev.key];
            if (!value) continue;
            await api.put(`/api/services/${serviceId}/env/${ev.key}`, { value, is_secret: ev.secret });
          }
        }
      }

      let deployError: string | undefined;
      try {
        await api.post(`/api/deployments/${dep.id}/redeploy`, {});
      } catch (e) {
        deployError = errMsg(e);
      }

      setResult({ projectId: pid, deploymentId: dep.id, deployError });
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  };

  if (result) {
    return (
      <>
        <p>
          <strong>
            {repo.owner}/{repo.name}
          </strong>{" "}
          is set up and deploying.
        </p>
        {result.deployError && <div className="error-banner">The first deploy failed to start: {result.deployError}</div>}
        <div className="modal-actions">
          <button className="btn btn-primary" onClick={() => onDeployed(result.projectId, result.deploymentId)}>
            View deployment
          </button>
        </div>
      </>
    );
  }

  return (
    <>
      {error && <div className="error-banner">{error}</div>}
      {detectError && (
        <div className="error-banner">Couldn't auto-detect this repo ({detectError}) -- review the settings below manually.</div>
      )}

      {!projectId && (
        <div className="form-row">
          <div className="field">
            <label htmlFor="ghw-name">Name</label>
            <input
              id="ghw-name"
              className="input"
              required
              value={appName}
              onChange={(e) => {
                setAppName(e.target.value);
                setSlug(slugify(e.target.value));
              }}
            />
          </div>
          <div className="field">
            <label htmlFor="ghw-slug">Slug</label>
            <input id="ghw-slug" className="input mono" required value={slug} onChange={(e) => setSlug(slugify(e.target.value))} />
          </div>
        </div>
      )}

      <div className="field">
        <label htmlFor="ghw-branch">Branch</label>
        <input id="ghw-branch" className="input mono" value={branch} onChange={(e) => onBranchChange(e.target.value)} />
      </div>

      <div className="field">
        <label htmlFor="ghw-strategy">Build strategy {detecting && <span className="text-faint">(detecting...)</span>}</label>
        <select id="ghw-strategy" className="input" value={strategy} onChange={(e) => setStrategy(e.target.value as Strategy)} disabled={detecting}>
          <option value="dockerfile">Dockerfile</option>
          <option value="nixpacks">Nixpacks (no Dockerfile needed)</option>
          <option value="compose">Docker Compose (multi-service)</option>
          <option value="static">Static site (build + serve files, no container)</option>
        </select>
      </div>

      {strategy === "dockerfile" && (
        <div className="field">
          <label htmlFor="ghw-dockerfile">Dockerfile path</label>
          <input id="ghw-dockerfile" className="input mono" value={dockerfilePath} onChange={(e) => setDockerfilePath(e.target.value)} />
        </div>
      )}
      {strategy === "compose" && (
        <>
          <div className="field">
            <label htmlFor="ghw-compose">Compose file path</label>
            <input id="ghw-compose" className="input mono" value={composePath} onChange={(e) => setComposePath(e.target.value)} />
          </div>
          <div className="field-hint">
            Compose stacks have per-service env vars -- set them from each service's page after the first deploy.
          </div>
        </>
      )}
      {strategy === "static" && (
        <>
          <div className="field">
            <label htmlFor="ghw-build-cmd">Build command (optional)</label>
            <input
              id="ghw-build-cmd"
              className="input mono"
              placeholder="npm run build -- leave blank if already pre-built"
              value={staticBuildCommand}
              onChange={(e) => setStaticBuildCommand(e.target.value)}
            />
          </div>
          <div className="field">
            <label htmlFor="ghw-output-dir">Output directory</label>
            <input id="ghw-output-dir" className="input mono" value={staticOutputDir} onChange={(e) => setStaticOutputDir(e.target.value)} />
          </div>
        </>
      )}

      {strategy !== "compose" && (
        <>
          <div className="card-title" style={{ marginTop: 18 }}>
            Service
          </div>
          <div className="field">
            <label htmlFor="ghw-service-name">Service name</label>
            <input id="ghw-service-name" className="input mono" value={serviceName} onChange={(e) => setServiceName(e.target.value)} />
          </div>
          {strategy !== "static" && (
            <div className="form-row">
              <div className="field">
                <label htmlFor="ghw-port">Internal port</label>
                <input id="ghw-port" className="input" type="number" value={internalPort} onChange={(e) => setInternalPort(Number(e.target.value))} />
              </div>
              <div className="field">
                <label htmlFor="ghw-memory">Memory limit (MB)</label>
                <input id="ghw-memory" className="input" type="number" value={memoryMB} onChange={(e) => setMemoryMB(Number(e.target.value))} />
              </div>
              <div className="field">
                <label htmlFor="ghw-cpu">CPU cores</label>
                <input id="ghw-cpu" className="input" type="number" step="0.1" value={cpuCores} onChange={(e) => setCpuCores(Number(e.target.value))} />
              </div>
            </div>
          )}
          <div className="field">
            <label style={{ display: "flex", alignItems: "center", gap: 8 }}>
              <input type="checkbox" checked={internalOnly} onChange={(e) => setInternalOnly(e.target.checked)} />
              Internal only (never expose a public port)
            </label>
          </div>
        </>
      )}

      {strategy !== "compose" && envVars.length > 0 && (
        <>
          <div className="card-title" style={{ marginTop: 18 }}>
            Environment variables found in this repo
          </div>
          {envVars.map((ev) => (
            <div className="field" key={ev.key}>
              <label htmlFor={`ghw-env-${ev.key}`}>
                {ev.key}
                {ev.secret && <span className="text-faint"> (secret)</span>}
              </label>
              <input
                id={`ghw-env-${ev.key}`}
                className="input mono"
                type={ev.secret ? "password" : "text"}
                value={envValues[ev.key] ?? ""}
                onChange={(e) => setEnvValues((prev) => ({ ...prev, [ev.key]: e.target.value }))}
              />
            </div>
          ))}
        </>
      )}

      <div className="modal-actions">
        <button type="button" className="btn" onClick={onBack} disabled={busy}>
          Back
        </button>
        <button type="button" className="btn btn-primary" onClick={submit} disabled={busy || detecting || !slug}>
          {busy ? "Deploying..." : "Deploy"}
        </button>
      </div>
    </>
  );
}

function errMsg(e: unknown): string {
  return e instanceof ApiError ? e.message : "Something went wrong";
}
