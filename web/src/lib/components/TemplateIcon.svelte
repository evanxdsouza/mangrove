<script lang="ts">
  import type { SVGAttributes } from "svelte/elements";
  import { templateIconMarkup } from "../templateIcons";

  let { templateKey, category, ...rest }: { templateKey: string; category: string } & SVGAttributes<SVGSVGElement> = $props();
</script>

<svg
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  stroke-width="1.7"
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
  {...rest}
>
  {@html templateIconMarkup(templateKey, category)}
</svg>
