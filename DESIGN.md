---
name: Mangrove
description: A self-hosted PaaS dashboard styled as a field station's instrument room monitoring a living system.
colors:
  bg: "#14100a"
  bg-elevated: "#1c160e"
  bg-card: "#201911"
  bg-inset: "#0f0b07"
  bg-recorder: "#0c0906"
  border: "#362a1a"
  border-hover: "#4d3c25"
  border-strong: "#5e4a2d"
  text: "#ece3d2"
  text-dim: "#a99b83"
  text-faint: "#6f6250"
  brass: "#caa057"
  brass-bright: "#e2bd76"
  brass-dim: "#7a6236"
  verdigris: "#55a087"
  verdigris-bright: "#6dbb9e"
  red: "#cc5c53"
  red-bright: "#e0776c"
  ochre: "#c98a3d"
  ochre-bright: "#dca158"
typography:
  display:
    fontFamily: "Space Grotesk, -apple-system, BlinkMacSystemFont, Segoe UI, Helvetica, Arial, sans-serif"
    fontSize: "22px"
    fontWeight: 600
    lineHeight: 1.2
    letterSpacing: "-0.02em"
  body:
    fontFamily: "Space Grotesk, -apple-system, BlinkMacSystemFont, Segoe UI, Helvetica, Arial, sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.5
    letterSpacing: "-0.006em"
  label:
    fontFamily: "Space Grotesk, sans-serif"
    fontSize: "12px"
    fontWeight: 600
    letterSpacing: "0.06em"
  mono:
    fontFamily: "IBM Plex Mono, SF Mono, ui-monospace, Menlo, Consolas, monospace"
    fontSize: "12.5px"
    fontWeight: 400
rounded:
  sm: "5px"
  md: "8px"
  lg: "12px"
  full: "999px"
spacing:
  1: "0.25rem"
  2: "0.5rem"
  3: "0.75rem"
  4: "1rem"
  5: "1.25rem"
  6: "1.5rem"
  8: "2rem"
components:
  button-primary:
    backgroundColor: "{colors.brass}"
    textColor: "#211705"
    rounded: "{rounded.sm}"
    padding: "8px 14px"
  button-primary-hover:
    backgroundColor: "{colors.brass-bright}"
  button-danger:
    backgroundColor: "transparent"
    textColor: "{colors.red-bright}"
    rounded: "{rounded.sm}"
    padding: "8px 14px"
  card:
    backgroundColor: "{colors.bg-card}"
    rounded: "{rounded.md}"
    padding: "20px"
  pill:
    backgroundColor: "{colors.verdigris}"
    rounded: "{rounded.full}"
    padding: "3px 9px"
---

# Design System: Mangrove

## Overview

**Creative North Star: "Field Station / Instrument Room"**

Mangrove's dashboard reads as the instrument room of a field station monitoring a living system: an operator's own infrastructure, glanced at through gauges, ledgers, and beacons rather than through a generic SaaS admin shell. The system was chosen through Impeccable's forced concept-seed roll specifically to refuse the category default for self-hosted ops tools — a navy-blue-on-near-black panel with a stock blue accent, cards-with-icons, and a kicker over every heading. Nothing here pretends to be a *physical* instrument (no fake bevels, no stamped-metal CSS, no faux-brass gradients standing in for the real material) — the instrument-room feeling comes from vocabulary and structure: gauge dials for resource readings, ruled-ledger tables for what's deployed, engraved-plaque uppercase labels for section titles, and a strip-chart trace for live logs, all rendered as flat, legible, digital instrumentation.

The palette is warm and near-black rather than blue-black — "instrument room at night," lit by one brass accent rather than a cool screen-glow blue. Two audiences (a technical operator's full dashboard, and a plain-language "Simple mode" for someone who just wants an app running) share the exact same system at two different densities, never two different skins.

**Key Characteristics:**
- Warm near-black grounds, never navy or pure black.
- One interactive accent color (brass), used sparingly, not scattered.
- Real gauges, real ledgers, real beacons — always paired with the actual number/word they represent, never standing in for it.
- No card-in-a-card nesting; a sub-panel living inside a card is visually distinct (recessed inset, not a second bordered box).
- The workspace/"station" switcher is pinned ambient context at the top of the sidebar, not a page you navigate away to.

## Colors

Warm, low-saturation, instrument-panel colors on a near-black ground — nothing in the palette is a pure hue; every color is slightly toasted, as if lit by a warm bulb rather than a cool monitor backlight.

### Primary
- **Brass** (`#caa057`, bright state `#e2bd76`): the one interactive accent — primary buttons, the active nav item, the station switcher's icon and active state, focus rings, chart-record ticks, gauge fill for values in the normal range. Used on interactive elements only, never as a large background field.

### Neutral
- **Instrument Black** (`#14100a`): the page ground.
- **Panel** (`#1c160e`): the sidebar/elevated surface.
- **Card** (`#201911`): card backgrounds.
- **Inset** (`#0f0b07`): recessed surfaces set *into* a card — form inputs, gauge sockets, the log/terminal recorder body.
- **Recorder Black** (`#0c0906`): the darkest surface, reserved for the log viewer and terminal, evoking a sealed instrument bay.
- **Parchment** (`#ece3d2`): primary text.
- **Dim Parchment** (`#a99b83`): secondary text, table body copy.
- **Faint Parchment** (`#6f6250`): placeholder text, disabled/faint labels.
- **Border** (`#362a1a`, hover `#4d3c25`, strong `#5e4a2d`): all hairline dividers and default borders.

### Semantic
- **Verdigris** (`#55a087`, bright `#6dbb9e`): success/healthy/running — an oxidized-copper green, deliberately not a generic "success green," chosen to echo the world's material vocabulary (copper, brass, aged metal) rather than a stock traffic-light hue.
- **Sealing-Wax Red** (`#cc5c53`, bright `#e0776c`): failed/danger/error.
- **Ochre** (`#c98a3d`, bright `#dca158`): building/pending/in-progress/warning — visually distinct from brass (more orange, less yellow) so an in-progress beacon never reads as a call-to-action.

### Named Rules
**The One Accent Rule.** Brass is the only color used for interactive/primary emphasis. Verdigris, red, and ochre are exclusively status semantics (health/success/danger/in-progress) and are never repurposed as a second "brand" accent.

### Instrument Accent Themes
Settings > Theme (`web/src/theme.tsx`) lets an operator swap which single color plays the accent role — Brass (default), Copper, Iron, Silver, or Indigo — persisted to `localStorage` and applied by overriding just `--brass`/`--brass-bright`/`--brass-dim`/`--brass-wash`/`--brass-wash-strong` per `[data-theme="..."]` in `styles.css`. Surfaces (`--bg`/`--border`/`--text` tokens) and the semantic colors (verdigris/red/ochre) stay fixed across every theme. This doesn't loosen the One Accent Rule — it still holds at every instant a theme is active, since switching themes replaces which single accent is live rather than adding a second one alongside brass. The station/workspace mark (`MangroveIcon` in `src/icons.tsx`) and the browser-tab favicon both read the active accent (via `currentColor`/`var(--brass-dim)`, and a regenerated data: URI respectively), so they re-color with the chosen theme automatically.

## Typography

**Display/UI Font:** Space Grotesk (self-hosted via `@fontsource`, latin + latin-ext subsets; system sans fallback stack)
**Mono Font:** IBM Plex Mono (self-hosted via `@fontsource`, same subsets)

**Character:** Space Grotesk's slightly geometric, faintly technical letterforms read as engineering/drafting lettering without tipping into a display-serif or a coded "hacker" mono-everywhere costume. IBM Plex Mono carries every value that is actually data — slugs, ports, timestamps, log lines, env vars, commit SHAs — so monospace always signals "this is a real measured value," never "this page is technical" as decoration.

### Hierarchy
- **Display** (600, 22px, -0.02em): page `<h1>` titles (`.page-header h1`).
- **Title** (600, 16.5px, -0.015em): modal titles, card section headers rendered as body-weight text.
- **Label** (600, 11–12px, 0.05–0.09em, uppercase): `.card-title`, table `<th>`, `.nav-section-label`, `.station-switcher-eyebrow` — the "engraved plaque" voice.
- **Body** (400, 13.5–14px, -0.006em): running copy, table cells, form labels' companion text.
- **Mono/Readout** (400–600, 11–13px, tabular-nums): every numeric or log/code value, via `.mono`, `.stat-value`, `.gauge-readout .n`, `.pill` text, `.kv-key`/`.kv-value`.

### Named Rules
**The Mono-Means-Measured Rule.** Monospace is reserved for values that are actually data (slugs, ports, hashes, timestamps, log/terminal output, numeric readouts). It is never applied to prose or labels purely to look "technical."

## Layout

Sidebar (240px, fixed) + fluid main content (`max-width: 1120px`, `padding: 32px 40px`). The sidebar is a two-part flex column: a `sidebar-topbar` (brand mark + a mobile-only menu toggle) and a `sidebar-body` (station switcher, primary nav, mode toggle, user, settings, log out) — on screens ≤760px the body collapses behind the toggle into an off-canvas drawer rather than wrapping into an unreadable row of nav links.

Cards stack with a consistent 16px gap. A card whose only content is a table (`.card:has(> table)`) loses its own padding and gains a faint vertical rule near the left edge (`background-image` gradient at 40–41px) so the table itself reads as a ruled ledger page, plus `overflow-x: auto` so a table wider than its card (an admin panel with several data columns and a row-action button) scrolls instead of clipping the last column.

A grid of readouts that would otherwise repeat as identical bordered tiles (resource gauges, in particular) is instead laid out as `.instrument-row`/`.instrument-cell`: one shared panel, cells divided by a hairline `border-right` (stacking to `border-bottom` on mobile), never N separate same-size cards — this is a deliberate refusal of the generic "icon + number + label card grid" admin-panel default.

### Named Rules
**The One Panel Rule.** A row of related instrument readings (CPU/memory/disk/load, memory budget, etc.) lives in one shared panel divided by hairlines, never as a repeated grid of identically-bordered tiles.

## Elevation & Depth

Mostly flat. Cards and inputs sit on a single hairline border (`--border`), not a shadow — this is a screen full of instruments set into one panel, not a stack of floating material cards. Depth exists only where something is genuinely lifted off the surface behind it: modals (`--shadow-lg`, real offset + blur, never a flat colored halo) and the station-switcher dropdown panel (`--shadow-lg`). Hover/active feedback on buttons and clickable cards is a border-color and background shift, not a shadow.

### Shadow Vocabulary
- **sm** (`0 1px 2px rgba(0,0,0,0.4)`): minor separation, rarely used directly.
- **md** (`0 10px 28px rgba(0,0,0,0.45)`): hover state on a clickable card.
- **lg** (`0 28px 72px rgba(0,0,0,0.55)`): modals, the station-switcher dropdown — anything genuinely floating above the page.

### Named Rules
**The Flat-Panel Rule.** Nothing on the page is a floating card by default. Elevation is reserved for content that is genuinely temporary/overlaid (modals, dropdowns).

## Shapes

Small, consistent corner radii throughout: 5px (`sm` — buttons, inputs, pills-as-rectangles), 8px (`md` — cards, the log/terminal frame), 12px (`lg` — modals), and a full pill radius for status beacons and the mode toggle. Nothing sharp-cornered, nothing heavily rounded — the radius scale itself reads as machined rather than soft. Icons (see `src/icons.tsx`) are a single authored line-icon set at 1.7px stroke weight, round caps/joins, drawn in the world's own grammar (a compass rose, a ruled ledger, a half-circle gauge, a stacked-drawer cabinet) rather than a generic icon-font import.

## Components

### Buttons
- **Shape:** 5px radius, 8px 14px padding (`.btn-sm`: 5px 10px).
- **Primary:** solid brass fill, near-black text (`#211705`) for contrast, 600 weight — the only solid-fill button on the page, reserved for the one primary action per view.
- **Danger:** transparent fill, red-bright text, red border on hover/wash background — never solid red at rest.
- **Default/Secondary:** `bg-elevated` fill, border, text color; border brightens on hover.
- **Hover/Active:** border-color shift + subtle background shift; `transform: scale(0.97)` on press (skipped entirely under `prefers-reduced-motion`).

### Status Beacons (`.pill`)
- **Style:** full-radius pill, a small solid dot + uppercase mono label, background at ~14% tint of the semantic color.
- **In-progress state:** the dot gets a `beacon-live` pulsing ring (`box-shadow`-free, pure `border` + `opacity`/`scale` keyframe) — the one authored motion moment for status, exempted from `prefers-reduced-motion` the same way a spinner is, since it is the only signal of ongoing activity.

### Gauges (`src/components/Gauge.tsx`)
- **Style:** a 250°-sweep SVG arc (track + value stroke) with quarter-turn tick marks and a centered numeric readout — never a full circle (which would read as a generic "progress ring"), and always rendered beside its actual value/label text, never replacing it.
- **Tone:** brass at rest, ochre >70%, red >90%.

### Cards / Containers
- **Corner:** 8px radius.
- **Background:** `bg-card` on `bg`; a nested "instrument socket" (`.gauge-tile`-style recess, now folded into `.instrument-cell`) uses `bg-inset` instead of stacking another bordered card.
- **Border:** 1px `--border`, no shadow at rest.
- **Padding:** 20px (0 when the card's sole content is a table).

### Inputs / Fields
- **Style:** `bg-inset` fill, 1px border, 5px radius, 8px 11px padding.
- **Focus:** border shifts to brass; global `:focus-visible` also gets a 2px brass outline with offset, for keyboard navigation everywhere (not just inputs).

### Navigation
- **Sidebar nav-link:** icon + label, 8px radius, brass-wash background + brass-bright text when active.
- **Station switcher:** pinned above the nav list — eyebrow label ("Station") + current workspace name + chevron, opening a dropdown listing every workspace with its project count, a "Manage workspaces" exit, and re-affirming the active selection with a filled brass dot. This is the direct structural fix for workspace management previously living as an orphaned, disconnected nav item.
- **Mobile:** the whole nav collapses behind a top-bar hamburger toggle into a full-width drawer, rather than wrapping into a multi-row jumble.

### Log Viewer / Strip-Chart (`src/components/LogViewer.tsx`)
A signature component: live logs render inside a `log-viewer-frame` with a `log-strip` above the text — one thin vertical tick per received line (height derived from that line's length, a taller red tick when the line matches an error/fail/panic pattern), scrolling right-aligned like a seismograph drum. This is a real reading of the actual stream, not decorative — the amplitude and color both come from real log content.

## Do's and Don'ts

### Do:
- **Do** pair every gauge/beacon with its real numeric or word value in text — the visualization augments the content, it never replaces it.
- **Do** use `.instrument-row`/`.instrument-cell` (a divided panel) for any new row of related readouts, not a grid of separate bordered tiles.
- **Do** keep the station/workspace switcher as ambient, always-visible sidebar context — never move workspace management back to a flat top-level nav item.
- **Do** use the authored icon set in `src/icons.tsx` for any new icon; extend it in the same 1.7px stroke/round-cap grammar rather than importing an icon font.
- **Do** reserve monospace for genuinely measured/logged values.

### Don't:
- **Don't** introduce a second "brand" accent color alongside brass — new emphasis needs are a brass variant (wash/dim/bright), not a new hue.
- **Don't** fake physical material (embossing, stamped-metal gradients, skeuomorphic bevels) — the instrument-room feeling comes from vocabulary and structure, not costume CSS.
- **Don't** add a kicker/eyebrow label above a page or card heading purely for decoration (the `station-switcher-eyebrow` and table `<th>` labels are functional value-labels for a live control or column, not decorative kickers, and that distinction should hold for anything new).
- **Don't** let a table's row-action button be the thing that determines whether the layout overflows — give a data-dense table its own full-width row rather than fighting it into half a `grid-2`.
