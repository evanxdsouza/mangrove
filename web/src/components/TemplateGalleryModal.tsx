import { useEffect, useState, type FormEvent } from "react";
import { api, ApiError, type TemplateInstallResult, type TemplateSummary } from "../api";
import { Modal } from "./Modal";
import { Link } from "../router";
import { slugify } from "../pages/ProjectsPage";

export function TemplateGalleryModal({
  projectId,
  onClose,
  onInstalled,
}: {
  projectId: number;
  onClose: () => void;
  onInstalled: () => void;
}) {
  const [templates, setTemplates] = useState<TemplateSummary[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<TemplateSummary | null>(null);
  const [result, setResult] = useState<TemplateInstallResult | null>(null);

  useEffect(() => {
    api
      .get<TemplateSummary[]>("/api/templates")
      .then(setTemplates)
      .catch((e) => setError(errMsg(e)));
  }, []);

  if (result) {
    return <TemplateInstalledView result={result} projectId={projectId} onDone={onInstalled} />;
  }

  if (selected) {
    return (
      <InstallTemplateForm
        projectId={projectId}
        template={selected}
        onBack={() => setSelected(null)}
        onClose={onClose}
        onInstalled={setResult}
      />
    );
  }

  return (
    <Modal title="Deploy from template" onClose={onClose}>
      {error && <div className="error-banner">{error}</div>}
      {templates === null ? (
        <div className="center-loading">
          <div className="spinner" />
        </div>
      ) : (
        <div className="grid grid-2">
          {templates.map((t) => (
            <div
              key={t.key}
              className="card card-clickable"
              style={{ marginBottom: 0 }}
              onClick={() => setSelected(t)}
            >
              <div className="card-title" style={{ margin: 0 }}>
                {t.name}
              </div>
              <p className="text-dim" style={{ fontSize: 13, margin: "6px 0" }}>
                {t.description}
              </p>
              <div className="text-faint" style={{ fontSize: 12 }}>
                {t.category} &middot; ~{t.total_memory_mb}MB
                {t.deployments.length > 1 ? ` across ${t.deployments.length} deployments` : ""}
              </div>
            </div>
          ))}
        </div>
      )}
    </Modal>
  );
}

function InstallTemplateForm({
  projectId,
  template,
  onBack,
  onClose,
  onInstalled,
}: {
  projectId: number;
  template: TemplateSummary;
  onBack: () => void;
  onClose: () => void;
  onInstalled: (result: TemplateInstallResult) => void;
}) {
  const [slug, setSlug] = useState(slugify(template.name));
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const result = await api.post<TemplateInstallResult>(`/api/projects/${projectId}/templates/${template.key}/install`, {
        slug,
      });
      onInstalled(result);
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal title={`Deploy ${template.name}`} onClose={onClose}>
      {error && <div className="error-banner">{error}</div>}
      <p className="text-dim" style={{ marginTop: 0 }}>
        {template.description}
      </p>
      <form onSubmit={submit}>
        <div className="field">
          <label htmlFor="template-slug">Slug</label>
          <input id="template-slug" className="input mono" required value={slug} onChange={(e) => setSlug(slugify(e.target.value))} />
          <div className="field-hint">
            {template.deployments.length > 1
              ? `Used as a base for ${template.deployments.length} linked deployments (e.g. "${slug}" and "${slug}${template.deployments[0].slug_suffix}").`
              : "The deployment's slug within this project."}
          </div>
        </div>

        <div className="card-title" style={{ marginTop: 18 }}>
          What this creates
        </div>
        <div className="kv-list">
          {template.deployments.map((d) => (
            <div className="kv-row" key={d.slug_suffix}>
              <span className="kv-key">
                {slug}
                {d.slug_suffix}
              </span>
              <span className="kv-value">
                {d.image_ref} &middot; {d.cpu_limit_cores} CPU / {d.memory_limit_mb}MB
                {d.force_internal_only ? " · internal only" : ""}
              </span>
            </div>
          ))}
        </div>
        <div className="field-hint" style={{ marginTop: 10 }}>
          Every deployment goes through the same memory/port admission control as a hand-created one -- this needs ~
          {template.total_memory_mb}MB of budget in total. Check Admin &rarr; Resource budget if you're unsure how much is free.
        </div>

        <div className="modal-actions">
          <button type="button" className="btn" onClick={onBack} disabled={busy}>
            Back
          </button>
          <button type="submit" className="btn btn-primary" disabled={busy}>
            {busy ? "Deploying..." : "Deploy"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function TemplateInstalledView({
  result,
  projectId,
  onDone,
}: {
  result: TemplateInstallResult;
  projectId: number;
  onDone: () => void;
}) {
  const credentials = Object.entries(result.credentials ?? {});
  return (
    <Modal title="Deployed" onClose={onDone}>
      <div className="kv-list">
        {result.deployments.map((d) => (
          <div className="kv-row" key={d.deployment_id}>
            <span className="kv-key">{d.name}</span>
            <span className="kv-value">
              <Link to={`/projects/${projectId}/deployments/${d.deployment_id}`}>{d.slug}</Link>
              {d.deploy_error ? <span style={{ color: "var(--red)" }}> -- failed: {d.deploy_error}</span> : null}
            </span>
          </div>
        ))}
      </div>

      {credentials.length > 0 && (
        <>
          <div className="card-title" style={{ marginTop: 18 }}>
            Generated credentials
          </div>
          <p className="field-hint" style={{ marginTop: 0 }}>
            Shown only once -- copy these now. They're stored encrypted and can't be displayed again.
          </p>
          {credentials.map(([label, value]) => (
            <div className="field" key={label}>
              <label>{label}</label>
              <input className="input mono" readOnly value={value} onFocus={(e) => e.target.select()} />
            </div>
          ))}
        </>
      )}

      <div className="modal-actions">
        <button className="btn btn-primary" onClick={onDone}>
          Done
        </button>
      </div>
    </Modal>
  );
}

function errMsg(e: unknown): string {
  return e instanceof ApiError ? e.message : "Something went wrong";
}
