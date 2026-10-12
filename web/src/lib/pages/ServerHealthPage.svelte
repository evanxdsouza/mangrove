<script lang="ts">
  import { api } from "../api";
  import Icon from "../components/Icon.svelte";
  import StatTile from "../components/StatTile.svelte";

  // Live status of the whole host -- CPU/RAM/disk/load of the machine, not
  // just Mangrove's containers. Refreshes every 5 seconds.
  interface SystemHealth {
    hostname: string;
    kernel: string;
    uptime_seconds: number;
    load_avg_1: number;
    load_avg_5: number;
    load_avg_15: number;
    cpu_count: number;
    cpu_percent: number;
    memory_total_gb: number;
    memory_used_gb: number;
    memory_used_pct: number;
    swap_total_gb: number;
    swap_used_gb: number;
    disk_total_gb: number;
    disk_used_gb: number;
    disk_used_pct: number;
    processes: number;
    running_process: number;
  }

  function fmtGB(gb: number): string {
    return `${gb.toFixed(1)} GB`;
  }

  function fmtUptime(seconds: number): string {
    const d = Math.floor(seconds / 86400);
    const h = Math.floor((seconds % 86400) / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const parts: string[] = [];
    if (d > 0) parts.push(`${d}d`);
    if (h > 0 || d > 0) parts.push(`${h}h`);
    parts.push(`${m}m`);
    return parts.join(" ");
  }

  let health = $state<SystemHealth | null>(null);
  let error = $state<string | null>(null);

  $effect(() => {
    let cancelled = false;
    const load = () => {
      api
        .get<SystemHealth>("/api/admin/system-health")
        .then((h) => {
          if (cancelled) return;
          health = h;
          error = null;
        })
        .catch((e) => {
          if (!cancelled) error = e instanceof Error ? e.message : "Failed to load server health";
        });
    };
    load();
    const timer = setInterval(load, 5000);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  });
</script>

<div class="page-header">
  <div>
    <h1>Server health</h1>
    <p>Live status of the whole host, not just Mangrove's containers.</p>
  </div>
  <Icon name="gauge" />
</div>

{#if error}<div class="error-banner">{error}</div>{/if}

{#if health}
  <div class="card">
    <div class="card-title">System</div>
    <table>
      <tbody>
        <tr><td class="text-dim">Host</td><td class="mono">{health.hostname}</td></tr>
        <tr><td class="text-dim">Kernel</td><td class="mono">{health.kernel || "—"}</td></tr>
        <tr><td class="text-dim">Uptime</td><td>{fmtUptime(health.uptime_seconds)}</td></tr>
        <tr><td class="text-dim">Processes</td><td>{health.running_process} running / {health.processes} total</td></tr>
      </tbody>
    </table>
  </div>

  <div class="card">
    <div class="card-title">Instruments</div>
    <div class="instrument-row">
      <StatTile
        label="CPU"
        value="{health.cpu_percent.toFixed(0)}% of {health.cpu_count} core{health.cpu_count === 1 ? '' : 's'}"
        fraction={health.cpu_percent / 100}
        size={16}
      />
      <StatTile label="Memory" value="{fmtGB(health.memory_used_gb)} / {fmtGB(health.memory_total_gb)}" fraction={health.memory_used_pct / 100} size={16} />
      <StatTile label="Disk" value="{fmtGB(health.disk_used_gb)} / {fmtGB(health.disk_total_gb)}" fraction={health.disk_used_pct / 100} size={16} />
      <StatTile
        label="Load"
        value="{health.load_avg_1.toFixed(2)} / {health.load_avg_5.toFixed(2)} / {health.load_avg_15.toFixed(2)}"
        fraction={health.cpu_count > 0 ? health.load_avg_1 / health.cpu_count : 0}
        size={16}
      />
      {#if health.swap_total_gb > 0}
        <StatTile
          label="Swap used"
          value="{fmtGB(health.swap_used_gb)} / {fmtGB(health.swap_total_gb)}"
          fraction={health.swap_used_gb / health.swap_total_gb}
          size={16}
        />
      {/if}
    </div>
  </div>
{:else}
  <div class="card"><div class="text-dim">Loading...</div></div>
{/if}
