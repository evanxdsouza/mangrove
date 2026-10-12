<script lang="ts">
  import { page } from "$app/state";
  import { uiMode } from "$lib/stores/ui-mode.svelte";
  import DeploymentDetailPage from "$lib/pages/DeploymentDetailPage.svelte";
  import SimpleAppDetailPage from "$lib/pages/simple/SimpleAppDetailPage.svelte";
</script>

{#if uiMode.mode === "simple"}
  <SimpleAppDetailPage deploymentId={Number(page.params.deploymentId)} />
{:else}
  <DeploymentDetailPage projectId={Number(page.params.projectId)} deploymentId={Number(page.params.deploymentId)} />
{/if}
