<script lang="ts">
  import { goto } from "$app/navigation";
  import { api, type Deployment, type Project, type Workspace } from "../api";
  import { errMsg } from "../errors";
  import { worstStatus } from "../status";
  import { fmtWhen } from "../format";
  import { EMPTY_DEPLOYMENTS } from "../copy";
  import { reloadWorkspaces, workspaceRole } from "../stores/workspaces.svelte";
  import Icon from "../components/Icon.svelte";
  import StatusPill from "../components/StatusPill.svelte";
  import TideGauge from "../components/TideGauge.svelte";
  import TideBanner from "../components/TideBanner.svelte";
  import ConfirmModal from "../components/ConfirmModal.svelte";
  import TemplateGalleryModal from "../components/TemplateGalleryModal.svelte";
  import GithubDeployWizard from "../components/GithubDeployWizard.svelte";
  import DeploymentCardSkeleton from "../components/skeleton/DeploymentCardSkeleton.svelte";
  import CreateDeploymentModal from "../components/projects/CreateDeploymentModal.svelte";
  import LinkRepoModal from "../components/projects/LinkRepoModal.svelte";

  let { projectId }: { projectId: number } = $props();

  interface ProjectRepoInfo {
    id: number;
    repo_owner: string;
    repo_name: string;
    default_branch: string;
    webhook_path: string;
  }
  type ProjectTab = "deployments" | "github" | "workspace";

  let project = $state<Project | null>(null);
  let deployments = $state<Deployment[] | null>(null);
  let repo = $state<ProjectRepoInfo | null | undefined>(undefined); // undefined = not loaded, null = none linked
  let error = $state<string | null>(null);
  let showCreate = $state(false);
  let showLinkRepo = $state(false);
  let showTemplates = $state(false);
  let showGithubWizard = $state(false);
  let showDelete = $state(false);
  let workspaces = $state<Workspace[]>([]);
  let workspaceId = $state<number>(1);
  let tab = $state<ProjectTab>("deployments");
  // Tracks each deployment's previous status so a fresh transition into
  // "running" pops that card's tide gauge -- real feedback from a real
  // state change, not decoration on a timer.
  let prevStatuses = new Map<number, string>();
  let burstKeys = $state<Map<number, number>>(new Map());

  const isAdmin = $derived(workspaceRole(project?.workspace_id) === "admin");

  function load() {
    api
      .get<Project>(`/api/projects/${projectId}`)
      .then((p) => {
        project = p;
        error = null;
      })
      .catch((e) => (error = errMsg(e)));
    api
      .get<Deployment[]>(`/api/projects/${projectId}/deployments`)
      .then((d) => {
        const list = d ?? [];
        const justStarted = list.filter((dep) => dep.status === "running" && prevStatuses.get(dep.id) != null && prevStatuses.get(dep.id) !== "running");
        if (justStarted.length > 0) {
          const next = new Map(burstKeys);
          for (const dep of justStarted) next.set(dep.id, Date.now());
          burstKeys = next;
        }
        prevStatuses = new Map(list.map((dep) => [dep.id, dep.status]));
        deployments = list;
        error = null;
      })
      .catch((e) => (error = errMsg(e)));
    api
      .get<ProjectRepoInfo>(`/api/projects/${projectId}/repo`)
      .then((r) => (repo = r))
      .catch(() => (repo = null));
  }

  $effect(() => {
    void projectId;
    load();
    const interval = setInterval(load, 4000);
    return () => clearInterval(interval);
  });

  $effect(() => {
    const id = projectId;
    api
      .get<{ workspace?: Workspace }[]>("/api/workspaces")
      .then((ws) => {
        workspaces = (ws ?? []).map((x) => (x.workspace ?? x) as Workspace);
        api.get<Project>(`/api/projects/${id}`).then((p) => (workspaceId = p.workspace_id)).catch(() => {});
      })
      .catch(() => {});
  });

  async function moveWorkspace() {
    error = null;
    try {
      await api.post(`/api/projects/${projectId}/workspace`, { workspace_id: workspaceId });
      load();
      reloadWorkspaces();
    } catch (e) {
      error = errMsg(e);
    }
  }

  // The header banner follows the project's worst deployment status, the
  // same tide level its card on the Projects page shows.
  const heroStatus = $derived(deployments && deployments.length > 0 ? worstStatus(deployments.map((d) => d.status)) : null);

  // Grouped by environment (real, tracked data) only once there's more than
  // one to separate, so a single-environment project stays one flat grid.
  const ENV_ORDER: Record<string, number> = { production: 0, staging: 1, preview: 2 };
  const ENV_LABEL: Record<string, string> = { production: "Production", staging: "Staging", preview: "Previews" };
  const environments = $derived(
    Array.from(new Set((deployments ?? []).map((d) => d.environment))).sort((a, b) => (ENV_ORDER[a] ?? 9) - (ENV_ORDER[b] ?? 9)),
  );
</script>

{#snippet grid(list: Deployment[])}
  <div class="deployment-grid">
    {#each list as d (d.id)}
      <a href="/projects/{projectId}/deployments/{d.id}" class="card card-clickable deployment-card">
        <div class="deployment-card-top">
          <span class="deployment-card-icon"><Icon name="box" /></span>
          <TideGauge status={d.status} size="sm" burstKey={burstKeys.get(d.id)} />
          <span class="deployment-card-heading">
            <span class="deployment-card-name">{d.name}</span>
            <span class="mono">{d.build_strategy}</span>
          </span>
          <StatusPill status={d.status} />
        </div>
        <div class="deployment-card-meta">
          {#if d.environment === "staging"}<span class="pill pill-yellow">staging</span>{/if}
          {#if d.environment === "preview"}
            <span class="pill pill-gray">preview{d.pr_number ? ` · PR #${d.pr_number}` : ""}</span>
          {/if}
          {#if d.is_public && d.password_protected}
            <span><Icon name="lock" /> Password-protected</span>
          {:else if d.is_public}
            <span><Icon name="globe" /> Public</span>
          {:else}
            <span>Internal only</span>
          {/if}
          <span>Last deployed {d.last_deployed_at ? fmtWhen(d.last_deployed_at) : "never"}</span>
        </div>
      </a>
    {/each}
  </div>
{/snippet}

<nav class="breadcrumb" aria-label="Breadcrumb">
  <a href="/" class="breadcrumb-back"><Icon name="chevron-down" /> Projects</a>
  <span>/</span>
  <span>{project?.name ?? "..."}</span>
</nav>

<section class="project-hero">
  <TideBanner status={heroStatus} />
  <div class="project-hero-text">
    <h1>{project?.name ?? "Loading..."}</h1>
    {#if project?.description}<p>{project.description}</p>{/if}
  </div>
</section>

<div class="project-actions">
  <button class="btn" onclick={() => (showTemplates = true)}><Icon name="layers" /> Deploy from template</button>
  <button class="btn" onclick={() => (showGithubWizard = true)}><Icon name="branch" /> Deploy from GitHub</button>
  <button class="btn btn-primary" onclick={() => (showCreate = true)}><Icon name="plus" /> New deployment</button>
  <span class="spacer"></span>
  {#if isAdmin}
    <button class="btn btn-danger" onclick={() => (showDelete = true)}><Icon name="trash" /> Delete project</button>
  {/if}
</div>

{#if error}<div class="error-banner">{error}</div>{/if}

<div class="tabs" role="tablist" aria-label="Project sections">
  <button type="button" role="tab" aria-selected={tab === "deployments"} class="tab {tab === 'deployments' ? 'active' : ''}" onclick={() => (tab = "deployments")}>
    Deployments{#if deployments != null}<span class="tab-count">{deployments.length}</span>{/if}
  </button>
  <button type="button" role="tab" aria-selected={tab === "github"} class="tab {tab === 'github' ? 'active' : ''}" onclick={() => (tab = "github")}>
    GitHub{#if repo}<span class="tab-count">1</span>{/if}
  </button>
  <button type="button" role="tab" aria-selected={tab === "workspace"} class="tab {tab === 'workspace' ? 'active' : ''}" onclick={() => (tab = "workspace")}>
    Workspace
  </button>
</div>

{#if tab === "deployments"}
  {#if deployments === null}
    <div class="deployment-grid">
      {#each { length: 3 } as _, i (i)}<DeploymentCardSkeleton />{/each}
    </div>
  {:else if deployments.length === 0}
    <div class="card empty-state">
      <p>{EMPTY_DEPLOYMENTS}</p>
      <div class="field-hint">
        A deployment is one running thing -- an app built from a repo, an image, or a template. Deploy from GitHub, from a
        template, or configure one by hand above.
      </div>
    </div>
  {:else if environments.length <= 1}
    {@render grid(deployments)}
  {:else}
    {#each environments as env (env)}
      {@const list = deployments.filter((d) => d.environment === env)}
      <div class="deployment-section">
        <div class="deployment-section-title">{ENV_LABEL[env] ?? env}<span class="tab-count">{list.length}</span></div>
        {@render grid(list)}
      </div>
    {/each}
  {/if}
{:else if tab === "github"}
  <div class="card">
    <div class="flex-between" style:margin-bottom={repo ? "14px" : "0"}>
      <div class="card-title" style="margin: 0">GitHub</div>
      {#if !repo}
        <button class="btn btn-sm" onclick={() => (showLinkRepo = true)}><Icon name="branch" /> Connect a repo</button>
      {/if}
    </div>
    {#if !repo}
      <p class="text-dim" style="margin: 0">
        Link a GitHub repo to auto-deploy on every push, get staging environments per branch, and preview deployments for
        open pull requests -- all configured per-deployment once this project has a repo.
      </p>
    {:else}
      <div class="kv-list">
        <div class="kv-row"><span class="kv-key">Repo</span><span class="kv-value">{repo.repo_owner}/{repo.repo_name}</span></div>
        <div class="kv-row"><span class="kv-key">Default branch</span><span class="kv-value">{repo.default_branch}</span></div>
        <div class="kv-row"><span class="kv-key">Webhook path</span><span class="kv-value">{repo.webhook_path}</span></div>
        <div class="field-hint">
          Set each deployment's auto-deploy branch from its own page. Paste the webhook URL and secret (shown once, at link
          time) into this repo's GitHub settings &rarr; Webhooks.
        </div>
      </div>
    {/if}
  </div>
{:else}
  <div class="card">
    <div class="card-title">Workspace</div>
    <div class="form-row" style="align-items: flex-end">
      <div class="field" style="margin-bottom: 0">
        <label for="project-workspace">Move to workspace</label>
        <select id="project-workspace" class="input" bind:value={workspaceId}>
          {#each workspaces as w (w.id)}<option value={w.id}>{w.name}</option>{/each}
        </select>
      </div>
      <button class="btn btn-sm" onclick={moveWorkspace}>Move</button>
    </div>
  </div>
{/if}

{#if showCreate}
  <CreateDeploymentModal {projectId} onClose={() => (showCreate = false)} onCreated={(id) => goto(`/projects/${projectId}/deployments/${id}`)} />
{/if}

{#if showLinkRepo}
  <LinkRepoModal
    {projectId}
    onClose={() => (showLinkRepo = false)}
    onLinked={() => {
      showLinkRepo = false;
      load();
    }}
  />
{/if}

{#if showTemplates}
  <TemplateGalleryModal
    {projectId}
    onClose={() => (showTemplates = false)}
    onInstalled={() => {
      showTemplates = false;
      load();
    }}
  />
{/if}

{#if showGithubWizard}
  <GithubDeployWizard
    {projectId}
    onClose={() => (showGithubWizard = false)}
    onDeployed={(_pid, deploymentId) => goto(`/projects/${projectId}/deployments/${deploymentId}`)}
  />
{/if}

{#if showDelete}
  <ConfirmModal
    title="Delete project"
    body={`This permanently deletes "${project?.name ?? "this project"}" and every deployment in it -- stopping containers, removing volumes, and releasing ports. This cannot be undone.`}
    confirmLabel="Delete project"
    onClose={() => (showDelete = false)}
    onConfirm={async () => {
      await api.del(`/api/projects/${projectId}`);
      window.location.href = "/";
    }}
  />
{/if}
