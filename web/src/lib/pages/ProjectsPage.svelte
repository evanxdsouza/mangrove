<script lang="ts">
  import { goto } from "$app/navigation";
  import { api, ApiError, type Deployment, type Project } from "../api";
  import { worstStatus } from "../status";
  import { fmtWhen } from "../format";
  import { EMPTY_PROJECTS, EMPTY_PROJECTS_WORKSPACE } from "../copy";
  import { firstTime } from "../milestones";
  import { imageIconKey } from "../templateIcons";
  import { showToast } from "../stores/toast.svelte";
  import { reloadWorkspaces, setActiveWorkspaceId, workspaceState } from "../stores/workspaces.svelte";
  import Icon from "../components/Icon.svelte";
  import StatusPill from "../components/StatusPill.svelte";
  import TideGauge from "../components/TideGauge.svelte";
  import TideBanner from "../components/TideBanner.svelte";
  import TemplateIcon from "../components/TemplateIcon.svelte";
  import ProjectCardSkeleton from "../components/skeleton/ProjectCardSkeleton.svelte";
  import ProjectRowSkeleton from "../components/skeleton/ProjectRowSkeleton.svelte";
  import CreateProjectModal from "../components/projects/CreateProjectModal.svelte";
  import CreateDeploymentModal from "../components/projects/CreateDeploymentModal.svelte";
  import RecentActivityPanel from "../components/projects/RecentActivityPanel.svelte";

  interface ProjectStatus {
    worst: string | null;
    count: number;
    runningCount: number;
  }

  // "Needs attention" is the same worst-status classification the pill
  // already uses (failed/unhealthy/error) -- a filter chip, not a new
  // status vocabulary.
  const ATTENTION_STATUSES = new Set(["failed", "unhealthy", "error"]);

  type ProjectView = "grid" | "list";
  const VIEW_STORAGE_KEY = "mangrove-projects-view";
  function readStoredView(): ProjectView {
    try {
      return window.localStorage.getItem(VIEW_STORAGE_KEY) === "list" ? "list" : "grid";
    } catch {
      return "grid";
    }
  }

  let projects = $state<Project[] | null>(null);
  let status = $state<Record<number, ProjectStatus>>({});
  // Kept alongside `status`'s rollup counts -- an expanded card renders
  // these directly, and the recent-activity panel flattens them, both for
  // free (no extra request beyond the one per project this page makes).
  let deploymentsByProject = $state<Record<number, Deployment[]>>({});
  let expanded = $state<Set<number>>(new Set());
  let search = $state("");
  let statusFilter = $state<"all" | "running" | "attention">("all");
  let view = $state<ProjectView>(readStoredView());
  let error = $state<string | null>(null);
  let showCreate = $state(false);
  // Quick-add a deployment straight from the list, without navigating in.
  let quickAddProjectId = $state<number | null>(null);

  function setPersistedView(next: ProjectView) {
    view = next;
    try {
      window.localStorage.setItem(VIEW_STORAGE_KEY, next);
    } catch {
      // localStorage unavailable -- view still works for this session.
    }
  }

  function load(workspaceId: number | null) {
    const q = workspaceId ? `?workspace_id=${workspaceId}` : "";
    api
      .get<Project[]>(`/api/projects${q}`)
      .then((p) => {
        const list = p ?? [];
        projects = list;
        error = null;
        // One request per project -- the status gauge is a real read of
        // what's actually deployed, not decoration.
        Promise.all(
          list.map((project) =>
            api
              .get<Deployment[]>(`/api/projects/${project.id}/deployments`)
              .then((deps): [number, Deployment[]] => [project.id, deps ?? []])
              .catch((): [number, Deployment[]] => [project.id, []]),
          ),
        ).then((entries) => {
          deploymentsByProject = Object.fromEntries(entries);
          status = Object.fromEntries(
            entries.map(([id, deps]): [number, ProjectStatus] => [
              id,
              {
                worst: worstStatus(deps.map((d) => d.status)),
                count: deps.length,
                runningCount: deps.filter((d) => d.status === "running").length,
              },
            ]),
          );
        });
      })
      .catch((e) => (error = e instanceof ApiError ? e.message : "Failed to load projects"));
  }

  $effect(() => {
    const ws = workspaceState.activeWorkspaceId;
    load(ws);
    const interval = setInterval(() => load(ws), 4000);
    return () => clearInterval(interval);
  });

  function toggleExpanded(projectId: number) {
    const next = new Set(expanded);
    if (next.has(projectId)) next.delete(projectId);
    else next.add(projectId);
    expanded = next;
  }

  const activeWorkspaceId = $derived(workspaceState.activeWorkspaceId);
  const activeWorkspaceName = $derived(
    workspaceState.workspaces.find((w) => w.workspace.id === activeWorkspaceId)?.workspace.name ?? null,
  );
  const q = $derived(search.trim().toLowerCase());
  const filteredProjects = $derived(
    (projects ?? []).filter((p) => {
      if (q && !p.name.toLowerCase().includes(q) && !p.slug.toLowerCase().includes(q)) return false;
      const worst = status[p.id]?.worst ?? null;
      if (statusFilter === "running") return worst === "running";
      if (statusFilter === "attention") return worst != null && ATTENTION_STATUSES.has(worst);
      return true;
    }),
  );
  const attentionCount = $derived(
    (projects ?? []).filter((p) => {
      const worst = status[p.id]?.worst;
      return worst != null && ATTENTION_STATUSES.has(worst);
    }).length,
  );
  // A plain-text summary strip -- counts only, built from the rollups this
  // page already computes, no new fetch.
  const totalDeployments = $derived(Object.values(status).reduce((sum, s) => sum + s.count, 0));
  const totalRunning = $derived(Object.values(status).reduce((sum, s) => sum + s.runningCount, 0));

  function lastDeployedAt(p: Project): string | null {
    const stamps = (deploymentsByProject[p.id] ?? []).map((d) => d.last_deployed_at).filter((t): t is string => !!t);
    if (stamps.length === 0) return null;
    return stamps.reduce((a, b) => (new Date(a).getTime() >= new Date(b).getTime() ? a : b));
  }

  function pickWorkspace(e: MouseEvent, id: number) {
    e.preventDefault();
    e.stopPropagation();
    setActiveWorkspaceId(id);
  }

  function quickAdd(e: MouseEvent, id: number) {
    e.preventDefault();
    e.stopPropagation();
    quickAddProjectId = id;
  }

  function toggle(e: MouseEvent, id: number) {
    e.preventDefault();
    e.stopPropagation();
    toggleExpanded(id);
  }
</script>

{#snippet rollup(s: ProjectStatus | undefined)}
  {#if s == null}loading...{:else if s.count === 0}no deployments yet{:else}<b>{s.runningCount} of {s.count}</b> deployment{s.count === 1 ? "" : "s"} running{/if}
{/snippet}

{#snippet worstPill(s: ProjectStatus | undefined)}
  {#if s != null}
    {#if s.worst != null}
      <StatusPill status={s.worst} />
    {:else}
      <span class="pill pill-gray"><span class="pill-dot"></span>no deployments</span>
    {/if}
  {/if}
{/snippet}

{#snippet workspacePill(p: Project)}
  {#if p.workspace_name}
    <a href="/" class="project-card-workspace" title="Show only {p.workspace_name}" onclick={(e) => pickWorkspace(e, p.workspace_id)}>
      {p.workspace_name}
    </a>
  {/if}
{/snippet}

<!-- An expanded project's deployments hang off the root line, each a real
     link to its own page, ending in a dashed "add a deployment" tile. -->
{#snippet childTiles(p: Project)}
  {@const kids = deploymentsByProject[p.id] ?? []}
  {#if expanded.has(p.id) && kids.length > 0}
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="project-card-children" onclick={(e) => e.stopPropagation()}>
      {#each kids as d (d.id)}
        {@const iconKey = d.build_strategy === "image" ? imageIconKey(d.image_ref) : null}
        <a href="/projects/{p.id}/deployments/{d.id}" class="project-child-tile">
          <span class="project-child-tile-icon">
            {#if iconKey}
              <TemplateIcon templateKey={iconKey} category="" class="project-child-tile-tech-icon" />
            {:else}
              <Icon name="box" />
            {/if}
          </span>
          <TideGauge status={d.status} size="sm" />
          <span class="project-child-tile-name">{d.name}</span>
          <span class="mono text-dim">{d.build_strategy}</span>
          <span class="project-child-tile-meta">
            {#if d.last_deployed_at || d.status !== "pending"}
              <StatusPill status={d.status} />
            {:else}
              <span class="project-child-tile-time">never deployed</span>
            {/if}
          </span>
        </a>
      {/each}
      <button type="button" class="project-child-add" onclick={(e) => quickAdd(e, p.id)}>+ Add a deployment</button>
    </div>
  {/if}
{/snippet}

{#snippet chevron(p: Project)}
  {@const s = status[p.id]}
  {@const isExpanded = expanded.has(p.id)}
  {#if s != null && s.count > 0}
    <button
      type="button"
      class="project-card-chevron-btn"
      aria-expanded={isExpanded}
      aria-label={isExpanded ? "Collapse deployments" : "Expand deployments"}
      onclick={(e) => toggle(e, p.id)}
    >
      <Icon name="chevron-down" class="project-card-chevron" />
    </button>
  {/if}
{/snippet}

{#snippet quickAddButton(p: Project)}
  <button type="button" class="project-card-quick-add" aria-label="New deployment in {p.name}" title="New deployment" onclick={(e) => quickAdd(e, p.id)}>
    <Icon name="plus" />
  </button>
{/snippet}

{#snippet gridCard(p: Project)}
  {@const s = status[p.id]}
  {@const last = lastDeployedAt(p)}
  <!-- The whole card navigates; its name is the real keyboard-reachable link. -->
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="card card-clickable project-card {expanded.has(p.id) ? 'project-card-wide' : ''}" onclick={() => goto(`/projects/${p.id}`)}>
    <div class="project-card-banner"><TideBanner status={s?.worst} /></div>
    <div class="project-card-body">
      <div class="project-card-top">
        <TideGauge status={s?.worst} />
        <div class="project-card-heading">
          <a href="/projects/{p.id}" class="project-card-name" onclick={(e) => e.stopPropagation()}>{p.name}</a>
          <span class="mono text-dim project-card-slug">{p.slug}</span>
        </div>
        {@render worstPill(s)}
        <span class="project-card-actions">
          {@render quickAddButton(p)}
          {@render chevron(p)}
        </span>
      </div>
      <p class="project-card-description">{p.description || "No description"}</p>
      <div class="project-card-footer">
        <span class="project-card-rollup">{@render rollup(s)}</span>
        {@render workspacePill(p)}
        <span class="project-card-created">Last deployed {last ? fmtWhen(last) : "never"}</span>
      </div>
      {@render childTiles(p)}
    </div>
  </div>
{/snippet}

{#snippet listRow(p: Project)}
  {@const s = status[p.id]}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="card card-clickable project-list-row {expanded.has(p.id) ? 'project-list-row-expanded' : ''}" onclick={() => goto(`/projects/${p.id}`)}>
    <div class="project-list-row-main">
      {#if s != null && s.count > 0}
        {@render chevron(p)}
      {:else}
        <span class="project-list-row-chevron-spacer"></span>
      {/if}
      <TideGauge status={s?.worst} size="sm" />
      <div class="project-list-row-heading">
        <a href="/projects/{p.id}" class="project-card-name" onclick={(e) => e.stopPropagation()}>{p.name}</a>
        <span class="mono text-dim project-card-slug">{p.slug}</span>
      </div>
      <span class="text-dim project-list-row-rollup">{@render rollup(s)}</span>
      <div class="project-list-row-end">
        {@render workspacePill(p)}
        <span class="text-dim project-card-created">{fmtWhen(p.created_at)}</span>
        {@render worstPill(s)}
        {@render quickAddButton(p)}
        <button
          type="button"
          class="project-list-row-open"
          aria-label="Open {p.name}"
          onclick={(e) => {
            e.stopPropagation();
            goto(`/projects/${p.id}`);
          }}
        >
          <Icon name="arrow-right" />
        </button>
      </div>
    </div>
    {@render childTiles(p)}
  </div>
{/snippet}

<div class="page-header">
  <div>
    <h1>{activeWorkspaceName ? `${activeWorkspaceName} projects` : "Projects"}</h1>
    <p>
      {activeWorkspaceName
        ? "Deployments grouped by app or site, scoped to this workspace."
        : "Group deployments by app or site, across every workspace."}
    </p>
  </div>
  <div class="flex gap-8">
    {#if activeWorkspaceId != null}
      <button class="btn" onclick={() => setActiveWorkspaceId(null)}>All workspaces</button>
    {/if}
    <button class="btn btn-primary" onclick={() => (showCreate = true)}><Icon name="plus" /> New project</button>
  </div>
</div>

{#if projects !== null && projects.length > 0}
  <p class="projects-summary-strip">
    <b>{totalRunning} of {totalDeployments}</b> deployment{totalDeployments === 1 ? "" : "s"} running
    {#if attentionCount > 0}
      &middot; <b class="attention">{attentionCount} need{attentionCount === 1 ? "s" : ""} attention</b>
    {/if}
    {#if activeWorkspaceId == null && workspaceState.workspaces.length > 1}
      &middot; across {workspaceState.workspaces.length} workspaces
    {/if}
  </p>
{/if}

{#if error}<div class="error-banner">{error}</div>{/if}

{#if projects !== null && projects.length > 0}
  <div class="projects-toolbar">
    <div class="search-field">
      <Icon name="search" />
      <input class="input" placeholder="Search projects..." bind:value={search} aria-label="Search projects" />
    </div>
    <div class="filter-chips" role="group" aria-label="Filter by status">
      <button type="button" class="filter-chip {statusFilter === 'all' ? 'active' : ''}" onclick={() => (statusFilter = "all")}>All</button>
      <button type="button" class="filter-chip {statusFilter === 'running' ? 'active' : ''}" onclick={() => (statusFilter = "running")}>Running</button>
      <button type="button" class="filter-chip {statusFilter === 'attention' ? 'active' : ''}" onclick={() => (statusFilter = "attention")}>
        Needs attention
        {#if attentionCount > 0}<span class="tab-count">{attentionCount}</span>{/if}
      </button>
    </div>
    <div class="view-toggle" role="group" aria-label="Layout">
      <button
        type="button"
        class="view-toggle-btn {view === 'grid' ? 'active' : ''}"
        aria-pressed={view === "grid"}
        aria-label="Grid view"
        onclick={() => setPersistedView("grid")}
      >
        <Icon name="grid" />
      </button>
      <button
        type="button"
        class="view-toggle-btn {view === 'list' ? 'active' : ''}"
        aria-pressed={view === "list"}
        aria-label="List view"
        onclick={() => setPersistedView("list")}
      >
        <Icon name="list-view" />
      </button>
    </div>
  </div>
{/if}

<div class="projects-layout">
  <div class="projects-main">
    {#if projects === null}
      {#if view === "grid"}
        <div class="project-grid">
          {#each { length: 4 } as _, i (i)}<ProjectCardSkeleton />{/each}
        </div>
      {:else}
        <div class="project-list">
          {#each { length: 4 } as _, i (i)}<ProjectRowSkeleton />{/each}
        </div>
      {/if}
    {:else if projects.length === 0}
      <div class="card empty-state">
        {#if activeWorkspaceId == null}
          <div class="empty-state-banner"><TideBanner level="low" /></div>
        {/if}
        <p>{activeWorkspaceId != null ? EMPTY_PROJECTS_WORKSPACE : EMPTY_PROJECTS}</p>
        <div class="field-hint">Create one to deploy your first app.</div>
        <button class="btn btn-primary" onclick={() => (showCreate = true)}><Icon name="plus" /> New project</button>
      </div>
    {:else if filteredProjects.length === 0}
      <div class="card empty-state">
        <p>
          {#if q && statusFilter !== "all"}
            No {statusFilter === "running" ? "running" : "attention-needing"} projects match &ldquo;{search}&rdquo;.
          {:else if q}
            No projects match &ldquo;{search}&rdquo;.
          {:else}
            No {statusFilter === "running" ? "running" : "attention-needing"} projects right now.
          {/if}
        </p>
      </div>
    {:else if view === "grid"}
      <div class="project-grid">
        {#each filteredProjects as p (p.id)}{@render gridCard(p)}{/each}
      </div>
    {:else}
      <div class="project-list">
        {#each filteredProjects as p (p.id)}{@render listRow(p)}{/each}
      </div>
    {/if}
  </div>

  <!-- A persistent rail so there's always something to look at while
       browsing, built entirely from data this page already fetched. -->
  {#if projects !== null}
    <RecentActivityPanel {projects} {deploymentsByProject} />
  {/if}
</div>

{#if showCreate}
  <CreateProjectModal
    workspaces={workspaceState.workspaces.map((w) => w.workspace)}
    defaultWorkspaceId={activeWorkspaceId ?? undefined}
    onClose={() => (showCreate = false)}
    onCreated={(id) => {
      // Scoped to a genuinely empty list at creation time, so a second
      // user's own first project on an existing instance still gets it.
      const isFirstEver = (projects?.length ?? 0) === 0;
      showCreate = false;
      reloadWorkspaces();
      goto(`/projects/${id}`);
      if (isFirstEver && firstTime("first-project")) {
        showToast("First project planted. Deploy something into it whenever you're ready.");
      }
    }}
  />
{/if}

{#if quickAddProjectId != null}
  {@const pid = quickAddProjectId}
  <CreateDeploymentModal
    projectId={pid}
    onClose={() => (quickAddProjectId = null)}
    onCreated={(id) => {
      quickAddProjectId = null;
      goto(`/projects/${pid}/deployments/${id}`);
    }}
  />
{/if}
