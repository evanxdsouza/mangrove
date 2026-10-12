<script lang="ts">
  import { api, ApiError } from "../../api";
  import { slugify } from "../../slug";
  import Modal from "../Modal.svelte";

  let { onClose, onCreated }: { onClose: () => void; onCreated: () => void } = $props();

  let name = $state("");
  let slug = $state("");
  let error = $state<string | null>(null);
  let busy = $state(false);

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    busy = true;
    error = null;
    try {
      await api.post("/api/workspaces", { name, slug });
      onCreated();
    } catch (err) {
      error = err instanceof ApiError ? err.message : "Failed to create workspace";
    } finally {
      busy = false;
    }
  }
</script>

<Modal title="New workspace" {onClose}>
  {#snippet children(close)}
    {#if error}<div class="error-banner">{error}</div>{/if}
    <form onsubmit={submit}>
      <div class="field">
        <label for="ws-name">Name</label>
        <input
          id="ws-name"
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
        <label for="ws-slug">Slug</label>
        <input id="ws-slug" class="input mono" required value={slug} oninput={(e) => (slug = slugify(e.currentTarget.value))} />
      </div>
      <div class="modal-actions">
        <button type="button" class="btn" onclick={close}>Cancel</button>
        <button type="submit" class="btn btn-primary" disabled={busy}>{busy ? "Creating..." : "Create workspace"}</button>
      </div>
    </form>
  {/snippet}
</Modal>
