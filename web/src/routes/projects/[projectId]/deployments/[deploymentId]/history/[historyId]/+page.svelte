<script lang="ts">
  import { page } from "$app/state";
  import { uiMode } from "$lib/stores/ui-mode.svelte";
  import RunDetailPage from "$lib/pages/RunDetailPage.svelte";
  import SimpleAppDetailPage from "$lib/pages/simple/SimpleAppDetailPage.svelte";
</script>

<!-- Deploy history is a technical-mode concept -- simple mode has no
     history tab to link a run out of, so a stray link lands on the app's
     own page instead. -->
{#if uiMode.mode === "simple"}
  <SimpleAppDetailPage deploymentId={Number(page.params.deploymentId)} />
{:else}
  <RunDetailPage
    projectId={Number(page.params.projectId)}
    deploymentId={Number(page.params.deploymentId)}
    historyId={Number(page.params.historyId)}
  />
{/if}
