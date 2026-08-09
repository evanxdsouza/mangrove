# Technical vs. simple mode

The dashboard has two front-end flows sharing one backend, toggled from
the sidebar (persisted in `localStorage`, so the choice survives a
reload). This isn't a relabeling of the same screens -- simple mode is a
genuinely separate set of pages, routed by `web/src/App.tsx` based on
`web/src/uiMode.tsx`'s `UiMode` (`"technical" | "simple"`).

## Technical mode

The full dashboard: projects group deployments, deployments have build
strategies (Dockerfile/nixpacks/compose/image/static), env vars (secret
and non-secret), access control, deploy history with rollback, live log
streaming, and per-service resource numbers (CPU/memory limits, health
check status, host port). This is what every doc other than this one
describes.

## Simple mode

Built for someone who wants "a blog" or "a password manager" running, not
someone who wants to configure a Dockerfile. Concretely:

- **No project concept.** `SimpleAppsPage` flattens every deployment
  across every project into one "Your apps" list -- a user never sees or
  manages a project as a separate thing.
- **"Add an app" is one click**, not two. `AddAppModal` shows only
  user-facing templates (blog, password manager, website monitor,
  automation, website builder, visitor stats -- see
  `SIMPLE_TEMPLATES` in `web/src/pages/simple/plainCopy.ts`) with plain
  names and one-sentence blurbs, deliberately excluding the standalone
  database templates (Postgres/MySQL/MongoDB/Redis): those exist as
  backing services other templates install alongside themselves, not
  something a non-technical user would knowingly add on their own.
  Picking one auto-creates a throwaway project behind the scenes (named
  after the app) and installs the template into it -- the two-step
  technical flow (create project, then install into it) collapses into
  one action the user never sees split apart.
- **Plain-language status**, not the raw status enum: `plainStatus()` in
  `plainCopy.ts` maps `running` &rarr; "Running", `pending`/`building`
  &rarr; "Starting up", `stopped` &rarr; "Turned off", `failed` &rarr;
  "Having a problem". No raw env var table, no CPU/memory numbers, no
  build-strategy internals on `SimpleAppDetailPage`.
- **Two actions**, not the full technical action set: **Try again**
  (re-triggers a deploy; shown only when the app has failed) and
  **Remove** (same delete-with-confirmation flow as technical mode,
  plain-language copy). There's deliberately no start/stop toggle:
  Mangrove's orchestrator has no way to stop a running deployment without
  deleting it (see [architecture.md](architecture.md)'s deploy flow --
  there's no "stopped by user request" state, only "never
  deployed"/"running"/"failed"), so simple mode doesn't show a control
  that wouldn't do anything real.
- **Escape hatch, not a dead end**: a "Switch to advanced view" button on
  `SimpleAppDetailPage` flips the global mode back to technical *on the
  same URL* -- `App.tsx`'s route dispatch just renders
  `DeploymentDetailPage` instead of `SimpleAppDetailPage` for that same
  `/projects/:id/deployments/:id` path, so nothing about the underlying
  data changes, only which component renders it.
- **Admin stays technical-only.** Users, ports, GitHub tokens, resource
  pruning -- none of it has a simple-mode equivalent; the nav link is
  simply hidden in simple mode (`Layout.tsx`). It's still reachable by
  switching back to technical mode, or by navigating to `/admin` directly
  (the route itself doesn't check mode -- only the sidebar entry point is
  hidden).

## Known limitation

Removing an app in simple mode deletes the deployment but leaves behind
its now-empty auto-created project. It's invisible in simple mode (which
only lists deployments, never empty projects), but shows up as a
zero-deployment project if you switch to technical mode. Not destructive,
just a little untidy -- not addressed here since it wasn't part of the
original scope, but straightforward to add (delete the parent project too
if it has zero remaining deployments after the delete) if it turns out to
matter in practice.

## Adding a new template to simple mode

A template only appears in simple mode's gallery if it has an entry in
`SIMPLE_TEMPLATES` (`web/src/pages/simple/plainCopy.ts`). See
[templates.md](templates.md)'s "Adding a new template" section, step 3.
