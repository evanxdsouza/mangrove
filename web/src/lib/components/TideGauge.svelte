<script lang="ts">
  import { createSpring, prefersReducedMotion } from "../spring";
  import { IN_PROGRESS_STATUSES } from "../status";
  import { tideLevel, type TideLevel } from "../tide";

  // The tide gauge -- Intertidal's one status mark, used on every project,
  // deployment, search result, and activity row. A small capsule whose
  // water line reads real state; a dry, red-rimmed gauge is the one that
  // needs you. In-progress statuses get a slow wave on the water line, and
  // a changed burstKey (e.g. a fresh successful deploy's history id) plays
  // a one-shot spring pop.
  let {
    status,
    size = "md",
    burstKey,
  }: { status: string | null | undefined; size?: "sm" | "md" | "lg"; burstKey?: number } = $props();

  const SIZES = { sm: [12, 26], md: [14, 30], lg: [22, 48] } as const;
  const WATER: Record<Exclude<TideLevel, "dry">, { y: number; fill: string }> = {
    high: { y: 8, fill: "var(--tide-high)" },
    mid: { y: 17, fill: "var(--tide-mid)" },
    low: { y: 25, fill: "var(--tide-low)" },
  };
  const LEVEL_LABEL: Record<TideLevel, string> = { high: "high", mid: "working", low: "at rest", dry: "needs attention" };

  const clipId = $props.id();
  const level = $derived(tideLevel(status));
  const dims = $derived(SIZES[size]);
  const live = $derived(status != null && IN_PROGRESS_STATUSES.has(status));
  const label = $derived(`Tide gauge: ${LEVEL_LABEL[level]}${status ? `, ${status.replace(/_/g, " ")}` : ""}`);

  let scale = $state(1);
  let lastKey: number | undefined = undefined;
  $effect(() => {
    const key = burstKey;
    if (key == null || key === lastKey || prefersReducedMotion()) {
      lastKey = key;
      return;
    }
    lastKey = key;
    const spring = createSpring(1.4, { damping: 0.5, response: 0.5, onUpdate: (v) => (scale = v) });
    spring.set(1);
    return () => spring.stop();
  });
</script>

<svg
  class="tide-gauge tide-gauge-{level} {live ? 'tide-gauge-live' : ''}"
  width={dims[0]}
  height={dims[1]}
  viewBox="0 0 14 30"
  role="img"
  aria-label={label}
  style:transform={scale !== 1 ? `scale(${scale})` : undefined}
>
  <defs>
    <clipPath id={clipId}><rect x="1" y="1" width="12" height="28" rx="6" /></clipPath>
  </defs>
  <rect x="1" y="1" width="12" height="28" rx="6" fill="var(--gauge-fill)" stroke={level === "dry" ? "var(--red)" : "var(--tide-rim)"} />
  {#if level === "dry"}
    <path d="M3 25h8" stroke="var(--red)" stroke-width="1.6" stroke-linecap="round" />
  {:else}
    <g clip-path="url(#{clipId})">
      <path class="tide-gauge-water" d="M-14 {WATER[level].y}q3.5-3 7 0t7 0t7 0t7 0t7 0V30H-14z" fill={WATER[level].fill} />
    </g>
  {/if}
</svg>
