<script lang="ts">
  import type { DeployHistory } from "../api";
  import { DOT_COLOR } from "../status";
  import { fmtDuration, fmtWhen } from "../format";
  import StatusPill from "./StatusPill.svelte";
  import CodeBlock from "./CodeBlock.svelte";

  // The deploy-history timeline: the root line running vertically. Each
  // entry links to its own run page when the project/deployment are known.
  let {
    history,
    onRollback,
    busyId,
    projectId,
    deploymentId,
  }: {
    history: DeployHistory[];
    onRollback: (id: number) => void;
    busyId: number | null;
    projectId?: number;
    deploymentId?: number;
  } = $props();
</script>

{#snippet body(h: DeployHistory)}
  <div class="timeline-meta">
    <StatusPill status={h.status} />
    {#if h.is_current}<span class="pill pill-gray">current</span>{/if}
    <span class="mono text-dim">{h.commit_sha ? h.commit_sha.slice(0, 8) : h.triggered_by}</span>
    {#if h.rollback_of_deploy_history_id}
      <span class="text-dim" style="font-size: 12px">rollback of #{h.rollback_of_deploy_history_id}</span>
    {/if}
  </div>
  <div class="timeline-time">
    {fmtWhen(h.started_at)} &middot; triggered by {h.triggered_by}{#if fmtDuration(h.started_at, h.finished_at)}&nbsp;&middot; {fmtDuration(h.started_at, h.finished_at)}{/if}
  </div>
  {#if h.commit_message}<div class="timeline-msg">{h.commit_message}</div>{/if}
  {#if h.error_message}
    <div style="margin-top: 8px"><CodeBlock tone="danger" text={h.error_message} /></div>
  {/if}
{/snippet}

{#if history.length === 0}
  <div class="empty-state"><p>No deploys yet.</p></div>
{:else}
  <div class="timeline">
    {#each history as h (h.id)}
      <div class="timeline-item">
        <div class="timeline-marker">
          <div class="dot" style:background={DOT_COLOR[h.status] ?? "var(--text-faint)"}></div>
        </div>
        {#if projectId != null && deploymentId != null}
          <a href="/projects/{projectId}/deployments/{deploymentId}/history/{h.id}" class="timeline-body timeline-item-link">
            {@render body(h)}
          </a>
        {:else}
          <div class="timeline-body">{@render body(h)}</div>
        {/if}
        {#if h.status === "success" && !h.is_current}
          <button class="btn btn-sm" disabled={busyId !== null} onclick={() => onRollback(h.id)}>
            {busyId === h.id ? "Rolling back..." : "Revert to this"}
          </button>
        {/if}
      </div>
    {/each}
  </div>
{/if}
