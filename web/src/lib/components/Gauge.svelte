<script lang="ts">
  // A resource tide bar for a reading (CPU/memory/disk/load/swap): the
  // fraction fills a rounded track -- teal to 70%, amber above 70%, red
  // above 90% -- with the percentage beside it. Renders alongside the
  // actual number and label, never in place of them.
  let { fraction, tone = "default" }: { fraction: number; tone?: "default" | "warn" | "danger" } = $props();
  const pct = $derived(Math.round(Math.min(Math.max(fraction, 0), 1) * 100));
</script>

<div class="tide-bar-row">
  <div class="tide-bar" role="meter" aria-valuemin={0} aria-valuemax={100} aria-valuenow={pct} aria-label="{pct}%">
    <i class={tone !== "default" ? tone : undefined} style:width="{pct}%"></i>
  </div>
  <span class="tide-bar-pct">{pct}%</span>
</div>
