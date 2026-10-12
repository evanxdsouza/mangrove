<script lang="ts">
  import { goto } from "$app/navigation";
  import { api, ApiError } from "../api";
  import { reloadWorkspaces, setActiveWorkspaceId, workspaceState } from "../stores/workspaces.svelte";
  import Icon from "../components/Icon.svelte";
  import CenterLoading from "../components/CenterLoading.svelte";
  import ActivityModal from "../components/workspaces/ActivityModal.svelte";
  import CreateWorkspaceModal from "../components/workspaces/CreateWorkspaceModal.svelte";
  import ManageMembersModal from "../components/workspaces/ManageMembersModal.svelte";

  let error = $state<string | null>(null);
  let showCreate = $state(false);
  let membersFor = $state<{ id: number; name: string } | null>(null);
  let activityFor = $state<{ id: number; name: string } | null>(null);

  function view(e: MouseEvent, id: number) {
    e.preventDefault();
    setActiveWorkspaceId(id);
    goto("/");
  }

  async function remove(id: number, name: string) {
    if (!confirm(`Delete workspace "${name}"? Its projects will be moved to the default workspace.`)) return;
    try {
      await api.del(`/api/workspaces/${id}`);
      reloadWorkspaces();
    } catch (e) {
      error = e instanceof ApiError ? e.message : "Failed to delete workspace";
    }
  }
</script>

<nav class="breadcrumb" aria-label="Breadcrumb">
  <a href="/" class="breadcrumb-back"><Icon name="chevron-down" /> Projects</a>
  <span>/</span>
  <span>Workspaces</span>
</nav>
<div class="page-header">
  <div>
    <h1>Workspaces</h1>
    <p>Group projects by environment (e.g. production, staging) or team. Pick one from the sidebar's station switcher to scope Projects.</p>
  </div>
  <button class="btn btn-primary" onclick={() => (showCreate = true)}><Icon name="plus" /> New workspace</button>
</div>

{#if error}<div class="error-banner">{error}</div>{/if}

{#if workspaceState.workspaces.length === 0}
  <CenterLoading />
{:else}
  <div class="card">
    <table>
      <thead>
        <tr><th>Name</th><th>Slug</th><th>Projects</th><th></th></tr>
      </thead>
      <tbody>
        {#each workspaceState.workspaces as w (w.workspace.id)}
          <tr>
            <td>
              <a href="/" onclick={(e) => view(e, w.workspace.id)} style="display: inline-flex; align-items: center; gap: 7px">
                <Icon name="mangrove" style="width: 14px; height: 14px; color: var(--brass-dim)" />
                {w.workspace.name}
              </a>
            </td>
            <td class="mono text-dim">{w.workspace.slug}</td>
            <td class="text-dim">{w.project_count}</td>
            <td class="text-right">
              {#if w.your_role}
                <button class="btn btn-sm" style="margin-right: 8px" onclick={() => (activityFor = { id: w.workspace.id, name: w.workspace.name })}>
                  <Icon name="ledger" /> Activity
                </button>
              {/if}
              {#if w.your_role === "admin"}
                <button class="btn btn-sm" style="margin-right: 8px" onclick={() => (membersFor = { id: w.workspace.id, name: w.workspace.name })}>
                  <Icon name="user" /> Members
                </button>
              {/if}
              {#if w.workspace.id !== 1 && w.your_role === "admin"}
                <button class="btn btn-sm btn-danger" onclick={() => remove(w.workspace.id, w.workspace.name)}><Icon name="trash" /> Delete</button>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

{#if showCreate}
  <CreateWorkspaceModal
    onClose={() => (showCreate = false)}
    onCreated={() => {
      showCreate = false;
      reloadWorkspaces();
    }}
  />
{/if}

{#if membersFor}
  <ManageMembersModal workspaceId={membersFor.id} workspaceName={membersFor.name} onClose={() => (membersFor = null)} />
{/if}

{#if activityFor}
  <ActivityModal workspaceId={activityFor.id} workspaceName={activityFor.name} onClose={() => (activityFor = null)} />
{/if}
