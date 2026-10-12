<script lang="ts">
  import { LOADING } from "../copy";

  let { label = LOADING }: { label?: string } = $props();
</script>

<div class="center-loading center-loading-labeled">
  <div class="spinner"></div>
  <span class="text-faint">{label}</span>
</div>
