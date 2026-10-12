<script lang="ts">
  import { uiMode } from "$lib/stores/ui-mode.svelte";
  import ProjectsPage from "$lib/pages/ProjectsPage.svelte";
  import SimpleAppsPage from "$lib/pages/simple/SimpleAppsPage.svelte";
</script>

{#if uiMode.mode === "simple"}
  <SimpleAppsPage />
{:else}
  <ProjectsPage />
{/if}
