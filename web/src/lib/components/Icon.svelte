<script lang="ts">
  import type { SVGAttributes } from "svelte/elements";
  import { ICONS, type IconName } from "../icons";

  let { name, ...rest }: { name: IconName } & SVGAttributes<SVGSVGElement> = $props();
</script>

<svg
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  stroke-width="1.9"
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
  {...rest}
>
  {@html ICONS[name]}
</svg>
