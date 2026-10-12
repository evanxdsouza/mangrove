<script lang="ts">
  import type { Deployment, Service } from "../../api";
  import RunCommandCard from "../RunCommandCard.svelte";
  import ScaleCard from "./ScaleCard.svelte";
  import ServiceCard from "./ServiceCard.svelte";

  let { services, deployment, canEdit }: { services: Service[]; deployment: Deployment | null; canEdit: boolean } = $props();
  const isStatic = $derived(deployment?.build_strategy === "static");
</script>

{#if services.length === 0}
  <div class="card empty-state"><p>No services yet.</p></div>
{:else}
  <!-- A single service's vitals are already the page-level stat strip; a
       compose deployment has no one canonical service, so each gets its own
       card here instead. -->
  {#if services.length > 1}
    <div class="grid grid-2">
      {#each services as s (s.id)}<ServiceCard service={s} {isStatic} />{/each}
    </div>
  {/if}
  {#if deployment}<ScaleCard deploymentId={deployment.id} {deployment} />{/if}
  <!-- Running a command in a live container is editor+ server-side, so it's
       hidden rather than shown-then-403ing for a viewer. -->
  {#if canEdit && !isStatic}
    {#each services as s (s.id)}
      <RunCommandCard serviceId={s.id} serviceName={s.name} showServiceName={services.length > 1} />
    {/each}
  {/if}
{/if}
