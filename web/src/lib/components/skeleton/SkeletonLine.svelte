<script lang="ts">
  // Shimmering placeholder shapes shown while a page's first fetch is in
  // flight, so the real layout is visible instead of a spinner. The shimmer
  // is a plain CSS animation, stopped by the global reduced-motion rule.
  let { width = "100%", height = 12 }: { width?: string | number; height?: number } = $props();
</script>

<span class="skeleton-line" style:width={typeof width === "number" ? `${width}px` : width} style:height="{height}px"></span>
