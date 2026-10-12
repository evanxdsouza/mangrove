<script lang="ts">
  import SkeletonLine from "./SkeletonLine.svelte";
</script>

<div class="card skeleton-card deployment-card">
  <div class="deployment-card-top">
    <SkeletonLine width={34} height={34} />
    <SkeletonLine width="50%" height={14} />
  </div>
  <SkeletonLine width="40%" height={11} />
  <div style="margin-top: 10px"><SkeletonLine width="60%" height={11} /></div>
</div>
