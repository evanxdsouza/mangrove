<script lang="ts">
  import SkeletonLine from "./SkeletonLine.svelte";
</script>

<div class="card skeleton-card project-list-row">
  <div class="project-list-row-main">
    <span class="project-list-row-chevron-spacer"></span>
    <SkeletonLine width={12} height={26} />
    <div class="project-list-row-heading" style="gap: 6px">
      <SkeletonLine width="30%" height={14} />
      <SkeletonLine width="20%" height={10} />
    </div>
    <SkeletonLine width={80} height={20} />
  </div>
</div>
