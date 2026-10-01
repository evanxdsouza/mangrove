import { useEffect, useState, type FormEvent } from "react";
import { api, ApiError, type Deployment, type Project } from "../api";
import { Link, useRouter } from "../router";
import { Modal, useModalClose } from "../components/Modal";
import { ProjectCardSkeleton, ProjectRowSkeleton } from "../components/Skeleton";
import { useWorkspaces } from "../workspaceContext";
import { StatusPill, worstStatus } from "../components/StatusPill";
import { PlantGlyph } from "../components/PlantGlyph";
import { ArrowRightIcon, ChevronDownIcon, EmptyLedgerIcon, GridIcon, ListViewIcon, PlusIcon, SearchIcon } from "../icons";
import { imageIconKey, TemplateIcon } from "../templateIcons";
import { fmtWhen } from "../lib/format";
import { EMPTY_PROJECTS, EMPTY_PROJECTS_WORKSPACE } from "../lib/copy";
import { firstTime } from "../lib/milestones";
import { useToast } from "../components/Toast";
import { CreateDeploymentModal } from "./ProjectDetailPage";

interface ProjectStatus {
  worst: string | null;
  count: number;
  runningCount: number;
}

// "Needs attention" is the same worst-status classification StatusPill
// already uses (failed/unhealthy/error) -- a filter chip, not a new
// status vocabulary.
const ATTENTION_STATUSES = new Set(["failed", "unhealthy", "error"]);

type ProjectView = "grid" | "list";
const VIEW_STORAGE_KEY = "mangrove-projects-view";

function readStoredView(): ProjectView {
  if (typeof window === "undefined") return "grid";
  return window.localStorage.getItem(VIEW_STORAGE_KEY) === "list" ? "list" : "grid";
}

export function ProjectsPage() {
  const { navigate } = useRouter();
  const { showToast } = useToast();
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
  const [statusFilter, setStatusFilter] = useState<"all" | "running" | "attention">("all");
  const [view, setView] = useState<ProjectView>(readStoredView);
  const [error, setError] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  // Quick-add a deployment straight from the list, without navigating into
  // the project first -- matches the reference dashboard's inline "+New"
  // on a project row.
  const [quickAddProjectId, setQuickAddProjectId] = useState<number | null>(null);

  const setPersistedView = (next: ProjectView) => {
    setView(next);
    try {
      window.localStorage.setItem(VIEW_STORAGE_KEY, next);
    } catch {
      // localStorage unavailable (private browsing etc.) -- view still
      // works for this session, just doesn't persist across reloads.
    }
  };

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
  const filteredProjects = (projects ?? []).filter((p) => {
    if (q && !p.name.toLowerCase().includes(q) && !p.slug.toLowerCase().includes(q)) return false;
    const worst = status[p.id]?.worst ?? null;
    if (statusFilter === "running") return worst === "running";
    if (statusFilter === "attention") return worst != null && ATTENTION_STATUSES.has(worst);
    return true;
  });
  const attentionCount = (projects ?? []).filter((p) => {
    const worst = status[p.id]?.worst;
    return worst != null && ATTENTION_STATUSES.has(worst);
  }).length;
  // A thin, plain-text summary strip -- counts only, never colored (the
  // same "static/structural content is never colored" rule the rest of
  // the color-rule audit already enforces), built from the per-project
  // rollups this page already computes per load, no new fetch.
  const totalDeployments = Object.values(status).reduce((sum, s) => sum + s.count, 0);
  const totalRunning = Object.values(status).reduce((sum, s) => sum + s.runningCount, 0);

  // Shared between the grid-card and list-row renderings: expanding reveals
  // a project's own deployments as a recessed inset (not a second bordered
  // card -- see DESIGN.md's "no card-in-a-card" rule), each still a real
  // link to its own detail page.
  const renderChildren = (p: Project, isExpanded: boolean) => {
    const children = deploymentsByProject[p.id] ?? [];
    if (!isExpanded || children.length === 0) return null;
    return (
      <div className="project-card-children" onClick={(e) => e.stopPropagation()}>
        {children.map((d) => {
          const iconKey = d.build_strategy === "image" ? imageIconKey(d.image_ref) : null;
          return (
            <Link key={d.id} to={`/projects/${p.id}/deployments/${d.id}`} className="project-child-tile">
              <div className="project-child-tile-top">
                <PlantGlyph status={d.status} size={16} />
                {/* A recognizable tech icon sits beside PlantGlyph, not
                    stacked on it -- the plant glyph stays the one
                    load-bearing status signal (DESIGN.md's Living-Resource
                    Rule); this is identity, not status, and an overlaid
                    corner badge at this size read as visual noise rather
                    than a legible mark. */}
                {iconKey && <TemplateIcon templateKey={iconKey} category="" className="project-child-tile-tech-icon" />}
                <span className="project-child-tile-name">{d.name}</span>
              </div>
              <div className="project-child-tile-meta">
                <StatusPill status={d.status} />
                <span className="mono text-dim">{d.build_strategy}</span>
              </div>
              <div className="text-dim project-child-tile-time">
                {d.last_deployed_at ? fmtWhen(d.last_deployed_at) : "never deployed"}
              </div>
            </Link>
          );
        })}
      </div>
    );
  };

  const renderGridCard = (p: Project) => {
    const s = status[p.id];
    const isExpanded = expanded.has(p.id);
    return (
      <div
        key={p.id}
        className={`card card-clickable project-card ${isExpanded ? "project-list-row-expanded" : ""}`}
        onClick={() => navigate(`/projects/${p.id}`)}
      >
        <div className="project-card-top">
          <PlantGlyph status={s?.worst} size={20} />
          <div className="project-card-heading">
            <Link to={`/projects/${p.id}`} className="project-card-name" onClick={(e) => e.stopPropagation()}>
              {p.name}
            </Link>
            <span className="mono text-dim project-card-slug">{p.slug}</span>
          </div>
          {s == null ? (
            <span className="text-faint mono" style={{ fontSize: 12 }}>...</span>
          ) : s.worst != null ? (
            <StatusPill status={s.worst} />
          ) : null}
          <button
            type="button"
            className="project-card-quick-add"
            aria-label={`New deployment in ${p.name}`}
            title="New deployment"
            onClick={(e) => {
              e.preventDefault();
              e.stopPropagation();
              setQuickAddProjectId(p.id);
            }}
          >
            <PlusIcon />
          </button>
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
            <span className="text-dim project-card-created">{fmtWhen(p.created_at)}</span>
          </div>
        </div>

        {renderChildren(p, isExpanded)}
      </div>
    );
  };

  const renderListRow = (p: Project) => {
    const s = status[p.id];
    const isExpanded = expanded.has(p.id);
    return (
      <div
        key={p.id}
        className={`card card-clickable project-list-row ${isExpanded ? "project-list-row-expanded" : ""}`}
        onClick={() => navigate(`/projects/${p.id}`)}
      >
        <div className="project-list-row-main">
          {s != null && s.count > 0 ? (
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
          ) : (
            <span className="project-list-row-chevron-spacer" />
          )}
          <PlantGlyph status={s?.worst} size={20} />
          <div className="project-list-row-heading">
            <span className="flex gap-8" style={{ alignItems: "baseline" }}>
              <Link to={`/projects/${p.id}`} className="project-card-name" onClick={(e) => e.stopPropagation()}>
                {p.name}
              </Link>
              <span className="mono text-dim project-card-slug">{p.slug}</span>
            </span>
            <span className="text-dim project-list-row-rollup">
              {s == null
                ? "loading..."
                : s.count === 0
                  ? "no deployments yet"
                  : `${s.runningCount} of ${s.count} deployment${s.count === 1 ? "" : "s"} running`}
            </span>
          </div>
          <div className="project-list-row-end">
            {p.workspace_name && (
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
            )}
            <span className="text-dim project-card-created">{fmtWhen(p.created_at)}</span>
            {s?.worst != null && <StatusPill status={s.worst} />}
            <button
              type="button"
              className="project-card-quick-add"
              aria-label={`New deployment in ${p.name}`}
              title="New deployment"
              onClick={(e) => {
                e.stopPropagation();
                setQuickAddProjectId(p.id);
              }}
            >
              <PlusIcon />
            </button>
            <button
              type="button"
              className="project-list-row-open"
              aria-label={`Open ${p.name}`}
              onClick={(e) => {
                e.stopPropagation();
                navigate(`/projects/${p.id}`);
              }}
            >
              <ArrowRightIcon />
            </button>
          </div>
        </div>
        {renderChildren(p, isExpanded)}
      </div>
    );
  };

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

      {projects !== null && projects.length > 0 && (
        <p className="projects-summary-strip">
          {totalRunning} of {totalDeployments} deployment{totalDeployments === 1 ? "" : "s"} running
          {attentionCount > 0 && <> &middot; {attentionCount} need{attentionCount === 1 ? "s" : ""} attention</>}
          {activeWorkspaceId == null && workspaces.length > 1 && <> &middot; across {workspaces.length} workspaces</>}
        </p>
      )}

      {error && <div className="error-banner">{error}</div>}

      {projects !== null && projects.length > 0 && (
        <div className="projects-toolbar">
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
          <div className="filter-chips" role="group" aria-label="Filter by status">
            <button type="button" className={`filter-chip ${statusFilter === "all" ? "active" : ""}`} onClick={() => setStatusFilter("all")}>
              All
            </button>
            <button
              type="button"
              className={`filter-chip ${statusFilter === "running" ? "active" : ""}`}
              onClick={() => setStatusFilter("running")}
            >
              Running
            </button>
            <button
              type="button"
              className={`filter-chip ${statusFilter === "attention" ? "active" : ""}`}
              onClick={() => setStatusFilter("attention")}
            >
              Needs attention
              {attentionCount > 0 && <span className="tab-count">{attentionCount}</span>}
            </button>
          </div>
          <div className="view-toggle" role="group" aria-label="Layout">
            <button
              type="button"
              className={`view-toggle-btn ${view === "grid" ? "active" : ""}`}
              aria-pressed={view === "grid"}
              aria-label="Grid view"
              onClick={() => setPersistedView("grid")}
            >
              <GridIcon />
            </button>
            <button
              type="button"
              className={`view-toggle-btn ${view === "list" ? "active" : ""}`}
              aria-pressed={view === "list"}
              aria-label="List view"
              onClick={() => setPersistedView("list")}
            >
              <ListViewIcon />
            </button>
          </div>
        </div>
      )}

      <div className="projects-layout">
        <div className="projects-main">
          {projects === null ? (
            view === "grid" ? (
              <div className="project-grid">
                {Array.from({ length: 4 }, (_, i) => <ProjectCardSkeleton key={i} />)}
              </div>
            ) : (
              <div className="project-list">
                {Array.from({ length: 4 }, (_, i) => <ProjectRowSkeleton key={i} />)}
              </div>
            )
          ) : projects.length === 0 ? (
            <div className="card empty-state">
              <EmptyLedgerIcon />
              <p>{activeWorkspaceId != null ? EMPTY_PROJECTS_WORKSPACE : EMPTY_PROJECTS}</p>
              <div className="field-hint">Create one to deploy your first app.</div>
            </div>
          ) : filteredProjects.length === 0 ? (
            <div className="card empty-state">
              <EmptyLedgerIcon />
              <p>
                {q && statusFilter !== "all"
                  ? <>No {statusFilter === "running" ? "running" : "attention-needing"} projects match &ldquo;{search}&rdquo;.</>
                  : q
                    ? <>No projects match &ldquo;{search}&rdquo;.</>
                    : <>No {statusFilter === "running" ? "running" : "attention-needing"} projects right now.</>}
              </p>
            </div>
          ) : view === "grid" ? (
            <div className="project-grid">{filteredProjects.map(renderGridCard)}</div>
          ) : (
            <div className="project-list">{filteredProjects.map(renderListRow)}</div>
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
            // The redirect into a brand-new, empty project used to be
            // completely silent -- the critique's "Jordan" persona flagged
            // this as the one real milestone (a first-timer's first
            // project) that passed with zero acknowledgment. Scoped to a
            // genuinely empty list at creation time, not "first time this
            // browser has seen a project" (an existing instance's second
            // user creating their own first project still gets the toast).
            const isFirstEver = (projects?.length ?? 0) === 0;
            setShowCreate(false);
            reloadWorkspaces();
            navigate(`/projects/${id}`);
            if (isFirstEver && firstTime("first-project")) {
              showToast("First project planted. Deploy something into it whenever you're ready.");
            }
          }}
        />
      )}

      {quickAddProjectId != null && (
        <CreateDeploymentModal
          projectId={quickAddProjectId}
          onClose={() => setQuickAddProjectId(null)}
          onCreated={(id) => {
            window.location.href = `/projects/${quickAddProjectId}/deployments/${id}`;
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
                <span className="text-dim activity-item-meta">
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
