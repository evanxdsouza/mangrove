<script lang="ts">
  import { page } from "$app/state";
  import { uiMode } from "$lib/stores/ui-mode.svelte";
  import ProjectDetailPage from "$lib/pages/ProjectDetailPage.svelte";
  import SimpleAppsPage from "$lib/pages/simple/SimpleAppsPage.svelte";
</script>

<!-- Simple mode has no "project" grouping page -- an app is a deployment,
     reached straight from the apps list -- so a stray link here (an old
     bookmark) lands on the apps list instead. -->
{#if uiMode.mode === "simple"}
  <SimpleAppsPage />
{:else}
  <ProjectDetailPage projectId={Number(page.params.projectId)} />
{/if}
