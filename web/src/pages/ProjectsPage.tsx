import { useEffect, useState, type FormEvent } from "react";
import { api, ApiError, type Deployment, type Project } from "../api";
import { Link, useRouter } from "../router";
import { Modal, useModalClose } from "../components/Modal";
import { CenterLoading } from "../components/CenterLoading";
import { useWorkspaces } from "../workspaceContext";
import { StatusPill, worstStatus } from "../components/StatusPill";
import { PlantGlyph } from "../components/PlantGlyph";
import { ChevronDownIcon, EmptyLedgerIcon, PlusIcon, SearchIcon } from "../icons";
import { fmtWhen } from "../lib/format";
import { EMPTY_PROJECTS, EMPTY_PROJECTS_WORKSPACE } from "../lib/copy";

interface ProjectStatus {
  worst: string | null;
  count: number;
  runningCount: number;
}

export function ProjectsPage() {
  const { navigate } = useRouter();
  const { workspaces, activeWorkspaceId, setActiveWorkspaceId, reload: reloadWorkspaces } = useWorkspaces();
  const [projects, setProjects] = useState<Project[] | null>(null);
  const [status, setStatus] = useState<Record<number, ProjectStatus>>({});
  // Kept alongside `status`'s rollup counts, not instead of them -- an
  // expanded card renders these directly, and the recent-activity panel
  // flattens them across every loaded project, both for free (no extra
  // request beyond the one this page already made per project).
  const [deploymentsByProject, setDeploymentsByProject] = useState<Record<number, Deployment[]>>({});
  const [expanded, setExpanded] = useState<Set<number>>(new Set());
  const [search, setSearch] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);

  const load = () => {
    const q = activeWorkspaceId ? `?workspace_id=${activeWorkspaceId}` : "";
    api
      .get<Project[]>(`/api/projects${q}`)
      .then((p) => {
        const list = p ?? [];
        setProjects(list);
        setError(null);
        // One request per project, same pattern SimpleAppsPage already uses
        // to flatten every deployment -- the ledger's status beacon is a
        // real read of what's actually deployed, not decoration.
        Promise.all(
          list.map((project) =>
            api
              .get<Deployment[]>(`/api/projects/${project.id}/deployments`)
              .then((deps): [number, Deployment[]] => [project.id, deps ?? []])
              .catch((): [number, Deployment[]] => [project.id, []]),
          ),
        ).then((entries) => {
          setDeploymentsByProject(Object.fromEntries(entries));
          setStatus(
            Object.fromEntries(
              entries.map(([id, deps]): [number, ProjectStatus] => [
                id,
                {
                  worst: worstStatus(deps.map((d) => d.status)),
                  count: deps.length,
                  runningCount: deps.filter((d) => d.status === "running").length,
                },
              ]),
            ),
          );
        });
      })
      .catch((e) => setError(e instanceof ApiError ? e.message : "Failed to load projects"));
  };

  const toggleExpanded = (projectId: number) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(projectId)) next.delete(projectId);
      else next.add(projectId);
      return next;
    });
  };

  useEffect(() => {
    load();
    const interval = setInterval(load, 4000);
    return () => clearInterval(interval);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeWorkspaceId]);

  const activeWorkspaceName = workspaces.find((w) => w.workspace.id === activeWorkspaceId)?.workspace.name ?? null;
  const q = search.trim().toLowerCase();
  const filteredProjects = (projects ?? []).filter(
    (p) => !q || p.name.toLowerCase().includes(q) || p.slug.toLowerCase().includes(q),
  );

  return (
    <>
      <div className="page-header">
        <div>
          <h1>{activeWorkspaceName ? `${activeWorkspaceName} projects` : "Projects"}</h1>
          <p>
            {activeWorkspaceName
              ? "Deployments grouped by app or site, scoped to this workspace."
              : "Group deployments by app or site, across every workspace."}
          </p>
        </div>
        <div className="flex gap-8">
          {activeWorkspaceId != null && (
            <button className="btn" onClick={() => setActiveWorkspaceId(null)}>
              All workspaces
            </button>
          )}
          <button className="btn btn-primary" onClick={() => setShowCreate(true)}>
            <PlusIcon /> New project
          </button>
        </div>
      </div>

      {error && <div className="error-banner">{error}</div>}

      {projects !== null && projects.length > 0 && (
        <div className="search-field">
          <SearchIcon />
          <input
            className="input"
            placeholder="Search projects..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            aria-label="Search projects"
          />
        </div>
      )}

      <div className="projects-layout">
        <div className="projects-main">
          {projects === null ? (
            <CenterLoading />
          ) : projects.length === 0 ? (
            <div className="card empty-state">
              <EmptyLedgerIcon />
              <p>{activeWorkspaceId != null ? EMPTY_PROJECTS_WORKSPACE : EMPTY_PROJECTS}</p>
              <div className="field-hint">Create one to deploy your first app.</div>
            </div>
          ) : filteredProjects.length === 0 ? (
            <div className="card empty-state">
              <EmptyLedgerIcon />
              <p>No projects match &ldquo;{search}&rdquo;.</p>
            </div>
          ) : (
            <div className="project-grid">
              {filteredProjects.map((p) => {
                const s = status[p.id];
                const isExpanded = expanded.has(p.id);
                const children = deploymentsByProject[p.id] ?? [];
                return (
                  <div key={p.id} className="card card-clickable project-card" onClick={() => navigate(`/projects/${p.id}`)}>
                    <div className="project-card-top">
                      <PlantGlyph status={s?.worst} size={20} />
                      <div className="project-card-heading">
                        <Link to={`/projects/${p.id}`} className="project-card-name" onClick={(e) => e.stopPropagation()}>
                          {p.name}
                        </Link>
                        <span className="mono text-faint project-card-slug">{p.slug}</span>
                      </div>
                      {s == null ? (
                        <span className="text-faint mono" style={{ fontSize: 12 }}>...</span>
                      ) : s.worst != null ? (
                        <StatusPill status={s.worst} />
                      ) : null}
                      {s != null && s.count > 0 && (
                        <button
                          type="button"
                          className="project-card-chevron-btn"
                          aria-expanded={isExpanded}
                          aria-label={isExpanded ? "Collapse deployments" : "Expand deployments"}
                          onClick={(e) => {
                            e.preventDefault();
                            e.stopPropagation();
                            toggleExpanded(p.id);
                          }}
                        >
                          <ChevronDownIcon className="project-card-chevron" />
                        </button>
                      )}
                    </div>

                    {p.description && <p className="project-card-description">{p.description}</p>}

                    <div className="project-card-footer">
                      <span className="project-card-rollup">
                        {s == null
                          ? "loading..."
                          : s.count === 0
                            ? "no deployments yet"
                            : `${s.runningCount} of ${s.count} deployment${s.count === 1 ? "" : "s"} running`}
                      </span>
                      <div className="project-card-footer-meta">
                        {p.workspace_name ? (
                          <a
                            href="/"
                            className="project-card-workspace"
                            onClick={(e) => {
                              e.preventDefault();
                              e.stopPropagation();
                              setActiveWorkspaceId(p.workspace_id);
                            }}
                          >
                            {p.workspace_name}
                          </a>
                        ) : (
                          <span />
                        )}
                        <span className="text-faint project-card-created">{fmtWhen(p.created_at)}</span>
                      </div>
                    </div>

                    {/* Expanding reveals this project's own deployments right
                        here, as a recessed inset (not a second bordered card
                        -- see DESIGN.md's "no card-in-a-card" rule), each
                        still a real link to its own detail page. */}
                    {isExpanded && children.length > 0 && (
                      <div className="project-card-children" onClick={(e) => e.stopPropagation()}>
                        {children.map((d) => (
                          <Link key={d.id} to={`/projects/${p.id}/deployments/${d.id}`} className="project-child-deployment">
                            <PlantGlyph status={d.status} size={14} />
                            <span className="project-child-name">{d.name}</span>
                            <span className="mono text-faint project-child-type">{d.build_strategy}</span>
                            <StatusPill status={d.status} />
                            <span className="text-faint project-child-meta">
                              {d.last_deployed_at ? fmtWhen(d.last_deployed_at) : "never deployed"}
                            </span>
                          </Link>
                        ))}
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </div>

        {/* A persistent rail so there's always something to look at while
            browsing, even a large/empty project grid -- see DESIGN.md's
            "resource list page" pattern. Built entirely from data this page
            already fetched per project (no extra request). */}
        {projects !== null && <RecentActivityPanel projects={projects} deploymentsByProject={deploymentsByProject} />}
      </div>

      {showCreate && (
        <CreateProjectModal
          workspaces={workspaces.map((w) => w.workspace)}
          defaultWorkspaceId={activeWorkspaceId ?? undefined}
          onClose={() => setShowCreate(false)}
          onCreated={(id) => {
            setShowCreate(false);
            reloadWorkspaces();
            navigate(`/projects/${id}`);
          }}
        />
      )}
    </>
  );
}

interface RecentActivityDeployment extends Deployment {
  projectId: number;
}

// The Projects list's persistent side rail: the most recently deployed
// things across every currently-loaded project, so there's always
// something to look at while browsing -- built entirely from data this
// page already fetched (one request per project it makes anyway), never
// a separate endpoint.
function RecentActivityPanel({
  projects,
  deploymentsByProject,
}: {
  projects: Project[];
  deploymentsByProject: Record<number, Deployment[]>;
}) {
  const projectNameById = new Map(projects.map((p) => [p.id, p.name]));
  const items: RecentActivityDeployment[] = Object.entries(deploymentsByProject)
    .flatMap(([projectId, deps]) => deps.map((d) => ({ ...d, projectId: Number(projectId) })))
    .filter((d) => d.last_deployed_at)
    .sort((a, b) => new Date(b.last_deployed_at!).getTime() - new Date(a.last_deployed_at!).getTime())
    .slice(0, 8);

  return (
    <div className="card activity-panel">
      <div className="card-title">Recent activity</div>
      {items.length === 0 ? (
        <p className="text-dim" style={{ marginTop: 0, marginBottom: 0 }}>
          Nothing deployed yet -- recent deployments across every project will show up here.
        </p>
      ) : (
        <div className="activity-list">
          {items.map((d) => (
            <Link key={d.id} to={`/projects/${d.projectId}/deployments/${d.id}`} className="activity-item">
              <PlantGlyph status={d.status} size={14} />
              <span className="activity-item-body">
                <span className="activity-item-name">{d.name}</span>
                <span className="text-faint activity-item-meta">
                  {projectNameById.get(d.projectId) ?? "..."} &middot; {fmtWhen(d.last_deployed_at)}
                </span>
              </span>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}

function CreateProjectModal({
  workspaces,
  defaultWorkspaceId,
  onClose,
  onCreated,
}: {
  workspaces: { id: number; name: string }[];
  defaultWorkspaceId?: number;
  onClose: () => void;
  onCreated: (id: number) => void;
}) {
  const requestClose = useModalClose();
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [description, setDescription] = useState("");
  const [workspaceId, setWorkspaceId] = useState<number>(defaultWorkspaceId ?? 1);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const project = await api.post<Project>("/api/projects", { name, slug, description, workspace_id: workspaceId });
      onCreated(project.id);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to create project");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal title="New project" onClose={onClose}>
      {error && <div className="error-banner">{error}</div>}
      <form onSubmit={submit}>
        <div className="field">
          <label htmlFor="project-name">Name</label>
          <input
            id="project-name"
            className="input"
            required
            value={name}
            onChange={(e) => {
              setName(e.target.value);
              if (!slug) setSlug(slugify(e.target.value));
            }}
          />
        </div>
        <div className="field">
          <label htmlFor="project-slug">Slug</label>
          <input
            id="project-slug"
            className="input mono"
            required
            value={slug}
            onChange={(e) => setSlug(slugify(e.target.value))}
          />
        </div>
        <div className="field">
          <label htmlFor="project-description">Description (optional)</label>
          <input id="project-description" className="input" value={description} onChange={(e) => setDescription(e.target.value)} />
        </div>
        <div className="field">
          <label htmlFor="project-workspace">Workspace</label>
          <select id="project-workspace" className="input" value={workspaceId} onChange={(e) => setWorkspaceId(Number(e.target.value))}>
            {workspaces.map((w) => (
              <option key={w.id} value={w.id}>
                {w.name}
              </option>
            ))}
          </select>
        </div>
        <div className="modal-actions">
          <button type="button" className="btn" onClick={requestClose}>
            Cancel
          </button>
          <button type="submit" className="btn btn-primary" disabled={busy}>
            {busy ? "Creating..." : "Create project"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

export function slugify(s: string): string {
  return s
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/(^-|-$)/g, "");
}
