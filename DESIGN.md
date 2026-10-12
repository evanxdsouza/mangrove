---
name: Mangrove
description: A self-hosted PaaS dashboard in the "Intertidal" language -- a light page, a deep-green floating sidebar, status told by a tide gauge.
source: https://claude.ai/artifact/XYbG54xxgtdKpmqneRdGxE (design canvas "Mangrove Intertidal: All Pages")
colors:
  bg: "#f2f1ec"
  bg-card: "#ffffff"
  bg-elevated: "#fbfbf8"
  bg-inset: "#e9e7df"
  bg-recorder: "#0f1b18"
  border: "#e4e2da"
  border-soft: "#efede6"
  text: "#1b2420"
  text-strong: "#44504a"
  text-dim: "#6b746e"
  side: "#173d31"
  leaf: "#1f5c47"
  tide-high: "#2f8f83"
  tide-mid: "#c98a2b"
  tide-low: "#8fa89b"
  red: "#b5472f"
typography:
  display:
    fontFamily: "Figtree, system-ui, sans-serif"
    fontSize: "28px"
    fontWeight: 700
    letterSpacing: "-0.02em"
  body:
    fontFamily: "Figtree, system-ui, sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.45
  label:
    fontFamily: "Figtree, sans-serif"
    fontSize: "12px"
    fontWeight: 600
  mono:
    fontFamily: "JetBrains Mono, ui-monospace, monospace"
    fontSize: "12.5px"
    fontWeight: 400
rounded:
  sm: "8px"
  md: "10px"
  lg: "14px"
  xl: "20px"
  full: "999px"
---

# Design System: Mangrove -- Intertidal

## Overview

One visual language for every page of the dashboard: **a light page, a
deep-green floating sidebar, status told by a tide gauge, banners that
follow deployment state, and a root line that shows what belongs to what.**

The source of truth is the design canvas linked in the frontmatter (17
boards: tokens and parts, setup/sign-in/404, shell states, Simple mode,
Projects grid/list/states, project tabs, deployment Overview/History/Logs/
Env, run detail, workspaces, admin, health/settings, storage, and every
dialog). `web/src/lib/styles.css` implements it as CSS custom properties; class
names predate the redesign and were kept so page code didn't churn.

## Colors

| Role | Token | Value |
|---|---|---|
| Page | `--bg` | `#F2F1EC` |
| Surface (cards, dialogs) | `--bg-card` | `#FFFFFF` |
| Field fill, hovered rows | `--bg-elevated` | `#FBFBF8` |
| Segmented-control track | `--bg-inset` | `#E9E7DF` |
| Line / hairline | `--border` / `--border-soft` | `#E4E2DA` / `#EFEDE6` |
| Ink / labels / muted | `--text` / `--text-strong` / `--text-dim` | `#1B2420` / `#44504A` / `#6B746E` |
| Sidebar | `--side` | `#173D31` |
| Accent (primary buttons, focus, checkboxes) | `--leaf` | `#1F5C47` |
| Terminal | `--bg-recorder` | `#0F1B18` |

**Tide (status) colors** -- the only colors that carry meaning:

| Level | Statuses | Gauge fill | Pill |
|---|---|---|---|
| High | running, success, healthy | `#2F8F83` | `#DDF1EE` / `#1D6B61` |
| Mid (working) | building, healthchecking, queued, timeout, rolled_back | `#C98A2B` | `#FFE5BC` / `#7A4E0E` |
| Low (at rest) | stopped, sleeping, pending (never deployed), unknown | `#8FA89B` | `#EFEDE6` / `#44504A` |
| Dry (needs you) | failed, unhealthy, error | red rim, no water | `#F8E1DB` / `#A23B2A` |

A project shows the **worst** level among its deployments (dry, then
working, then at rest, then running). Only building, healthchecking and
queued pulse.

### Instrument metal (Settings > Theme)

The `--brass*` family is swappable (Brass/Copper/Iron/Silver/Indigo). In
Intertidal it only tints the sidebar's small section labels and the station
eyebrow; the deep-green sidebar, leaf-green buttons, and tide colors never
change. The default (Brass) is `#8FB0A2`, matching the canvas.

### Dark appearance

"The same language after sunset." Settings > Appearance offers Light, Dark,
and System (the default, following `prefers-color-scheme` live). The choice
is stored per browser (`mangrove-appearance`) and always resolves to
`data-color-scheme="light|dark"` on `<html>`, set by `app.html` before first
paint so a reload never flashes the wrong scheme. It is independent of the
instrument metal (`data-theme`).

| Role | Dark | Light |
|---|---|---|
| Page / Surface / Raised | `#0E1512` / `#151F1B` / `#1B2823` | `#F2F1EC` / `#FFFFFF` / -- |
| Line / Input | `#263530` / `#0F1814` | `#E4E2DA` / `#FBFBF8` |
| Ink / Muted / Labels | `#E7EEE9` / `#8FA298` / `#B6C6BD` | `#1B2420` / `#6B746E` / `#44504A` |
| Sidebar / Accent | `#123227` / `#2A7A5F` | `#173D31` / `#1F5C47` |
| Tide high / mid / low / dry | `#3FB5A6` / `#E0A444` / `#6F8A7D` / `#E0694F` | see above |
| Terminal | `#070D0B`, with a hairline border | `#0F1B18` |

Cards lift by lightness, not shadow; only dialogs keep a heavy shadow, over a
60% black scrim. Status colours are ~15% brighter, with pills as opaque
tints of their status colour. Banners become night scenes: moon and stars
in place of the sun, while "turning" stays a warm dusk so it still stands
apart. Type, spacing, radii, layout, copy, and icons are unchanged.
Contrast was checked by the canvas (ink on page 15.7, muted on surface 6.3,
white on the accent button 5.2, every pill 6.4+).

## Typography

Figtree for everything, JetBrains Mono for slugs, ports, env keys, image
refs, and logs. Self-hosted via `@fontsource` (latin + latin-ext only), never
a CDN -- the dashboard ships inside the box it manages.

| Use | Spec |
|---|---|
| Page title | Figtree 700 · 28 · -0.02em |
| Dialog title | Figtree 700 · 20 |
| Card title | Figtree 700 · 15 (no uppercase eyebrow) |
| Body | Figtree 400 · 14 |
| Controls | Figtree 600 · 13 |
| Labels, hints | Figtree 600 · 12 / 400 · 12 muted |
| Terminal | JetBrains Mono 12 on `#0F1B18`, text `#BFD6CB` |

## Components

- **Tide gauge** (`web/src/lib/components/TideGauge.svelte`) -- the one status mark, used on
  every project card, deployment card, child tile, search result, activity
  row, and the deployment header (sizes sm 12×26, md 14×30, lg 22×48). The
  water line animates slowly only for in-progress statuses; a spring "pop"
  (`burstKey`) marks a fresh successful deploy.
- **Tide banner** (`TideBanner.svelte`) -- mangrove-at-the-waterline
  art over a project card, the project header, the login card, and the
  first-run empty state. Art, not data, but it follows the same level as the
  gauge: high tide, turning, low tide, storm. Day and night palettes of one
  scene, switched by the appearance.
- **Root line** -- an expanded project's deployments hang off one vertical
  line, each joined by a curved root (`.project-card-children`), ending in a
  dashed "+ Add a deployment" tile. The deploy-history timeline is the same
  line running vertically.
- **Resource tide bars** (`Gauge.svelte`) -- replace the old arc
  dial. Teal to 70%, amber above 70%, red above 90%; the number always sits
  beside the bar.
- **Sidebar** -- floating, 264px, 20px radius, `#173D31`. Station switcher
  ("Station  All workspaces"), ⌘K search, Workspace and Host sections, a
  Technical/Simple toggle, Collapse, and the user's email with Settings / Log
  out links. Collapsed: a 72px icon rail. Phone: a dark top bar plus a drawer
  over a scrim. The brand mark turns red (with a note) when host memory or
  disk exceeds 90% (owner only).
- **Buttons** -- 10px radius, Figtree 600 13. Primary: leaf fill. Default:
  white with a hairline. Danger: red text with a pink hairline; "Delete
  deployment" in the header is a borderless red ghost.
- **Segmented control** (`.tabs`, `.view-toggle`, `.mode-toggle`) -- a
  `#E9E7DF` track, the active segment white with a soft shadow. A tab's count
  reads "Deployments · 3", never tinted.
- **Chips** (`.filter-chip`) -- pills; the selected one is filled sidebar
  green.
- **Pills** (`.pill`) -- sentence case, 12px 600, a 6px dot.
- **Cards** -- white, 20px radius, 1px line, near-invisible shadow. Project
  cards put the banner on top (96px) and body below.
- **Breadcrumb** -- a white "‹ Projects" back pill, then plain crumbs.
- **Dialogs** -- 20px radius, 460px (760px for the large size), over a
  translucent sidebar-green scrim.
- **Toasts** -- sidebar-green, white text, the mark in pale green.
- **Empty states** -- say what the thing is, then how to start; no icon. The
  page-level empty state is bold 16px over muted 13px.
- **Skeletons** -- the real card outline with shimmering bars.
- **Terminal** -- logs, run output, and code blocks share the `#0F1B18`
  surface; the log viewer has a "STDOUT · RECORDING" header bar.

## Do's and Don'ts

**Do**
- Let the tide gauge be the status signal; pair it with a pill, never
  replace the pill's text.
- Keep color for status. Everything structural (counts, workspace pills,
  metadata) stays neutral.
- Use JetBrains Mono for anything a user might copy (slugs, ports, env,
  image refs, commands).
- Make every page work at 390px: the sidebar becomes a drawer, toolbars
  wrap, tables scroll inside their card.

**Don't**
- Don't introduce a new hue per project or per feature; banners vary by
  tide level only.
- Don't nest a bordered card inside a card; child deployments are tiles on
  the root line.
- Don't use emoji or an icon font; icons live in `web/src/lib/icons.ts`, rendered by `Icon.svelte` (1.9 stroke,
  round caps, 24px grid).
- Don't load fonts or scripts from a CDN.
