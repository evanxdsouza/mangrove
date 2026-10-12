<script lang="ts">
  import { appearanceState } from "../stores/theme.svelte";
  import { tideLevel, type TideLevel } from "../tide";

  // The mangrove-at-the-waterline banner art behind a project card, the
  // project header, the login card, and the first-run empty state. Art, not
  // data -- but it follows the same tide level as the project's gauge, so a
  // storm banner always sits over a dry gauge. In the dark appearance the
  // scenes turn to night: the sun becomes a moon and stars come out, while
  // "turning" (building) stays a warm dusk so it still stands apart.
  let { status, level, className }: { status?: string | null; level?: TideLevel; className?: string } = $props();

  interface Scene {
    sky: [string, string, number];
    water: [string, string];
    waterTop: number;
    ridge: [string, number];
    halo: { color: string; strength: number; r: number; cy: number; opacity: number };
    orb?: { cy: number; r: number; fill: string; opacity?: number };
    glints?: { fill: string; opacity: number; rows: 2 | 3 };
    ripples?: { stroke: string; opacity: number; puddle: string };
    stars: number;
    roots: string;
    rootsOpacity: number;
    vignette: [string, number];
  }

  const SUN = "#FFE9B8";
  const DAY: Record<TideLevel, Scene> = {
    high: {
      sky: ["#2E6B66", "#E7B77A", 0.75], water: ["#3C7F78", "#15423A"], waterTop: 92, ridge: ["#2B6B5F", 0.5],
      halo: { color: SUN, strength: 0.9, r: 60, cy: 76, opacity: 1 }, orb: { cy: 80, r: 13, fill: "#FFF1CC" },
      glints: { fill: "#FFF1CC", opacity: 0.5, rows: 3 }, stars: 0, roots: "#0F2B24", rootsOpacity: 1, vignette: ["#0A1F1A", 0.38],
    },
    mid: {
      sky: ["#5C7FA3", "#F2C98B", 0.8], water: ["#6A93A8", "#2A4F5E"], waterTop: 94, ridge: ["#3E6A6A", 0.5],
      halo: { color: SUN, strength: 0.9, r: 70, cy: 90, opacity: 1 }, orb: { cy: 94, r: 14, fill: "#FFF1CC" },
      glints: { fill: "#FFF1CC", opacity: 0.55, rows: 3 }, stars: 0, roots: "#0F2B24", rootsOpacity: 1, vignette: ["#0A1F1A", 0.38],
    },
    low: {
      sky: ["#A9BDB3", "#E4DCC6", 0.8], water: ["#B7A98A", "#7A6C52"], waterTop: 94, ridge: ["#8FA89B", 0.5],
      halo: { color: SUN, strength: 0.9, r: 46, cy: 70, opacity: 0.5 },
      ripples: { stroke: "#6A5E46", opacity: 0.5, puddle: "#9FB6AE" }, stars: 0, roots: "#0F2B24", rootsOpacity: 0.9, vignette: ["#0A1F1A", 0.38],
    },
    dry: {
      sky: ["#3A3F46", "#B8735F", 0.8], water: ["#4A4C52", "#23252A"], waterTop: 94, ridge: ["#2C2F34", 0.6],
      halo: { color: SUN, strength: 0.9, r: 60, cy: 82, opacity: 0.35 }, orb: { cy: 86, r: 11, fill: "#E7A38B" },
      glints: { fill: "#E7A38B", opacity: 0.35, rows: 3 }, stars: 0, roots: "#0F2B24", rootsOpacity: 1, vignette: ["#0A1F1A", 0.38],
    },
  };

  const MOON = "#BFE3DC";
  const DUSK = "#F2C98B";
  const NIGHT: Record<TideLevel, Scene> = {
    high: {
      sky: ["#06202A", "#1F4A4A", 0.75], water: ["#1E5B58", "#061D1A"], waterTop: 92, ridge: ["#0E3A38", 0.7],
      halo: { color: MOON, strength: 0.55, r: 60, cy: 62, opacity: 1 }, orb: { cy: 66, r: 12, fill: "#EAF2E1" },
      glints: { fill: MOON, opacity: 0.4, rows: 3 }, stars: 1, roots: "#04100C", rootsOpacity: 1, vignette: ["#000000", 0.5],
    },
    mid: {
      sky: ["#182640", "#8A6340", 0.8], water: ["#3B6378", "#0B1D2A"], waterTop: 94, ridge: ["#1F3B4A", 0.7],
      halo: { color: DUSK, strength: 0.6, r: 66, cy: 92, opacity: 1 }, orb: { cy: 94, r: 13, fill: DUSK },
      glints: { fill: DUSK, opacity: 0.5, rows: 2 }, stars: 0.6, roots: "#04100C", rootsOpacity: 1, vignette: ["#000000", 0.5],
    },
    low: {
      sky: ["#10181E", "#2B3A3A", 0.8], water: ["#3A4A44", "#151D1A"], waterTop: 94, ridge: ["#2A3A36", 0.7],
      halo: { color: MOON, strength: 0.55, r: 40, cy: 70, opacity: 0.35 }, orb: { cy: 72, r: 9, fill: "#C9D6CF", opacity: 0.55 },
      ripples: { stroke: "#5E6E64", opacity: 0.45, puddle: "#3F524A" }, stars: 0.5, roots: "#04100C", rootsOpacity: 1, vignette: ["#000000", 0.5],
    },
    dry: {
      sky: ["#101216", "#4A2A28", 0.8], water: ["#2B2E34", "#0A0B0E"], waterTop: 94, ridge: ["#1A1C20", 0.8],
      halo: { color: DUSK, strength: 0.6, r: 50, cy: 84, opacity: 0.3 }, orb: { cy: 86, r: 10, fill: "#D98A74" },
      glints: { fill: "#D98A74", opacity: 0.35, rows: 2 }, stars: 0, roots: "#04100C", rootsOpacity: 1, vignette: ["#000000", 0.5],
    },
  };

  const STARS: [number, number, number, number][] = [
    [120, 22, 1.2, 0.8], [210, 40, 1, 0.6], [300, 16, 1.4, 0.85], [380, 34, 1, 0.55],
    [560, 20, 1.3, 0.8], [640, 42, 1, 0.6], [720, 18, 1.2, 0.75], [60, 46, 1, 0.5],
  ];

  const uid = $props.id();
  const id = uid.replace(/[^a-zA-Z0-9]/g, "");
  const lv = $derived(level ?? tideLevel(status));
  const s = $derived((appearanceState.scheme === "dark" ? NIGHT : DAY)[lv]);
</script>

<svg class="tide-banner {className ?? ''}" viewBox="0 0 800 132" preserveAspectRatio="xMidYMid slice" aria-hidden="true">
  <defs>
    <linearGradient id="{id}s" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0" stop-color={s.sky[0]} />
      <stop offset={s.sky[2]} stop-color={s.sky[1]} />
    </linearGradient>
    <linearGradient id="{id}w" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0" stop-color={s.water[0]} />
      <stop offset="1" stop-color={s.water[1]} />
    </linearGradient>
    <radialGradient id="{id}h">
      <stop offset="0" stop-color={s.halo.color} stop-opacity={s.halo.strength} />
      <stop offset="1" stop-color={s.halo.color} stop-opacity="0" />
    </radialGradient>
    <linearGradient id="{id}v" x1="0" y1="0" x2="1" y2="0">
      <stop offset="0" stop-color={s.vignette[0]} stop-opacity={s.vignette[1]} />
      <stop offset=".25" stop-color={s.vignette[0]} stop-opacity="0" />
      <stop offset=".75" stop-color={s.vignette[0]} stop-opacity="0" />
      <stop offset="1" stop-color={s.vignette[0]} stop-opacity={s.vignette[1]} />
    </linearGradient>
  </defs>
  <rect width="800" height="132" fill="url(#{id}s)" />
  {#if s.stars > 0}
    <g fill="#EAF2E1" opacity={s.stars}>
      {#each STARS as [cx, cy, r, o], i (i)}<circle {cx} {cy} {r} opacity={o} />{/each}
    </g>
  {/if}
  <circle cx="470" cy={s.halo.cy} r={s.halo.r} fill="url(#{id}h)" opacity={s.halo.opacity} />
  {#if s.orb}<circle cx="470" cy={s.orb.cy} r={s.orb.r} fill={s.orb.fill} opacity={s.orb.opacity ?? 1} />{/if}
  <path d="M0 88Q90 70 180 82T360 79T540 84T800 76V100H0z" fill={s.ridge[0]} opacity={s.ridge[1]} />
  <rect y={s.waterTop} width="800" height={132 - s.waterTop} fill="url(#{id}w)" />
  {#if s.glints}
    <g fill={s.glints.fill} opacity={s.glints.opacity}>
      <rect x="448" y={s.waterTop + 6} width="44" height="2.5" rx="1" />
      <rect x="456" y={s.waterTop + 13} width="30" height="2.5" rx="1" />
      {#if s.glints.rows === 3}<rect x="462" y={s.waterTop + 20} width="18" height="2.5" rx="1" />{/if}
    </g>
  {/if}
  {#if s.ripples}
    <g fill="none" stroke={s.ripples.stroke} stroke-width="1.6" opacity={s.ripples.opacity}>
      <path d="M20 106q30-4 60 0t60 0" />
      <path d="M300 114q40-4 80 0t80 0" />
      <path d="M560 104q30-4 60 0t60 0" />
    </g>
    <ellipse cx="420" cy="116" rx="60" ry="5" fill={s.ripples.puddle} opacity=".8" />
  {/if}
  <g fill={s.roots} opacity={s.rootsOpacity}>
    <ellipse cx="86" cy="60" rx="64" ry="24" opacity=".92" />
    <ellipse cx="140" cy="68" rx="38" ry="16" opacity=".85" />
    <ellipse cx="716" cy="64" rx="70" ry="24" opacity=".92" />
    <ellipse cx="660" cy="72" rx="34" ry="14" opacity=".85" />
    <g fill="none" stroke={s.roots} stroke-width="4" stroke-linecap="round">
      <path d="M30 132C34 106 52 88 82 74" />
      <path d="M62 132C64 112 78 96 96 82" />
      <path d="M104 132C104 114 112 100 124 88" />
      <path d="M780 132C774 104 752 88 720 76" />
      <path d="M746 132C744 112 730 96 712 84" />
      <path d="M700 132C702 114 694 100 684 90" />
    </g>
  </g>
  <rect width="800" height="132" fill="url(#{id}v)" />
</svg>
