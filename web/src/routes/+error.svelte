<script lang="ts">
  import { page } from "$app/state";
  import ErrorFallback from "$lib/components/ErrorFallback.svelte";
  import NotFoundPage from "$lib/pages/NotFoundPage.svelte";
</script>

{#if page.status === 404}
  <NotFoundPage path={page.url.pathname} />
{:else}
  <ErrorFallback error={page.error?.message ?? "Unknown error"} />
{/if}
