<script lang="ts">
  import SkeletonLine from "./SkeletonLine.svelte";
</script>

<div class="card skeleton-card project-card">
  <span class="skeleton-line" style="height: 96px; border-radius: 0; display: block"></span>
  <div class="project-card-body">
    <SkeletonLine width="46%" height={16} />
    <SkeletonLine width="80%" height={12} />
    <SkeletonLine width="38%" height={12} />
  </div>
</div>
