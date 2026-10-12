<script lang="ts">
  import { api, ApiError, type Project } from "../../api";
  import { slugify } from "../../slug";
  import Modal from "../Modal.svelte";

  let {
    workspaces,
    defaultWorkspaceId,
    onClose,
    onCreated,
  }: {
    workspaces: { id: number; name: string }[];
    defaultWorkspaceId?: number;
    onClose: () => void;
    onCreated: (id: number) => void;
  } = $props();

  let name = $state("");
  let slug = $state("");
  let description = $state("");
  // svelte-ignore state_referenced_locally
  let workspaceId = $state<number>(defaultWorkspaceId ?? 1);
  let error = $state<string | null>(null);
  let busy = $state(false);

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    busy = true;
    error = null;
    try {
      const project = await api.post<Project>("/api/projects", { name, slug, description, workspace_id: workspaceId });
      onCreated(project.id);
    } catch (err) {
      error = err instanceof ApiError ? err.message : "Failed to create project";
    } finally {
      busy = false;
    }
  }
</script>

<Modal title="New project" {onClose}>
  {#snippet children(close)}
    {#if error}<div class="error-banner">{error}</div>{/if}
    <form onsubmit={submit}>
      <div class="field">
        <label for="project-name">Name</label>
        <input
          id="project-name"
          class="input"
          required
          value={name}
          oninput={(e) => {
            name = e.currentTarget.value;
            if (!slug) slug = slugify(name);
          }}
        />
      </div>
      <div class="field">
        <label for="project-slug">Slug</label>
        <input id="project-slug" class="input mono" required value={slug} oninput={(e) => (slug = slugify(e.currentTarget.value))} />
      </div>
      <div class="field">
        <label for="project-description">Description (optional)</label>
        <textarea id="project-description" class="input" rows="2" bind:value={description}></textarea>
      </div>
      <div class="field">
        <label for="project-workspace">Workspace</label>
        <select id="project-workspace" class="input" bind:value={workspaceId}>
          {#each workspaces as w (w.id)}
            <option value={w.id}>{w.name}</option>
          {/each}
        </select>
      </div>
      <div class="modal-actions">
        <button type="button" class="btn" onclick={close}>Cancel</button>
        <button type="submit" class="btn btn-primary" disabled={busy}>{busy ? "Creating..." : "Create project"}</button>
      </div>
    </form>
  {/snippet}
</Modal>
