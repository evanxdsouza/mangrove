<script lang="ts">
  import { tick } from "svelte";
  import { goto } from "$app/navigation";
  import { api, type Deployment, type Project } from "../api";
  import Icon from "./Icon.svelte";
  import TideGauge from "./TideGauge.svelte";

  // Jump straight to a project or deployment by name/slug from anywhere in
  // the technical dashboard -- Ctrl/Cmd+K focuses it from any page.
  // Client-side only: one /api/projects call (every workspace) cached for
  // the component's lifetime; deployments load lazily the first time a
  // query needs them, so most visits pay no N+1 cost.
  let { collapsed, onRequestExpand }: { collapsed: boolean; onRequestExpand: () => void } = $props();

  interface SearchMatch {
    kind: "project" | "deployment";
    id: number;
    name: string;
    slug: string;
    projectId: number;
    projectName?: string;
    status?: string;
  }

  let projects = $state<Project[] | null>(null);
  let deploymentsByProject = $state<Record<number, Deployment[]> | null>(null);
  let loadingDeployments = $state(false);
  let query = $state("");
  let open = $state(false);
  let activeIndex = $state(0);
  let rootEl = $state<HTMLDivElement | null>(null);
  let inputEl = $state<HTMLInputElement | null>(null);

  api
    .get<Project[]>("/api/projects")
    .then((p) => (projects = p ?? []))
    .catch(() => (projects = []));

  function ensureDeploymentsLoaded() {
    if (deploymentsByProject != null || loadingDeployments || projects == null) return;
    loadingDeployments = true;
    Promise.all(
      projects.map((p) =>
        api
          .get<Deployment[]>(`/api/projects/${p.id}/deployments`)
          .then((d): [number, Deployment[]] => [p.id, d ?? []])
          .catch((): [number, Deployment[]] => [p.id, []]),
      ),
    ).then((entries) => {
      deploymentsByProject = Object.fromEntries(entries);
      loadingDeployments = false;
    });
  }

  // When Ctrl/Cmd+K (or the rail's search button) fires while collapsed,
  // the input doesn't exist until the sidebar re-expands; focus it after.
  async function focusAfterExpand() {
    if (collapsed) {
      onRequestExpand();
      await tick();
    }
    inputEl?.focus();
    inputEl?.select();
  }

  function onWindowKey(e: KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
      e.preventDefault();
      focusAfterExpand();
    }
  }

  function onDocMouseDown(e: MouseEvent) {
    if (open && rootEl && !rootEl.contains(e.target as Node)) open = false;
  }

  const matches = $derived.by((): SearchMatch[] => {
    const q = query.trim().toLowerCase();
    if (!q) return [];
    const projectNameById = new Map((projects ?? []).map((p) => [p.id, p.name]));
    return [
      ...(projects ?? [])
        .filter((p) => p.name.toLowerCase().includes(q) || p.slug.toLowerCase().includes(q))
        .map((p): SearchMatch => ({ kind: "project", id: p.id, name: p.name, slug: p.slug, projectId: p.id })),
      ...Object.entries(deploymentsByProject ?? {}).flatMap(([projectId, deps]) =>
        deps
          .filter((d) => d.name.toLowerCase().includes(q) || d.slug.toLowerCase().includes(q))
          .map(
            (d): SearchMatch => ({
              kind: "deployment",
              id: d.id,
              name: d.name,
              slug: d.slug,
              projectId: Number(projectId),
              projectName: projectNameById.get(Number(projectId)),
              status: d.status,
            }),
          ),
      ),
    ].slice(0, 8);
  });

  function go(m: SearchMatch) {
    query = "";
    open = false;
    inputEl?.blur();
    goto(m.kind === "project" ? `/projects/${m.projectId}` : `/projects/${m.projectId}/deployments/${m.id}`);
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      open = false;
      inputEl?.blur();
    } else if (e.key === "ArrowDown" && matches.length > 0) {
      e.preventDefault();
      activeIndex = (activeIndex + 1) % matches.length;
    } else if (e.key === "ArrowUp" && matches.length > 0) {
      e.preventDefault();
      activeIndex = (activeIndex - 1 + matches.length) % matches.length;
    } else if (e.key === "Enter" && matches[activeIndex]) {
      go(matches[activeIndex]);
    }
  }
</script>

<svelte:window onkeydown={onWindowKey} />
<svelte:document onmousedown={onDocMouseDown} />

{#if collapsed}
  <button type="button" class="nav-link sidebar-search-collapsed-btn" title="Search (Ctrl/Cmd+K)" aria-label="Search" onclick={focusAfterExpand}>
    <Icon name="search" />
  </button>
{:else}
  <div class="sidebar-search" bind:this={rootEl}>
    <Icon name="search" class="sidebar-search-icon" />
    <input
      bind:this={inputEl}
      class="sidebar-search-input"
      placeholder="Search projects and deployments"
      aria-label="Search projects and deployments"
      bind:value={query}
      oninput={() => {
        activeIndex = 0;
        open = true;
        if (query.trim()) ensureDeploymentsLoaded();
      }}
      onfocus={() => {
        if (query) {
          open = true;
          ensureDeploymentsLoaded();
        }
      }}
      onkeydown={onKeydown}
    />
    {#if !query}<span class="sidebar-search-kbd">&#8984;K</span>{/if}
    {#if open && query}
      <div class="sidebar-search-panel">
        {#if matches.length === 0}
          <div class="sidebar-search-empty">
            {#if loadingDeployments}Searching...{:else}No projects or deployments match &ldquo;{query}&rdquo;{/if}
          </div>
        {:else}
          {#each matches as m, i (`${m.kind}-${m.id}`)}
            <button
              type="button"
              class="sidebar-search-item {i === activeIndex ? 'active' : ''}"
              onmouseenter={() => (activeIndex = i)}
              onclick={() => go(m)}
            >
              {#if m.kind === "project"}
                <Icon name="ledger" style="width: 16px; height: 16px; flex-shrink: 0; color: var(--leaf)" />
              {:else}
                <TideGauge status={m.status} size="sm" />
              {/if}
              <span class="sidebar-search-item-text">
                <b>{m.name}</b>
                <span class="sidebar-search-item-slug {m.kind === 'project' ? 'mono' : ''}">
                  {m.kind === "deployment" ? m.projectName : m.slug}
                </span>
              </span>
            </button>
          {/each}
        {/if}
      </div>
    {/if}
  </div>
{/if}
