<script lang="ts">
  import type { AuditEvent } from "../api";
  import { fmtWhen, humanLabel } from "../format";

  let { events, showWorkspace = false }: { events: AuditEvent[] | null; showWorkspace?: boolean } = $props();
</script>

{#if events === null}
  <div class="text-dim">Loading...</div>
{:else if events.length === 0}
  <div class="empty-state"><p>No activity recorded yet.</p></div>
{:else}
  <table>
    <thead>
      <tr>
        <th>When</th>
        <th>Who</th>
        <th>Action</th>
        <th>Resource</th>
        {#if showWorkspace}<th>Workspace</th>{/if}
        <th>Detail</th>
      </tr>
    </thead>
    <tbody>
      {#each events as e (e.id)}
        <tr>
          <td class="text-dim mono">{fmtWhen(e.created_at)}</td>
          <td>{e.actor_email}</td>
          <td>{humanLabel(e.action)}</td>
          <td class="text-dim">{humanLabel(e.resource_type)}{e.resource_id != null ? ` #${e.resource_id}` : ""}</td>
          {#if showWorkspace}<td class="text-dim">{e.workspace_id ?? "—"}</td>{/if}
          <td class="text-dim">{e.detail || "—"}</td>
        </tr>
      {/each}
    </tbody>
  </table>
{/if}
