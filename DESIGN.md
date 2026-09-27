---
name: Mangrove
description: A self-hosted PaaS dashboard styled as a field station's instrument room monitoring a living mangrove system.
colors:
  bg: "#0a0f0b"
  bg-elevated: "#121a15"
  bg-card: "#1b2620"
  bg-inset: "#070a08"
  bg-recorder: "#050805"
  border: "#362a1a"
  border-hover: "#4d3c25"
  border-strong: "#5e4a2d"
  text: "#ece3d2"
  text-dim: "#a99b83"
  text-faint: "#6f6250"
  leaf: "#4fae42"
  leaf-bright: "#6fcf5c"
  leaf-dim: "#35692e"
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
    backgroundColor: "{colors.leaf}"
    textColor: "#0c1509"
    rounded: "{rounded.sm}"
    padding: "8px 14px"
  button-primary-hover:
    backgroundColor: "{colors.leaf-bright}"
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

**Creative North Star: "Field Station / Instrument Room," now with something alive in it**

Mangrove's dashboard reads as the instrument room of a field station monitoring a living system: an operator's own infrastructure, glanced at through gauges, ledgers, beacons, and living resource cards rather than through a generic SaaS admin shell. The system refuses the category default for self-hosted ops tools — a navy-blue-on-near-black panel with a stock blue accent, cards-with-icons, and a kicker over every heading. Nothing here pretends to be a *physical* instrument (no fake bevels, no stamped-metal CSS, no faux-brass gradients standing in for the real material) — the instrument-room feeling comes from vocabulary and structure: gauge dials for resource readings, ruled-ledger tables for what's deployed, engraved-plaque uppercase labels for section titles, and a strip-chart trace for live logs, all rendered as flat, legible, digital instrumentation.

This is a from-a-real-critique revision of that original system: the first version leaned entirely on brass-as-hero and read as low-contrast and funereal rather than alive, internal enum values leaked into copy, the same content type (raw failure output) got two different treatments in two different tabs, and project/deployment pages were mostly dead space below a few small cards. This pass keeps the concept — swamp/field-station, self-hosted, built by a teenager, not an enterprise SaaS vendor — but promotes a real, living leaf green to the accent that actually carries the brand, widens the gap between surfaces so the screen has real depth, and gives every resource (project, deployment) a face instead of a table row.

The ground is now a cooler "swamp water at night" near-black-green rather than a warm brown-black, lit by brass-trimmed instrumentation — a deliberate warm-metal-in-a-cool-swamp duality — with real separation between page, sidebar, card, and border. Two audiences (a technical operator's full dashboard, and a plain-language "Simple mode" for someone who just wants an app running) still share the exact same system at two different densities, never two different skins.

**Key Characteristics:**
- A cool, near-black "swamp at night" ground, never navy or pure black, with brass-tinted borders/dividers as the warm counterpoint.
- One living/primary accent (leaf green), used for every primary action and active state; brass is the secondary structural metal — see the Two-Accent Rule below.
- Real gauges, real ledgers, real beacons, real plant glyphs — always paired with the actual number/word/status they represent, never standing in for it.
- No card-in-a-card nesting; a sub-panel living inside a card is visually distinct (recessed inset, not a second bordered box).
- The workspace/"station" switcher is pinned ambient context at the top of the sidebar, not a page you navigate away to.
- A resource (project, deployment) is a card with an identity — a plant glyph, a name, a status — not a row in a table; a project rolls up its deployments' counts rather than making you drill in to see how many are healthy.

## Colors

Two families now carry the palette: a living leaf green (the one primary accent) and brass (the secondary structural metal), both sitting on a cool, low-saturation near-black ground.

### Primary
- **Leaf** (`#4fae42`, bright `#6fcf5c`, dim `#35692e`): the one living/primary accent — primary buttons, active nav/tab, the station mark's icon and active state, focus rings, the default (healthy-range) gauge fill, hover feedback on a clickable card. Deliberately hued toward grass/lime rather than verdigris's teal-green, so a status beacon sitting right next to it never blends in. Used on interactive/primary elements only, never as a large background field — the one exception is `.project-hero-motif` (`ProjectDetailPage`'s decorative banner watermark), a large glyph at 16% opacity that carries no status of its own and is explicitly art, not data, per the Do/Don't list below.

### Secondary
- **Brass** (`#caa057`, bright `#e2bd76`, dim `#7a6236`): the secondary metallic accent — borders, dividers, `.card-title` "engraved plaque" labels, page-identifier icons, the log/resource strip-chart trace, texture. Never used for a primary call-to-action; if something needs a second interactive emphasis, it's a leaf variant (wash/dim/bright), not brass promoted back to primary. Settings > Theme still lets an operator swap which metal plays this role (Copper/Iron/Silver/Indigo) — see Instrument Metal Themes below.

### Neutral
- **Swamp Black** (`#0a0f0b`): the page ground — cooler and darker than the sidebar/card surfaces above it.
- **Panel** (`#121a15`): the sidebar/elevated surface.
- **Card** (`#1b2620`): card backgrounds — a clear, deliberate step up from the sidebar, not a few percent of lightness apart.
- **Inset** (`#070a08`): recessed surfaces set *into* a card — form inputs, gauge sockets, the log/terminal recorder body — darker than the page ground itself.
- **Recorder Black** (`#050805`): the darkest surface, reserved for the log viewer, terminal, and `CodeBlock`, evoking a sealed instrument bay.
- **Parchment** (`#ece3d2`): primary text.
- **Dim Parchment** (`#a99b83`): secondary text, table body copy.
- **Faint Parchment** (`#6f6250`): placeholder text, disabled/faint labels.
- **Border** (`#362a1a`, hover `#4d3c25`, strong `#5e4a2d`): all hairline dividers and default borders — kept warm/brass-tinted against the now-cooler ground on purpose.

### Semantic
- **Verdigris** (`#55a087`, bright `#6dbb9e`): success/healthy/running — an oxidized-copper teal-green, deliberately hued away from both brass *and* the new leaf primary so a status pill always reads as its own signal rather than blending into either accent.
- **Sealing-Wax Red** (`#cc5c53`, bright `#e0776c`): failed/danger/error.
- **Ochre** (`#c98a3d`, bright `#dca158`): building/pending/in-progress/warning — visually distinct from both brass and leaf so an in-progress beacon never reads as a call-to-action.

### Named Rules
**The Two-Accent Rule.** Leaf is the one living/primary accent — reserved for primary actions and active/selected state, used consistently everywhere across the app. Brass is the one secondary/structural accent — chrome, borders, plaque labels, never a call-to-action. Verdigris, red, and ochre are exclusively status semantics and are never repurposed as either accent. This replaces the original single-accent system's "One Accent Rule": there are now exactly two accent roles, each with exactly one color at a time, never three, and never the same color playing both roles.

### Instrument Metal Themes
Settings > "Instrument metal" (`web/src/theme.tsx`) lets an operator swap which single metal plays the *secondary* role — Brass (default), Copper, Iron, Silver, or Indigo — persisted to `localStorage` and applied by overriding just `--brass`/`--brass-bright`/`--brass-dim`/`--brass-wash`/`--brass-wash-strong` per `[data-theme="..."]` in `styles.css`. The living `--leaf*` primary and the semantic colors (verdigris/red/ochre) stay fixed across every theme — switching metals never touches the primary accent. This keeps the Two-Accent Rule true at every instant: exactly one metal is ever live in the secondary role, and leaf never stops being the primary one. The theme-swatch preview itself (`.theme-swatch`) is the one place a swatch's own color is read as a scoped inline `--brass`, not the globally active theme, so each swatch always previews in its own metal.

## Typography

**Display/UI Font:** Space Grotesk (self-hosted via `@fontsource`, latin + latin-ext subsets; system sans fallback stack)
**Mono Font:** IBM Plex Mono (self-hosted via `@fontsource`, same subsets)

**Character:** Space Grotesk's slightly geometric, faintly technical letterforms read as engineering/drafting lettering without tipping into a display-serif or a coded "hacker" mono-everywhere costume. IBM Plex Mono carries every value that is actually data — slugs, ports, timestamps, log lines, env vars, commit SHAs — so monospace always signals "this is a real measured value," never "this page is technical" as decoration.

### Hierarchy
- **Display** (600, 22px, -0.02em): page `<h1>` titles (`.page-header h1`).
- **Title** (600, 16.5px, -0.015em): modal titles, card section headers rendered as body-weight text.
- **Label** (600, 11–12px, 0.05–0.09em, uppercase): `.card-title`, table `<th>`, `.nav-section-label`, `.station-switcher-eyebrow` — the "engraved plaque" voice, always brass, never the primary leaf accent.
- **Body** (400, 13.5–14px, -0.006em): running copy, table cells, form labels' companion text.
- **Mono/Readout** (400–600, 11–13px, tabular-nums): every numeric or log/code value, via `.mono`, `.stat-value`, `.gauge-readout .n`, `.pill` text, `.kv-key`/`.kv-value`.

### Named Rules
**The Mono-Means-Measured Rule.** Monospace is reserved for values that are actually data (slugs, ports, hashes, timestamps, log/terminal output, numeric readouts). It is never applied to prose or labels purely to look "technical."

## Copy

**The swamp voice.** A handful of secondary-surface strings (loading, empty states) speak in the world's own voice instead of generic system text — `web/src/lib/copy.ts` is the one place these live (`"Still growing roots…"` for loading, `"Nothing planted here yet"` for an empty project list, etc.), used via the shared `CenterLoading` component and inline in empty-state copy. This is deliberately confined to secondary surfaces: error banners, failure detail, and anything an operator needs to act on fast stay in plain, direct language — see the Do/Don't list.

**Internal values never leak verbatim.** `web/src/lib/format.ts`'s `humanLabel()` maps a backend enum/DB value (port allocation type, audit-log action/resource type) to a real label, falling back to a generic snake_case→Title Case transform for anything not explicitly listed, so a future value never regresses to raw `snake_case` in the UI. `fmtWhen()`/`fmtWhenFull()` are the one date formatter used everywhere a timestamp renders — no inline `.toLocaleString()` variant per file.

## Layout

Sidebar (240px, fixed) + fluid main content (`max-width: 1120px`, `padding: 32px 40px`). The sidebar is a two-part flex column: a `sidebar-topbar` (brand mark + a mobile-only menu toggle) and a `sidebar-body` (station switcher, primary nav, mode toggle, user, settings, log out) — on screens ≤760px the body collapses behind the toggle into an off-canvas drawer rather than wrapping into an unreadable row of nav links.

Cards stack with a consistent 16px gap. A card whose only content is a table (`.card:has(> table)`) loses its own padding and gains a faint vertical rule near the left edge (`background-image` gradient at 40–41px) so the table itself reads as a ruled ledger page, plus `overflow-x: auto` so a table wider than its card scrolls instead of clipping the last column.

A grid of readouts that would otherwise repeat as identical bordered tiles (resource gauges, in particular) is instead laid out as `.instrument-row`/`.instrument-cell`: one shared panel, cells divided by a hairline `border-right` (stacking to `border-bottom` on mobile), never N separate same-size cards.

**Resource cards, not tables.** Projects (`.project-grid`/`.project-card`) and a project's own deployments (`.deployment-grid`/`.deployment-card`) render as a card grid (`auto-fill, minmax(260px, 1fr)`), not a ledger table — each card carries a `PlantGlyph` (status-reflecting glyph), name, slug, status pill, and a rollup line (`"2 of 3 deployments running"` at the project level). `WorkspacesPage` stays a table — a workspace is an admin-list concept, not a living resource, and forcing every list into a card grid regardless of what it lists is exactly the kind of uniform-default this system otherwise refuses.

**Project/group page.** `ProjectDetailPage` leads with `.project-hero`: a full-width decorative band (a subtle `bg-elevated`→`bg-card` gradient with a large, low-opacity `MangroveIcon` watermark bleeding off the right edge) behind the project name, description, and primary actions — art, not data, so it's exempt from the neutral-card-background rule below. Directly under it, an `.instrument-row` rollup (deployment count, running count, staging/preview count, all plain text) — never colored, since these are counts, not live thresholds. A `.card-title` section label ("Deployments") introduces the card grid, so the page never opens onto dead space below a couple of small cards, and GitHub config / Workspace settings follow as their own neutral panels.

**Resource detail page.** `DeploymentDetailPage` leads with a header (plant glyph, editable name, status pill, right-aligned actions — one primary). Directly below and above the tab bar, `ServiceStatRow` renders a plain `.instrument-row` stat strip (Image / Internal port / Resources / Replicas / Host port / Health) for the deployment's one primary service — no card, no per-number border, just hairline dividers (`.deployment-stat-row`). This only fires for the common single-service case; a compose deployment has no one canonical service, so it falls back to per-service `ServiceCard`s inside the Overview tab instead, and the two never render the same service's vitals twice (`useLatestHealth` is the one hook both pull from, so they can't drift). Overview/History/Logs/Env then render as real per-tab content, not a stack of always-visible cards.

### Named Rules
**The One Panel Rule.** A row of related instrument readings (CPU/memory/disk/load, memory budget, a deployment's own service vitals) lives in one shared panel divided by hairlines, never as a repeated grid of identically-bordered tiles.
**The Living-Resource Rule.** A project or a deployment is a card with a plant glyph and a rollup, never a bare table row — reserve tables for genuinely list-shaped, non-"alive" data (ports, sessions, team members, audit log, workspaces).

## Elevation & Depth

Mostly flat. Cards and inputs sit on a single hairline border (`--border`), not a shadow — this is a screen full of instruments set into one panel, not a stack of floating material cards. Depth exists only where something is genuinely lifted off the surface behind it: modals (`--shadow-lg`, real offset + blur, never a flat colored halo), the station-switcher dropdown panel (`--shadow-lg`), and milestone toasts (`--shadow-md`). Hover/active feedback on buttons and clickable cards is a border-color and background shift, not a shadow.

### Shadow Vocabulary
- **sm** (`0 1px 2px rgba(0,0,0,0.4)`): minor separation, rarely used directly.
- **md** (`0 10px 28px rgba(0,0,0,0.45)`): hover state on a clickable card, milestone toasts.
- **lg** (`0 28px 72px rgba(0,0,0,0.55)`): modals, the station-switcher dropdown — anything genuinely floating above the page.

### Named Rules
**The Flat-Panel Rule.** Nothing on the page is a floating card by default. Elevation is reserved for content that is genuinely temporary/overlaid (modals, dropdowns, toasts).

## Shapes

Small, consistent corner radii throughout: 5px (`sm` — buttons, inputs, pills-as-rectangles), 8px (`md` — cards, the log/terminal frame), 12px (`lg` — modals), and a full pill radius for status beacons and the mode toggle. Nothing sharp-cornered, nothing heavily rounded — the radius scale itself reads as machined rather than soft. Icons (see `src/icons.tsx`) are a single authored line-icon set at 1.7px stroke weight, round caps/joins, drawn in the world's own grammar (a mangrove's prop roots, a ruled ledger, a half-circle gauge, a stacked-drawer cabinet) rather than a generic icon-font import. Per-template icons (`src/templateIcons.tsx`) extend the same grammar with one recognizable glyph per template (Ghost's ghost, Postgres' elephant, Redis's cube stack, Gitea's teacup, Supabase's bolt, MongoDB's leaf, …), falling back to a category icon so a future template never regresses to plain text.

## Components

### Buttons
- **Shape:** 5px radius, 8px 14px padding (`.btn-sm`: 5px 10px).
- **Primary:** solid leaf fill, near-black text (`#0c1509`) for contrast, 600 weight — the only solid-fill button on the page, reserved for the one primary action per view.
- **Danger:** transparent fill, red-bright text, red border on hover/wash background — never solid red at rest.
- **Default/Secondary:** `bg-elevated` fill, border, text color; border brightens on hover.
- **Hover/Active:** border-color shift + subtle background shift; `transform: scale(0.97)` on press (skipped entirely under `prefers-reduced-motion`).

### Status Beacons (`.pill`)
- **Style:** full-radius pill, a small solid dot + uppercase mono label, background at ~14% tint of the semantic color.
- **In-progress state:** the dot gets a `beacon-live` pulsing ring (`box-shadow`-free, pure `border` + `opacity`/`scale` keyframe) — the one authored motion moment for status, exempted from `prefers-reduced-motion` the same way a spinner is, since it is the only signal of ongoing activity.

### Plant Glyph (`src/components/PlantGlyph.tsx`)
A small root-and-sprout SVG whose posture reflects real status, reusing `StatusPill`'s own green/yellow/red/gray classification so a card's glyph and its pill always agree: upright and full (leaf-bright) when healthy, still uncurling with a gentle sway animation (ochre) while building, drooping (red) on failure, a bare stem (text-faint) when stopped. Sits on every project/deployment card and the deployment-detail header. A fresh transition into "running" triggers a one-shot spring-driven scale pop (`lib/spring.ts`, not a CSS keyframe, so it can be interrupted/retargeted the same way `Modal`'s open/close already is) — a real growth burst tied to a real state change, never a decoration on a timer.

### Gauges (`src/components/Gauge.tsx`)
- **Style:** a 250°-sweep SVG arc (track + value stroke) with quarter-turn tick marks and a centered numeric readout — never a full circle (which would read as a generic "progress ring"), and always rendered beside its actual value/label text, never replacing it.
- **Tone:** leaf at rest (the default/healthy range), ochre >70%, red >90%.

### Cards / Containers
- **Corner:** 8px radius.
- **Background:** `bg-card` on `bg`; a nested "instrument socket" (folded into `.instrument-cell`) uses `bg-inset` instead of stacking another bordered card.
- **Border:** 1px `--border`, no shadow at rest; `--leaf-dim` on hover for a clickable card (`.card-clickable`).
- **Padding:** 20px (0 when the card's sole content is a table).
- **Resource cards** (`.project-card`/`.deployment-card`): see Layout's "Resource cards, not tables" above.

### CodeBlock (`src/components/CodeBlock.tsx`)
The one component for a finished block of raw machine output an operator might need to read closely — a build failure, a run-command result — rendered in the same `bg-recorder` terminal-box treatment `LogViewer` uses for a *live* stream. `LogViewer` owns the live-tail case (its strip-chart + recorder status only make sense for an active connection); `CodeBlock` is the static counterpart, so a finished error dump and a live log tail never diverge into two different visual treatments for the same kind of content again (this was a real defect in the previous pass — History's build-failure text was bare red paragraph text next to Logs' proper terminal box).

### Inputs / Fields
- **Style:** `bg-inset` fill, 1px border, 5px radius, 8px 11px padding.
- **Focus:** border shifts to leaf; global `:focus-visible` also gets a 2px leaf-bright outline with offset, for keyboard navigation everywhere (not just inputs).

### Navigation
- **Sidebar nav-link:** icon + label, 8px radius, leaf-wash background + leaf-bright text when active. Active-state membership is explicit (`path === "/"` or a `/projects/...` prefix for "Projects", etc.) rather than an inverse "not one of the other pages" check, so an unmatched path never lights up a nav item by accident.
- **Station switcher:** pinned above the nav list — eyebrow label ("Station") + current workspace name + chevron, opening a dropdown listing every workspace with its project count, a "Manage workspaces" exit, and re-affirming the active selection with a filled leaf dot.
- **Mobile:** the whole nav collapses behind a top-bar hamburger toggle into a full-width drawer, rather than wrapping into a multi-row jumble.
- **Sidebar mark reacts to real state:** an owner-only poll of the same resource budget Admin shows (every 5 minutes — that endpoint's own disk scan is expensive, so this deliberately doesn't poll faster than the backend's own resource-sampler cadence) tints the mark to the danger color when memory or disk is critical, instead of it being a static logo.

### Modal (`src/components/Modal.tsx`)
Spring-driven open/close (`lib/spring.ts`, critically damped) with two widths: `size="md"` (460px, the default, for a form) and `size="lg"` (720px, `.modal-lg`) for anything that lays out a card grid inside itself — the template gallery and GitHub repo picker used to squeeze a `grid-2` into a 460px modal, which is what clipped template descriptions mid-sentence and forced a horizontal scroll on a popup.

### Toast (`src/components/Toast.tsx`)
Rare, on-brand milestone moments (first deploy ever, first rollback ever — gated by a one-time `localStorage` flag via `lib/milestones.ts` so they stay genuinely rare) surface as a small spring-animated toast, bottom-right, `--leaf-dim` bordered. Never used for routine status or errors — those stay in an `.error-banner` or a status pill, read instantly, no animation to wait out.

### Log Viewer / Strip-Chart (`src/components/LogViewer.tsx`)
A signature component: live logs render inside a `log-viewer-frame` with a `log-strip` above the text — one thin vertical tick per received line (height derived from that line's length, a taller red tick when the line matches an error/fail/panic pattern), scrolling right-aligned like a seismograph drum. `ResourceHistoryStrip` reuses the same idiom for periodic resource snapshots, with a `.chart-legend` explaining its tone colors and a capped per-tick width (`flex: 0 1 10px`) so a fresh instance with only one or two samples reads as "a few real readings," not one bar stretched to fill the whole strip like a decorative fill.

## Do's and Don'ts

### Do:
- **Do** pair every gauge/beacon/plant-glyph with its real numeric or word value in text — the visualization augments the content, it never replaces it.
- **Do** use `.instrument-row`/`.instrument-cell` (a divided panel) for any new row of related readouts, not a grid of separate bordered tiles.
- **Do** render a project or deployment as a card with a `PlantGlyph` and a rollup, not a bare table row (the Living-Resource Rule) — reserve tables for genuinely list-shaped data.
- **Do** keep the station/workspace switcher as ambient, always-visible sidebar context — never move workspace management back to a flat top-level nav item.
- **Do** use the authored icon set in `src/icons.tsx` (or `src/templateIcons.tsx` for a template) for any new icon; extend it in the same 1.7px stroke/round-cap grammar rather than importing an icon font.
- **Do** reserve monospace for genuinely measured/logged values.
- **Do** route any internal enum/DB value through `humanLabel()` and any timestamp through `fmtWhen()`/`fmtWhenFull()` before it reaches JSX.
- **Do** keep swamp-voice copy confined to loading/empty states (`lib/copy.ts`) — errors and anything an operator must act on stay plain and legible first.

### Don't:
- **Don't** use brass for a primary call-to-action, or leaf for structural chrome (borders, plaque labels, dividers) — the Two-Accent Rule is directional, not just "don't add a third color."
- **Don't** fake physical material (embossing, stamped-metal gradients, skeuomorphic bevels) — the instrument-room feeling comes from vocabulary and structure, not costume CSS.
- **Don't** add a kicker/eyebrow label above a page or card heading purely for decoration (the `station-switcher-eyebrow` and table `<th>` labels are functional value-labels for a live control or column, not decorative kickers, and that distinction should hold for anything new).
- **Don't** let a table's row-action button be the thing that determines whether the layout overflows — give a data-dense table its own full-width row rather than fighting it into half a `grid-2`.
- **Don't** give the same content type (e.g. raw build/command output) two different visual treatments in two different places — use `CodeBlock` for a finished dump, `LogViewer` for a live stream, never a bare `<p>`/inline style.
- **Don't** let an unmatched route silently render an existing page (it used to fall through to Projects/Your-apps) — `NotFoundPage` renders for anything that isn't an explicitly matched path.
- **Don't** fill a card/panel background with an accent color because something inside it is positive/active/important — the one narrow exception is a page-level decorative banner (`.project-hero`) that carries no status of its own; a card or panel always stays neutral (`bg-card`/`bg-elevated`/`bg-inset`) regardless of what's inside it.
