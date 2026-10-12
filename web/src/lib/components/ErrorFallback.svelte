<script lang="ts">
  import Icon from "./Icon.svelte";

  let { error }: { error: unknown } = $props();
  const message = $derived(error instanceof Error ? error.message : String(error));
</script>

<div class="card empty-state">
  <Icon name="alert" />
  <p>Something went wrong loading this page.</p>
  <p class="text-dim mono" style="font-size: 12px">{message}</p>
</div>
