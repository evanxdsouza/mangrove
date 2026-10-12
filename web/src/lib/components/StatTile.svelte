<script lang="ts">
  import Gauge from "./Gauge.svelte";

  let { label, value, fraction, size = 15 }: { label: string; value: string; fraction: number; size?: number } = $props();
  const tone = $derived(fraction > 0.9 ? "danger" : fraction > 0.7 ? "warn" : "default");
</script>

<div class="instrument-cell">
  <Gauge {fraction} {tone} />
  <div class="stat-tile">
    <div class="stat-value" style:font-size="{size}px">{value}</div>
    <div class="stat-label">{label}</div>
  </div>
</div>
