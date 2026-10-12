<script lang="ts">
  import type { Service } from "../../api";
  import StatusPill from "../StatusPill.svelte";
  import { latestHealth } from "./latestHealth.svelte";

  let { service, isStatic }: { service: Service; isStatic: boolean } = $props();
  const health = latestHealth(
    () => service.id,
    () => isStatic,
  );
</script>

<div class="card">
  <div class="flex-between" style="margin-bottom: 12px">
    <div class="card-title" style="margin: 0">{service.name}</div>
    <StatusPill status={service.status} />
  </div>
  <div class="kv-list">
    {#if isStatic}
      <div class="kv-row"><span class="kv-key">Served by</span><span class="kv-value">Caddy (file_server) -- no container runs for a static site</span></div>
    {:else}
      <div class="kv-row"><span class="kv-key">Image</span><span class="kv-value">{service.image_tag_current ?? "—"}</span></div>
      <div class="kv-row"><span class="kv-key">Internal port</span><span class="kv-value">{service.internal_port || "—"}</span></div>
      <div class="kv-row">
        <span class="kv-key">Internal address</span>
        <span class="kv-value mono">{service.container_name}:{service.internal_port || "?"}</span>
      </div>
      <div class="kv-row"><span class="kv-key">Resources</span><span class="kv-value">{service.cpu_limit_cores} CPU / {service.memory_limit_mb}MB</span></div>
      <div class="kv-row"><span class="kv-key">Replicas</span><span class="kv-value">{(service.replica_container_ids?.length ?? 1) || 1}</span></div>
    {/if}
    <div class="kv-row">
      <span class="kv-key">Host port</span>
      <span class="kv-value">{service.host_port ?? (service.is_internal_only ? "internal only" : "not yet assigned")}</span>
    </div>
    {#if !isStatic}
      <div class="kv-row">
        <span class="kv-key">Health</span>
        <span class="kv-value">
          {#if health.current}
            <StatusPill status={health.current.status} /> {health.current.response_time_ms}ms
          {:else}
            <span class="text-faint">no checks yet</span>
          {/if}
        </span>
      </div>
    {/if}
  </div>
</div>
