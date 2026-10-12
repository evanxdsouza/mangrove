<script lang="ts">
  import { uiMode } from "$lib/stores/ui-mode.svelte";
  import WorkspacesPage from "$lib/pages/WorkspacesPage.svelte";
  import SimpleAppsPage from "$lib/pages/simple/SimpleAppsPage.svelte";
</script>

<!-- Workspaces are a technical grouping concept; simple mode has no
     equivalent page, so a direct navigation lands on the apps list. -->
{#if uiMode.mode === "simple"}
  <SimpleAppsPage />
{:else}
  <WorkspacesPage />
{/if}
