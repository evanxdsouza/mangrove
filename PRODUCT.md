# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Two overlapping audiences on one shared backend, toggled via a Technical/Simple
mode switch:

- **Technical operators** (the primary/default mode): people comfortable with
  Docker, env vars, and Git who are self-hosting their own apps and other
  people's apps on a small box they own or rent (reference target: a
  2 vCPU / 2GB RAM / 16GB VPS). They think in terms of projects, deployments,
  build strategies, workspaces, domains, and deploy history.
- **Non-technical owners** (Simple mode): someone who wants "a blog" or "a
  password manager" running, not someone who wants to configure a Dockerfile.
  Same backend, a flattened "your apps" view with plain-language status and a
  two-action model (Try again / Remove).

Both can be the same person switching context, or literally different people
sharing one instance under owner/member roles.

## Product Purpose

Mangrove is a small, self-hosted PaaS control plane: point it at a Git repo, a
Dockerfile, a docker-compose file, or a pre-built image, and it builds, runs,
health-checks, and reverse-proxies it (blue/green, automatic rollback). The
web dashboard is the primary control surface for all of that — the thing this
redesign covers. Success is an operator being able to see, at a glance, the
state of everything they've deployed, and act on it (deploy, roll back,
inspect logs, open a live terminal, manage domains/env vars/access) without
digging.

## Positioning

Where a hosted PaaS (Vercel/Render/Railway) runs on the vendor's
infrastructure and bills per usage, Mangrove runs entirely on hardware the
user already owns or rents, with no metering and no vendor lock-in — the
trade being that the user (or this dashboard, on their behalf) is the ops
team. Its differentiator is doing the "real PaaS" feature set (GitHub
auto-deploy, staging environments, PR previews, custom domains with
auto-TLS/DDNS, one-click templates, a live browser terminal, drive-to-NAS
sharing) on a single small self-hosted box, not just running containers.

## Operating Context

- Single Go binary embeds the built SPA (`internal/webui`) — no separate
  frontend deploy, no CDN; the dashboard is served by the same process it's
  managing.
- Real infrastructure underneath: Docker (build/run), Caddy (reverse proxy +
  TLS, driven via its admin API), SQLite (source of truth). The dashboard is
  a control surface over genuinely live infrastructure state (running
  containers, health checks, live logs, a live pty), not a mock/staging UI.
  Loading and error states are not decorative — they reflect a real backend
  that can be mid-build, mid-health-check, or briefly unreachable.
- Owner/member roles gate a real subset of actions (admin, storage, some
  danger-zone operations are owner-only) — this is a functioning permission
  boundary the UI must reflect honestly, not just visually gray out.
- Deployments are grouped into **projects**, and projects are grouped into
  **workspaces** (e.g. by environment or team) — workspace is a structural
  organizing concept across the whole technical-mode dashboard, not a
  one-off settings page. This redesign's explicit trigger is that the
  current implementation treats "Workspaces" as an orphaned flat nav item
  disconnected from everywhere else workspace context matters (the projects
  list, project creation, etc.) — the IA needs the workspace to behave as
  ambient scoping context, not a page you have to detour through.
- Simple mode has **no project or workspace concept at all** — it flattens
  every deployment into one list. The two modes are intentionally different
  information architectures over the same data, not the same screens
  relabeled.

## Capabilities and Constraints

- No react-router, no Redux/state library — hand-rolled `router.tsx` (path +
  simple `matchPath`) and small context providers (`userContext`,
  `uiMode`). Any redesign works within this, not by introducing a routing
  library.
- No component/CSS framework (no Tailwind, no MUI) — one hand-written
  `styles.css` with CSS custom-property tokens. A design-token-driven
  approach is already the established pattern, not something to introduce.
- Existing tokens already handle `prefers-reduced-motion`,
  `prefers-reduced-transparency`, and `prefers-contrast: more` — any new
  token/component work must preserve these, not regress them.
- Real-time-ish surfaces exist today (log streaming, deploy timeline, xterm.js
  terminal over websocket) and must keep working, not just look better.
- Full page inventory (technical mode): Login/setup, Projects (list),
  Workspaces (list), Project detail, Deployment detail (build strategy, env
  vars, domains, deploy history/rollback, logs, terminal), Admin (users,
  ports, GitHub tokens, pruning), Server health, Storage (NAS), Settings.
  Simple mode: Your apps (list), App detail, Add-app modal.
- Terminology to preserve: "Projects", "Workspaces", "Deployments",
  "Build strategy", "Simple mode" / "Technical mode", "owner" / "member".

## Brand Commitments

- Name: **Mangrove**. No existing logo/mark beyond a small CSS gradient
  square in the current sidebar (`--accent` → `--green` gradient) — not a
  binding asset, fair game to redesign along with everything else per this
  session's "full visual reinvention" direction.
- No other confirmed brand assets (no style guide, no marketing site
  reviewed as part of this task).

## Evidence on Hand

- No user research, screenshots of complaints, or analytics — the brief for
  this redesign is the user's direct assessment in this session ("the UI is
  horrible... nothing is tied together") plus this agent's own reading of
  the current implementation. No fabricated testimonials/metrics belong
  anywhere in the redesigned UI.

## Product Principles

1. **Real infrastructure, not a mock** — every state (loading, error, empty,
   live-updating) must read as honestly reflecting real Docker/Caddy/DB
   state, never as decorative placeholder UI.
2. **Workspace and project are ambient context, not detours** — an operator
   should always know which workspace/project they're in and be able to
   switch without leaving their task, not navigate to a separate page to
   change scope.
3. **Two audiences, one system** — Technical and Simple mode must feel like
   the same product at two altitudes (shared visual language, shared
   components where the concepts overlap), not two unrelated skins.
4. **Density with clarity** — technical operators want real information
   (status, resource numbers, build strategy, timestamps) visible without
   digging, but it must stay scannable, not turn into a wall of raw data.
5. **Self-hosted, not enterprise-SaaS-generic** — this runs on a box someone
   owns, often for personal or small-team use; the tone and craft should
   feel considered and a little distinctive, not like a templated admin
   panel.

## Accessibility & Inclusion

No explicit standard was specified. Preserve and extend the existing
`prefers-reduced-motion` / `prefers-reduced-transparency` / `prefers-contrast`
handling already in `styles.css`; treat WCAG AA contrast as the working floor
given no stricter requirement was stated.
