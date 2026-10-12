<script lang="ts">
  import { api, ApiError, type AuditEvent } from "../../api";
  import Modal from "../Modal.svelte";
  import AuditLogTable from "../AuditLogTable.svelte";

  // Viewer+ (matching GET /api/workspaces/{id}/audit-log's own gate) --
  // accountability for who deployed/deleted/changed access is something a
  // workspace's own members should be able to see, not just its admins.
  let { workspaceId, workspaceName, onClose }: { workspaceId: number; workspaceName: string; onClose: () => void } = $props();

  let events = $state<AuditEvent[] | null>(null);
  let error = $state<string | null>(null);

  $effect(() => {
    api
      .get<AuditEvent[]>(`/api/workspaces/${workspaceId}/audit-log`)
      .then((e) => (events = e ?? []))
      .catch((e) => (error = e instanceof ApiError ? e.message : "Failed to load activity"));
  });
</script>

<Modal title={`Activity in "${workspaceName}"`} {onClose}>
  {#snippet children(close)}
    {#if error}<div class="error-banner">{error}</div>{/if}
    <AuditLogTable {events} />
    <div class="modal-actions"><button type="button" class="btn" onclick={close}>Close</button></div>
  {/snippet}
</Modal>
