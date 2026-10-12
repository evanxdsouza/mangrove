# The dashboard (web/)

How the web dashboard is put together, how to work on it, and the handful
of patterns every page follows. For *what it should look like* (palette,
type, the tide language, do's and don'ts) see [DESIGN.md](../DESIGN.md);
for the two UI flows see [modes.md](modes.md). This file is the "how".

## The shape of it

The dashboard is a **SvelteKit 2 + Svelte 5 single-page app** that builds
to a plain directory of static files. There is no Node process in
production:

```
web/  --npm run build-->  internal/webui/dist/  --go:embed-->  mangrove binary
```

- `@sveltejs/adapter-static` writes straight into `internal/webui/dist`
  (`web/svelte.config.js`), with `fallback: "index.html"`.
- `src/routes/+layout.ts` sets `ssr = false` and `prerender = false`:
  every route renders client-side from that one `index.html`.
- `internal/webui/embed.go` embeds `dist/` and serves it. A path that isn't
  a real file gets `index.html`, so a hard refresh on
  `/projects/3/deployments/7` works and SvelteKit's client router takes
  over from there.
- SvelteKit's server features (`+page.server.ts`, form actions, server
  `load`, hooks) are **deliberately unused**. Pages fetch from the Go API
  themselves. Adding a server-only file would break the static build
  (`strict: true` in the adapter config fails it loudly).

Why SvelteKit and not a hand-rolled router: see
[architecture.md](architecture.md#why-chi-why-no-cgo-sqlite-why-a-hand-rolled-frontend-router).
SvelteKit is pinned to 2.x; 3.0 (October 2026) hasn't been evaluated.

### Caching contract

`kit.appDir` is `"assets"`, so the build output looks like:

| Path | Hashed? | `Cache-Control` set by `embed.go` |
|---|---|---|
| `index.html` (and every SPA-fallback path) | no | `no-store` |
| `assets/immutable/**` | yes, content hash in the filename | `public, max-age=31536000, immutable` |
| `assets/version.json` | no | `no-store` |
| `favicon.svg` and other `static/` files | no | `no-store` |

Only `assets/immutable/` is cached forever. If you ever change `appDir` or
add an unhashed file under it, re-check this table;
`internal/webui/embed_test.go` covers all three cases (the asset tests skip
when the frontend hasn't been built).

`dist/` is gitignored except for a `.gitkeep`, so `go build` works on a
fresh clone. Vite empties `dist/` on every build, which is why the `build`
script ends with `touch ../internal/webui/dist/.gitkeep`.

## Commands

All run from `web/`:

| Command | What it does |
|---|---|
| `npm install` | Install deps. On a small box keep this in `web/`, never under a tmpfs `/tmp`. |
| `npm run build` | Production build into `internal/webui/dist`. Rebuild the Go binary afterwards to pick it up. |
| `npm run check` | `svelte-kit sync` + `svelte-check`: TypeScript and Svelte diagnostics for `.ts` and `.svelte`. Should report 0 errors, 0 warnings. |
| `npm run lint` | `oxlint src`: fast lint of the TypeScript. `.svelte` files are covered by `check`. |
| `npm run dev` | Vite dev server with hot reload; proxies `/api` to `127.0.0.1:7777`. |

### Dev loop

1. Run a Mangrove backend on `:7777` (`go build -o mangrove ./cmd/mangrove && ./mangrove`;
   it needs Docker and a Caddy admin API, see the README's Quickstart).
2. `cd web && npm run dev` and open the URL Vite prints (port 5173 by
   default). Session cookies work because the browser only ever talks to
   Vite's origin, which forwards `/api/*`.
3. Before committing: `npm run build && npm run check && npm run lint`.

`npm run dev` against a **production** backend is fine for reading, but
anything you click is real. Prefer a throwaway backend (see
[QA against a throwaway instance](#qa-against-a-throwaway-instance)).

## Source layout

```
web/
  svelte.config.js        adapter-static -> ../internal/webui/dist, appDir "assets"
  vite.config.ts          sveltekit() + the /api dev proxy
  static/favicon.svg
  src/
    app.html              HTML shell + the pre-paint theme/appearance script
    routes/               one thin file per URL (see below)
    lib/
      api.ts              typed fetch client + every API response type
      errors.ts           errMsg(): any thrown value -> a display string
      format.ts           dates, durations, enum -> human labels
      status.ts           status -> colour, worst-status rollup, in-progress set
      tide.ts             status -> tide level (high/mid/low/dry)
      plainCopy.ts        simple-mode wording + SIMPLE_TEMPLATES
      copy.ts             loading / empty-state voice lines
      milestones.ts       firstTime(key): once-per-browser flags
      spring.ts           critically-damped spring animator
      slug.ts, hash.ts    small pure helpers
      icons.ts            authored icon table (render with Icon.svelte)
      templateIcons.ts    per-template icons
      styles.css          the whole design system (tokens + classes)
      stores/             app-wide state, Svelte 5 rune modules
      pages/              one component per screen (+ simple/)
      components/         shared parts, grouped by area
```

### Routes

Route files stay thin. They read params, pick a page component, and pass
numbers in. All logic lives in `src/lib/pages/`.

| URL | Route file | Renders |
|---|---|---|
| `/` | `routes/+page.svelte` | `ProjectsPage`, or `SimpleAppsPage` in simple mode |
| `/projects/:projectId` | `routes/projects/[projectId]/+page.svelte` | `ProjectDetailPage` |
| `/projects/:p/deployments/:d` | `.../deployments/[deploymentId]/+page.svelte` | `DeploymentDetailPage`, or `SimpleAppDetailPage` in simple mode |
| `/projects/:p/deployments/:d/history/:h` | `.../history/[historyId]/+page.svelte` | `RunDetailPage` |
| `/workspaces` | `routes/workspaces/+page.svelte` | `WorkspacesPage` |
| `/admin` | `routes/admin/+page.svelte` | `AdminPage` (owner-only data; the API enforces it) |
| `/server-health` | `routes/server-health/+page.svelte` | `ServerHealthPage` |
| `/storage` | `routes/storage/+page.svelte` | `StoragePage` |
| `/settings` | `routes/settings/+page.svelte` | `SettingsPage` (appearance, instrument metal, password) |
| anything else | `routes/[...rest]/+page.svelte` | `NotFoundPage` |

`+error.svelte` also renders `NotFoundPage` for a 404 and `ErrorFallback`
for anything else, but the `[...rest]` catch-all means unknown paths
normally never reach it. An unknown path must never silently show the
Projects list.

### The root layout

`routes/+layout.svelte` does four jobs, in order:

1. **Imports global CSS**: the `@fontsource` faces (Latin and Latin
   Extended subsets only, to keep the binary small), `lib/styles.css`, and
   `lib/stores/theme.svelte` for its side effect of applying the theme.
2. **Session gate.** It calls `GET /api/auth/status`, then `GET /api/auth/me`,
   and lands in one of `loading`, `needs-setup`, `needs-login` or
   `authenticated`. Anything but authenticated renders `LoginPage` (in
   setup mode on a fresh install). An unreachable API is treated as
   needs-login, so the user gets a form rather than an endless spinner.
3. **Shell.** Once authenticated it renders `Layout` (sidebar, station
   switcher, ⌘K search, host-critical dot) around the page.
4. **Error boundary.** Each page renders inside `{#key page.url.pathname}`
   and a `<svelte:boundary>`. A page that throws while rendering shows
   `ErrorFallback` instead of a blank screen. The `#key` makes navigation
   remount the page, which resets its state and polls cleanly.

`ToastStack` sits outside the gate so toasts survive a logout.

## State: rune stores

App-wide state lives in `src/lib/stores/*.svelte.ts`. Each module exports a
`$state` object and a few functions. There are no Svelte 4 `writable`
stores and no context providers. Import the object and read it; Svelte
tracks the reads.

| Store | Exports | Persisted as (localStorage) |
|---|---|---|
| `user.svelte.ts` | `session.user`, `currentUser()`, `isOwner()` | not persisted; set by the root layout |
| `ui-mode.svelte.ts` | `uiMode.mode` (`technical`/`simple`), `setMode()` | `mangrove-ui-mode` |
| `workspaces.svelte.ts` | `workspaceState` (list + `activeWorkspaceId`), `reloadWorkspaces()`, `setActiveWorkspaceId()`, `workspaceRole(id)` | `mangrove-active-workspace` |
| `theme.svelte.ts` | `themeState`/`setTheme()` (the instrument metal), `appearanceState`/`setAppearance()` (light/dark/system) | `mangrove-theme`, `mangrove-appearance` |
| `toast.svelte.ts` | `toasts`, `showToast()`, `dismissToast()` | not persisted |

Other localStorage keys: `mangrove-sidebar-collapsed` (in `Layout.svelte`)
and `mangrove-milestone-*` (in `milestones.ts`).

Rules every store follows:

- **Wrap every localStorage access in `try`/`catch`.** Private browsing
  and blocked storage throw. The feature should still work for the
  session; it just doesn't persist.
- **Validate what you read back.** An unknown stored value falls back to
  the default. For example, a deleted workspace id falls back to "All
  workspaces".
- **Role checks are cosmetic.** `isOwner()` and `workspaceRole()` hide
  buttons. The real boundary is the API (`auth.RequireOwner`,
  `auth.RequireWorkspaceRole`); see [multi-user.md](multi-user.md).
  `workspaceRole()` returns `""` when unknown, so `=== "admin"` checks fail
  closed.

## Talking to the API

`lib/api.ts` is the only place that calls `fetch` for JSON:

```ts
import { api, type Deployment } from "$lib/api";
import { errMsg } from "$lib/errors";

const d = await api.get<Deployment>(`/api/deployments/${id}`);
await api.post(`/api/deployments/${id}/sleep`, { enabled, idle_minutes });
```

- Requests are same-origin with the session cookie. There are no tokens
  in JavaScript.
- A non-2xx response throws `ApiError` with `.status` and the server's
  `{ "error": "..." }` message. Show it with `errMsg(e)`.
- `204` resolves to `undefined`.
- The TypeScript interfaces in `api.ts` mirror `internal/models` and the
  handler response shapes **by hand**. When you add or rename a JSON
  field in Go, update the interface in the same change.

Log streaming is the one exception: `LogViewer.svelte` opens an
`EventSource` on `/api/services/:id/logs/stream` and closes it in the
effect cleanup.

### Polling

There are no websockets for state. Detail pages poll, and every poll
follows the same shape:

```svelte
<script lang="ts">
  let { deploymentId }: { deploymentId: number } = $props();
  let deployment = $state<Deployment | null>(null);
  let error = $state<string | null>(null);

  function load() {
    api.get<Deployment>(`/api/deployments/${deploymentId}`)
      .then((d) => (deployment = d))
      .catch((e) => (error = errMsg(e)));
  }

  $effect(() => {
    void deploymentId;               // re-run when the id changes
    load();
    const t = setInterval(load, 4000);
    return () => clearInterval(t);   // always clean up
  });
</script>
```

Intervals in use: 4 s for project, deployment, run and simple-mode pages
(and `latestHealth.svelte.ts`), 5 s for Server health, and 5 min for the
sidebar's host-critical check. Keep new polls at 4 s or slower. The box
this runs on is small, and some endpoints (`/api/admin/resource-budget`)
are expensive.

### Editable cards under a poll

A settings card (sleep, self-heal, scale, access control...) gets a fresh
`deployment` object every 4 seconds. Copying props into local state on
every update would wipe whatever the user is typing. The cards only
re-sync when the **server's values actually changed**:

```ts
let synced = "";
$effect(() => {
  if (!deployment) return;
  const sig = JSON.stringify([deployment.sleep_enabled, deployment.sleep_idle_minutes]);
  if (sig === synced) return;   // same server state: keep the user's edits
  synced = sig;
  enabled = deployment.sleep_enabled;
  idleMinutes = deployment.sleep_idle_minutes;
});
```

Use this pattern for any new form that sits on a polled page.

## Components worth knowing

- **`Modal.svelte`**: spring open/close, Escape and backdrop to dismiss.
  Its `children` snippet **receives `close`**. Wire Cancel buttons to it
  so they play the exit animation:

  ```svelte
  <Modal title="Rename" onClose={() => (open = false)}>
    {#snippet children(close)}
      ...
      <button class="btn" onclick={close}>Cancel</button>
    {/snippet}
  </Modal>
  ```

  Nested components can get the same function with
  `getContext("modal-close")`. `ConfirmModal` wraps this for yes/no
  prompts. `size="lg"` (760 px) is for dialogs that lay out a grid.
- **`TideGauge.svelte`** and **`TideBanner.svelte`**: the status language.
  Both take a raw `status` and map it with `tideLevel()`, so a gauge, its
  banner and its `StatusPill` always agree. `TideBanner` switches to night
  palettes when `appearanceState.scheme === "dark"`.
- **`StatusPill.svelte`**: the text partner of a gauge. Simple mode uses
  `pages/simple/PlainStatusPill.svelte` for plain-language labels.
- **`Gauge.svelte`**: tide bars for resource meters (memory, disk).
- **`Icon.svelte`**: `<Icon name="deploy" />`. Names come from
  `lib/icons.ts`. To add an icon, add an entry there: inner SVG markup on
  a 24 px grid, 1.9 stroke, round caps. No icon fonts and no emoji.
- **`LogViewer.svelte`**: streamed logs with stick-to-bottom auto-scroll.
- **`ErrorFallback.svelte`**, **`CenterLoading.svelte`**, and
  `components/skeleton/*`: error, loading and skeleton states. Use the
  skeletons for lists so the layout doesn't jump when data arrives.
- **`components/deployment/*`**: the cards on a deployment's Overview and
  Settings tabs, one per feature (sleep, self-heal, scale, staging,
  previews, promote, auto-deploy, access control, webhook health).

## Styling, themes and dark mode

All styling is in **one hand-written stylesheet**, `src/lib/styles.css`.
There is no Tailwind and no component library. Components use its shared
classes (`.card`, `.btn`, `.btn-primary`, `.pill`, `.field`,
`.kv-list`, `.page-header`, `.empty-state`, ...) and only add scoped
`<style>` for layout that truly belongs to one component.

Colours, spacing, radii and shadows are CSS custom properties on `:root`.
Never hard-code a colour in a component. If no token fits, add one, and
give it a dark value too.

Two independent attributes on `<html>` change the palette:

| Attribute | Values | Set by | Changes |
|---|---|---|---|
| `data-color-scheme` | `light`, `dark` | `appearanceState` (Settings → Appearance) | the whole palette, via one `:root[data-color-scheme="dark"]` token block plus a few component overrides |
| `data-theme` | `brass` (default), `copper`, `iron`, `silver`, `indigo` | `themeState` (Settings → Instrument metal) | only the `--brass*` accent used by small sidebar labels |

Appearance has three choices. **System** (the default) follows
`prefers-color-scheme` live, including when the OS switches at sunset.
**Light** and **Dark** override it. Whatever is chosen, the store resolves
it to `data-color-scheme="light"` or `"dark"`. That one attribute is all
the CSS keys on; the CSS never uses `@media (prefers-color-scheme)`.

**No flash on load:** `src/app.html` runs a tiny inline script before the
app bundle loads. It reads `mangrove-theme` and `mangrove-appearance`
from localStorage and sets both attributes. The store then re-applies
them and keeps them in sync. If you add another visual preference, set it
in both places.

When you add UI, check it in both schemes. In practice:

- Use tokens such as `--bg-card`, `--bg-input`, `--bg-modal`, `--text*`,
  `--border*` and the status washes, not literal hex values.
- SVG art that can't use CSS variables (like `TideBanner`'s gradients)
  reads `appearanceState.scheme` and picks a palette in script.
- Dark status pills use opaque fills, because translucent washes go muddy
  on the dark page. See DESIGN.md's "Dark appearance" section.

## Recipes

### Add a page

1. Create `src/lib/pages/FooPage.svelte`. Poll with the pattern above if
   it shows live state.
2. Create `src/routes/foo/+page.svelte` that imports and renders it, with
   numeric params converted using `Number(page.params.x)`.
3. If it has a simple-mode version, branch on `uiMode.mode` in the route
   file, the way `/` does (see [modes.md](modes.md)).
4. Add a sidebar entry in `components/Layout.svelte` if it's top-level.
   Keep the active-state check explicit, so a 404 never highlights it.
5. Update `docs/REPO_MAP.md` (the `web/` row and "Where to look").

### Add a field from the API

1. Add the field to the Go model/handler.
2. Add it to the matching interface in `src/lib/api.ts`.
3. Render it. If it's editable on a polled page, include it in that
   card's sync signature.

### Add a design token

1. Add it to `:root` in `styles.css` with a light value and a short
   comment.
2. Add its dark value to the `:root[data-color-scheme="dark"]` block.
3. If it's part of the documented palette, add it to DESIGN.md.

## QA against a throwaway instance

Don't point experiments at a live install, and don't run `./e2e/run.sh` on
a box that also runs production Mangrove: its teardown force-removes every
`mangrove-*` container. Instead, run a second, isolated binary:

```sh
cd web && npm run build && cd ..
go build -o "$SCRATCH/mangrove-qa" ./cmd/mangrove

MANGROVE_DATA_DIR="$SCRATCH/data" \
MANGROVE_STATIC_SITES_DIR="$SCRATCH/static" \
MANGROVE_PORT=7791 \
MANGROVE_NETWORK=mangrove-uitest-net \
MANGROVE_CADDY_ADMIN_ADDR=http://127.0.0.1:9 \
MANGROVE_PORT_RANGE_MIN=39100 MANGROVE_PORT_RANGE_MAX=39110 \
"$SCRATCH/mangrove-qa"
```

- The separate data dir, Docker network and port range keep it away from
  the real install's DB, containers and ports.
- Pointing the Caddy admin address at a closed port (`:9`) means it can
  never touch the live Caddy config.
- Seed data through the API. To see every visual state at once, set a
  spread of deployment statuses (`running`, `building`, `failed`,
  `stopped`) directly in the scratch SQLite DB.
- Check both appearances: change Settings → Appearance, or emulate the
  colour scheme in Playwright with `colorScheme: "dark"`. Also check
  1440 px and 390 px widths.
- `POST /api/auth/login` (and setup) is rate-limited to 5 requests per 5 minutes per
  IP (`internal/api/router.go`). A script that logs in for every page will
  start failing with a generic error; log in once and reuse the cookie,
  or restart the instance to reset the limiter.
- Tear down when done: stop the process by its PID,
  `docker network rm mangrove-uitest-net`, and delete `$SCRATCH/data`.

## Shipping a frontend change

The dashboard is compiled into the binary, so a UI change ships like any
backend change: `npm run build`, `go build`, then the binary swap in
[deployment.md](deployment.md). Open tabs pick up the new UI on their next
full load, because `index.html` is never cached and the hashed bundles
it points to are new filenames.
