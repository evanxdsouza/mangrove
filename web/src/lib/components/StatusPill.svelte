<script lang="ts">
  import { IN_PROGRESS_STATUSES, STATUS_COLORS } from "../status";

  let { status }: { status: string } = $props();
</script>

<span class="pill pill-{STATUS_COLORS[status] ?? 'gray'}">
  <span class="pill-dot {IN_PROGRESS_STATUSES.has(status) ? 'beacon-live' : ''}"></span>
  {status.replace(/_/g, " ")}
</span>
