<script lang="ts">
  import { goto } from "$app/navigation";
  import { page } from "$app/state";
  import { setActiveWorkspaceId, workspaceState } from "../stores/workspaces.svelte";
  import Icon from "./Icon.svelte";

  // The active workspace is ambient context pinned at the top of the
  // sidebar, changeable in one click from anywhere in the technical
  // dashboard, rather than an orphaned nav item detouring to its own page.
  let open = $state(false);
  let rootEl = $state<HTMLDivElement | null>(null);

  const label = $derived(
    workspaceState.workspaces.find((w) => w.workspace.id === workspaceState.activeWorkspaceId)?.workspace.name ?? "All workspaces",
  );

  // Close after any navigation, including one this panel triggered.
  $effect(() => {
    void page.url.pathname;
    open = false;
  });

  function pick(id: number | null) {
    setActiveWorkspaceId(id);
    open = false;
    goto("/");
  }
</script>

<svelte:window onkeydown={(e) => open && e.key === "Escape" && (open = false)} />
<svelte:document onmousedown={(e) => open && rootEl && !rootEl.contains(e.target as Node) && (open = false)} />

<div class="station-switcher" bind:this={rootEl}>
  <button type="button" class="station-switcher-button" aria-expanded={open} aria-haspopup="listbox" onclick={() => (open = !open)}>
    <span class="station-switcher-labels">
      <span class="station-switcher-eyebrow">Station</span>
      <span class="station-switcher-name">{label}</span>
    </span>
    <Icon name="chevron-down" class="station-switcher-chevron" />
  </button>

  {#if open}
    <div class="station-switcher-panel" role="listbox">
      <button
        type="button"
        class="station-switcher-item {workspaceState.activeWorkspaceId == null ? 'active' : ''}"
        onclick={() => pick(null)}
      >
        <span>All workspaces</span>
      </button>
      {#each workspaceState.workspaces as w (w.workspace.id)}
        <button
          type="button"
          class="station-switcher-item {workspaceState.activeWorkspaceId === w.workspace.id ? 'active' : ''}"
          onclick={() => pick(w.workspace.id)}
        >
          <span>{w.workspace.name}</span>
          <span class="station-switcher-item-count">{w.project_count} project{w.project_count === 1 ? "" : "s"}</span>
        </button>
      {/each}
      <div class="station-switcher-divider"></div>
      <button
        type="button"
        class="station-switcher-manage"
        onclick={() => {
          open = false;
          goto("/workspaces");
        }}
      >
        <Icon name="gear" />
        Manage workspaces
      </button>
    </div>
  {/if}
</div>
