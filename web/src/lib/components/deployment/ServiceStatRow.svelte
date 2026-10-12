<script lang="ts">
  import type { Service } from "../../api";
  import StatusPill from "../StatusPill.svelte";
  import { latestHealth } from "./latestHealth.svelte";

  // The page-level stat strip for the resource-detail layout. Mirrors the
  // fields ServiceCard shows for the multi-service case, so the two never
  // drift apart on what a service's vitals are.
  let { service, isStatic }: { service: Service; isStatic: boolean } = $props();
  const health = latestHealth(
    () => service.id,
    () => isStatic,
  );
</script>

{#snippet cell(label: string, value: string, mono = false, title?: string)}
  <div class="instrument-cell">
    <div class="stat-tile">
      <div class="stat-value {mono ? 'mono' : ''}" {title}>{value}</div>
      <div class="stat-label">{label}</div>
    </div>
  </div>
{/snippet}

<div class="instrument-row deployment-stat-row">
  {#if isStatic}
    {@render cell("Served by", "Caddy (file_server)")}
  {:else}
    {@render cell("Image", service.image_tag_current ?? "—", true)}
    {@render cell("Internal port", String(service.internal_port || "—"), true)}
    {@render cell(
      "Internal address",
      `${service.container_name}:${service.internal_port || "?"}`,
      true,
      "Reachable from any other deployment on this box, over the private Docker network every managed container joins",
    )}
    {@render cell("Resources", `${service.cpu_limit_cores} CPU / ${service.memory_limit_mb}MB`)}
    {@render cell("Replicas", String((service.replica_container_ids?.length ?? 1) || 1))}
  {/if}
  {@render cell("Host port", String(service.host_port ?? (service.is_internal_only ? "internal only" : "not yet assigned")), true)}
  {#if !isStatic}
    <div class="instrument-cell">
      <div class="stat-tile">
        <div class="stat-value">
          {#if health.current}
            <StatusPill status={health.current.status} /> <span class="text-dim">{health.current.response_time_ms}ms</span>
          {:else}
            <span class="text-faint">no checks yet</span>
          {/if}
        </div>
        <div class="stat-label">Health</div>
      </div>
    </div>
  {/if}
</div>
