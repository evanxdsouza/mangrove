<script lang="ts">
  import type { ResourceUsageSnapshot } from "../api";
  import { fmtWhen } from "../format";

  // Resource-usage history: one tick per 5-minute snapshot, height from its
  // fraction of ceiling/total, colored by the same thresholds as the tide bars.
  let { snapshots, metric }: { snapshots: ResourceUsageSnapshot[]; metric: "memory" | "disk" } = $props();

  function toneColor(fraction: number): string {
    if (fraction > 0.9) return "var(--red)";
    if (fraction > 0.7) return "var(--ochre)";
    return "var(--tide-high)";
  }

  const ticks = $derived(
    snapshots.map((s) => {
      const fraction =
        metric === "memory" ? s.memory_used_mb / Math.max(s.memory_ceiling_mb, 1) : s.disk_used_gb / Math.max(s.disk_total_gb, 1);
      const clamped = Math.min(Math.max(fraction, 0), 1);
      const label =
        metric === "memory"
          ? `${s.memory_used_mb.toFixed(0)} / ${s.memory_ceiling_mb} MB`
          : `${s.disk_used_gb.toFixed(1)} / ${s.disk_total_gb.toFixed(1)} GB`;
      return { id: s.id, clamped, title: `${fmtWhen(s.recorded_at)} -- ${label}` };
    }),
  );
</script>

{#if snapshots.length === 0}
  <div class="text-dim" style="font-size: 12.5px; padding: 8px 0">Not enough history yet -- sampled every 5 minutes.</div>
{:else}
  <div class="resource-strip">
    {#each ticks as t (t.id)}
      <span
        class="resource-strip-tick"
        style:height="{Math.max(t.clamped * 100, 3)}%"
        style:background={toneColor(t.clamped)}
        title={t.title}
      ></span>
    {/each}
  </div>
{/if}
