<script lang="ts">
  import { api, ApiError, type WorkspaceMember, type WorkspaceRole } from "../../api";
  import Modal from "../Modal.svelte";
  import Icon from "../Icon.svelte";

  // Admin-only, both server-side (RequireWorkspaceRole on
  // /api/workspaces/{id}/members) and here (only opened for a workspace
  // where your_role === "admin"). Adding is by exact email match against an
  // *existing* account -- deliberately not a picker over the full user
  // roster, which stays reserved for global owners (docs/multi-user.md).
  let { workspaceId, workspaceName, onClose }: { workspaceId: number; workspaceName: string; onClose: () => void } = $props();

  const ROLES: WorkspaceRole[] = ["viewer", "editor", "admin"];

  let members = $state<WorkspaceMember[] | null>(null);
  let error = $state<string | null>(null);
  let email = $state("");
  let role = $state<WorkspaceRole>("editor");
  let adding = $state(false);

  function load() {
    api
      .get<WorkspaceMember[]>(`/api/workspaces/${workspaceId}/members`)
      .then((m) => (members = m ?? []))
      .catch((e) => (error = e instanceof ApiError ? e.message : "Failed to load members"));
  }
  $effect(() => {
    void workspaceId;
    load();
  });

  async function add(e: SubmitEvent) {
    e.preventDefault();
    if (!email.trim()) return;
    adding = true;
    error = null;
    try {
      await api.post(`/api/workspaces/${workspaceId}/members`, { email: email.trim(), role });
      email = "";
      load();
    } catch (err) {
      error = err instanceof ApiError ? err.message : "Failed to add member";
    } finally {
      adding = false;
    }
  }

  async function setMemberRole(userId: number, newRole: WorkspaceRole) {
    try {
      await api.put(`/api/workspaces/${workspaceId}/members/${userId}`, { role: newRole });
      load();
    } catch (err) {
      error = err instanceof ApiError ? err.message : "Failed to change role";
    }
  }

  async function remove(userId: number) {
    try {
      await api.del(`/api/workspaces/${workspaceId}/members/${userId}`);
      load();
    } catch (err) {
      error = err instanceof ApiError ? err.message : "Failed to remove member";
    }
  }
</script>

<Modal title={`Members of "${workspaceName}"`} {onClose}>
  {#snippet children(close)}
    {#if error}<div class="error-banner">{error}</div>{/if}
    {#if members && members.length > 0}
      <div class="kv-list" style="margin-bottom: 16px">
        {#each members as m (m.user_id)}
          <div class="kv-row">
            <span class="kv-key">{m.email}</span>
            <select
              class="input mono"
              style="width: auto"
              aria-label="Role for {m.email}"
              value={m.role}
              onchange={(e) => setMemberRole(m.user_id, e.currentTarget.value as WorkspaceRole)}
            >
              {#each ROLES as r (r)}<option value={r}>{r}</option>{/each}
            </select>
            <button class="btn btn-sm btn-danger" onclick={() => remove(m.user_id)}><Icon name="trash" /> Remove</button>
          </div>
        {/each}
      </div>
    {/if}
    <form onsubmit={add} class="form-row" style="align-items: flex-end">
      <div class="field" style="margin-bottom: 0; flex: 1">
        <label for="member-email">Add an existing account by email</label>
        <input id="member-email" class="input" type="email" required bind:value={email} placeholder="teammate@example.com" />
      </div>
      <div class="field" style="margin-bottom: 0">
        <label for="member-role">Role</label>
        <select id="member-role" class="input mono" bind:value={role}>
          {#each ROLES as r (r)}<option value={r}>{r}</option>{/each}
        </select>
      </div>
      <button class="btn" type="submit" disabled={adding || !email.trim()}><Icon name="plus" /> Add</button>
    </form>
    <div class="modal-actions"><button type="button" class="btn" onclick={close}>Close</button></div>
  {/snippet}
</Modal>
