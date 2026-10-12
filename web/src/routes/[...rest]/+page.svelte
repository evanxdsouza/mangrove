<script lang="ts">
  import { page } from "$app/state";
  import NotFoundPage from "$lib/pages/NotFoundPage.svelte";
</script>

<!-- Catch-all: any path no other route matches renders the 404 page here,
     rather than through +error.svelte (which SvelteKit logs as an error). -->
<NotFoundPage path={page.url.pathname} />
