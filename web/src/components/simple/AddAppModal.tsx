import { useEffect, useState } from "react";
import { api, ApiError, type Project, type TemplateInstallResult, type TemplateSummary } from "../../api";
import { Modal, useModalClose } from "../Modal";
import { SIMPLE_TEMPLATES } from "../../pages/simple/plainCopy";

// Simple mode's "add an app" flow collapses Mangrove's two-step technical
// flow (create a project, then install a template into it) into one
// click: it creates a throwaway project behind the scenes, named after
// the app, and installs the chosen template straight into it. The
// project concept never surfaces in this mode's UI.
export function AddAppModal({ onClose, onAdded }: { onClose: () => void; onAdded: (projectId: number, deploymentId: number) => void }) {
  const requestClose = useModalClose();
  const [templates, setTemplates] = useState<TemplateSummary[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [installing, setInstalling] = useState<string | null>(null);
  const [result, setResult] = useState<{ templateKey: string; res: TemplateInstallResult; projectId: number } | null>(null);

  useEffect(() => {
    api
      .get<TemplateSummary[]>("/api/templates")
      .then((all) => setTemplates((all ?? []).filter((t) => t.key in SIMPLE_TEMPLATES)))
      .catch((e) => setError(errMsg(e)));
  }, []);

  const install = async (template: TemplateSummary) => {
    const copy = SIMPLE_TEMPLATES[template.key];
    setInstalling(template.key);
    setError(null);
    try {
      const suffix = Math.random().toString(36).slice(2, 7);
      const project = await api.post<Project>("/api/projects", {
        name: copy.label,
        slug: `${slugify(copy.label)}-${suffix}`,
        description: "",
      });
      const res = await api.post<TemplateInstallResult>(
        `/api/projects/${project.id}/templates/${template.key}/install`,
        { slug: slugify(copy.label) },
      );
      setResult({ templateKey: template.key, res, projectId: project.id });
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setInstalling(null);
    }
  };

  if (result) {
    const primary = result.res.deployments[0];
    const credentials = Object.entries(result.res.credentials ?? {});
    return (
      <Modal title="Your app is ready" onClose={onClose}>
        <p>{SIMPLE_TEMPLATES[result.templateKey].label} is set up and starting.</p>
        {primary?.deploy_error && (
          <div className="error-banner">Something went wrong getting it running: {primary.deploy_error}</div>
        )}
        {credentials.length > 0 && (
          <>
            <p className="text-dim" style={{ fontSize: 13 }}>
              Save this now -- you'll need it to log in, and it won't be shown again.
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
          <button
            className="btn btn-primary"
            onClick={() => primary && onAdded(result.projectId, primary.deployment_id)}
          >
            Done
          </button>
        </div>
      </Modal>
    );
  }

  return (
    <Modal title="Add an app" onClose={onClose}>
      {error && <div className="error-banner">{error}</div>}
      {templates === null ? (
        <div className="center-loading">
          <div className="spinner" />
        </div>
      ) : (
        <div className="grid grid-2">
          {templates.map((t) => {
            const copy = SIMPLE_TEMPLATES[t.key];
            return (
              <div
                key={t.key}
                className="card card-clickable"
                style={{ marginBottom: 0, opacity: installing && installing !== t.key ? 0.5 : 1 }}
                onClick={() => !installing && install(t)}
              >
                <div className="card-title" style={{ margin: 0 }}>
                  {copy.label}
                </div>
                <p className="text-dim" style={{ fontSize: 13, margin: "6px 0 0" }}>
                  {installing === t.key ? "Setting it up..." : copy.blurb}
                </p>
              </div>
            );
          })}
        </div>
      )}
      <div className="modal-actions">
        <button type="button" className="btn" onClick={requestClose} disabled={!!installing}>
          Cancel
        </button>
      </div>
    </Modal>
  );
}

function slugify(s: string): string {
  return s
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/(^-|-$)/g, "");
}

function errMsg(e: unknown): string {
  return e instanceof ApiError ? e.message : "Something went wrong";
}
