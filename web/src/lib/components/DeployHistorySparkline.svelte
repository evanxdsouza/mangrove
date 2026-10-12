<script lang="ts">
  import type { DeployHistory } from "../api";
  import { DOT_COLOR } from "../status";
  import { fmtWhen } from "../format";

  // A run-outcome strip above the History tab's timeline: one uniform tick
  // per run, colored by its real status, oldest on the left. `history`
  // arrives newest-first; capped to the most recent 40.
  let { history }: { history: DeployHistory[] } = $props();
  const recent = $derived(history.slice(0, 40).slice().reverse());
</script>

{#if history.length >= 2}
  <div class="deploy-sparkline" title="Recent deploy outcomes, oldest to newest">
    {#each recent as h (h.id)}
      <span
        class="deploy-sparkline-tick"
        style:background={DOT_COLOR[h.status] ?? "var(--text-faint)"}
        title="{h.status.replace(/_/g, ' ')} · {fmtWhen(h.started_at)}"
      ></span>
    {/each}
  </div>
{/if}
