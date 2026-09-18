# Multi-user and roles

Two independent axes, not one:

- **Global role** (`users.role`: `owner` or `member`) -- host-level. Governs
  things with no per-project meaning at all: managing org accounts, the
  `/admin` panel (sessions, ports, pruning, backups), the storage/NAS
  feature. An `owner` is also an implicit **admin of every workspace**,
  bypassing the second axis entirely -- see "How it's enforced" below.
- **Workspace role** (`workspace_members.role`: `admin`, `editor`, or
  `viewer`) -- per workspace, governs everything workspace-scoped: a
  project's deployments, services, secrets, custom domains, exec/shell
  access. A `member` with no `workspace_members` row in a given workspace
  has **no access to it at all** -- not read-only, none.

Before this existed, `workspace_members` was a schema stub nobody read
(every `member` could see and act on every project in every workspace,
including running an arbitrary command in any container via
`POST /services/{id}/exec` -- the single biggest gap this closed). Finishing
it is what makes `owner`/`member` actually mean "host administrator" vs.
"everyone else," instead of the only permission distinction in the app.

## How an account is created

- The **very first account**, created via the one-time setup flow when no
  users exist yet, is automatically `owner`.
- Every account after that is created by an existing owner, from the
  dashboard's Admin page (technical mode only) or `POST /api/admin/users`
  -- there's no self-service signup and no outbound invite email. "Invite"
  means an owner sets an initial email/password directly and shares it out
  of band; the new user can change their password afterward via
  `POST /api/auth/change-password`.
- Every new account auto-joins the default workspace (id 1) as **editor**
  -- not a mirror of their global role -- so the common single-workspace
  install keeps working immediately without a separate membership step.
  Access to any other workspace, or a different role in the default one,
  is granted explicitly afterward (see "Managing workspace membership").

## What a member can't do (global, host-level)

Everything not listed here works the same for both global roles -- the
real per-project distinctions now live in workspace roles, not here. A
member is blocked from exactly three things, each enforced **server-side**
via `auth.RequireOwner` (see `internal/api/roles_test.go`'s
`TestMemberForbiddenFromOwnerOnlyRoutes`):

1. **Managing other org accounts** -- listing, creating, or removing
   (everything under `/api/admin/users`). An obvious privilege-escalation
   vector if members could grant themselves or others owner access.
2. **Listing or revoking sessions** -- `GET /api/admin/sessions` and
   `DELETE /api/admin/sessions/{id}` operate on *every* session on the box,
   not just the caller's own; letting a member call these would let them
   revoke the owner's session (a lockout vector) or harvest every user's
   session metadata.
3. **Port registry management, container pruning, and backups** --
   `GET`/`POST /api/admin/ports`, `DELETE /api/admin/ports/{port}`,
   `POST /api/admin/prune`, and `GET /api/admin/backup`. All are
   system-wide, unscoped-to-any-workspace operations -- a backup in
   particular is equivalent to every secret on the box (see
   [backup.md](backup.md)).

## Workspace roles: admin / editor / viewer

Scoped per workspace, checked via `auth.RequireWorkspaceRole` (or, for the
handful of routes with no single resource ID to resolve a workspace from,
`auth.HasWorkspaceRole` called inline -- see "How it's enforced"). A
global `owner` always passes regardless of workspace role.

- **viewer**: read-only. View projects, deployments, services, logs,
  history, non-secret env vars, domains.
- **editor**: viewer, plus deploying/redeploying/scaling/stopping/
  restarting/rolling back, editing non-secret env vars, installing
  templates, connecting a repo, creating staging/PR-preview deployments,
  verifying a pending custom domain, and **running a command in a live
  container** (`POST /services/{id}/exec`) -- the route that used to be
  open to any authenticated user regardless of workspace.
- **admin**: editor, plus the four things that used to be global-owner-only
  and are now devolved to whoever administers *that* workspace: deleting a
  project or deployment, setting a secret env var, changing a deployment's
  access control (public/private/password), adding or removing a custom
  domain, deleting the workspace itself, and managing its membership (see
  below). A workspace-admin has **no special power outside their own
  workspace** -- see `TestWorkspaceAdminScopedNotGlobal` in
  `internal/api/workspace_roles_test.go`.

A resource that doesn't exist returns `404` even to someone with no access
to it, not `403` -- a caller shouldn't be able to distinguish "exists, not
yours" from "doesn't exist" by status code.

### Known limitation: GitHub PATs aren't workspace-scoped

`github_pats` is an org-level resource (`org_id`, not `workspace_id`), and
a single PAT can back repos across multiple projects in multiple
workspaces at once via `project_repos.github_pat_id`. There's no single
workspace to check a PAT's routes against without a schema change, so
`/api/github/pats` stays open to any authenticated user, unchanged. Real
gap, left as-is rather than guessed at.

## Managing workspace membership

Admin-only (`/api/workspaces/{id}/members`, `GET`/`POST`/`PUT`/`DELETE`),
reachable from the dashboard's Workspaces page:

- **Add** grants an *existing* org account a role, looked up by exact
  email match -- deliberately not a picker over the full user roster,
  which stays reserved for global owners (see above). This is how a
  workspace-admin who isn't a global owner adds a known colleague without
  that broader visibility.
- **Change role** / **remove** are exactly what they say; removing a
  member revokes their access to that workspace only, it does not touch
  their org account (`DELETE /api/admin/users/{id}` is the only thing that
  does, and stays global-owner-only).

There's no "last admin" guardrail on a workspace the way there is for the
last org owner (`CountOwners`, below) -- a global owner can always still
manage any workspace regardless of its `workspace_members` rows, so a
workspace can never actually get stranded.

## Audit log

Matters more now that workspace-admins, not just global owners, can
delete/set-secrets/change-access -- see "Workspace roles" above.
`audit_log` (`internal/store/audit.go`) records who deployed, deleted, or
changed access, and when, for:

- **Deploy**: every trigger -- manual, redeploy, rollback, scale, promote,
  and an automated GitHub push/PR-preview sync (recorded under the actor
  `github-webhook`, since there's no user session behind a webhook
  delivery) -- action name matches `deploy_history.triggered_by`'s own
  vocabulary (`manual`, `redeploy`, `rollback`, `scale`, `promote`,
  `push`).
- **Delete**: project, deployment, workspace, custom domain, org user.
- **Changed access**: deployment access control (public/private/password
  -- never the password itself), a secret env var (never the value, just
  which key changed), custom domain add, workspace membership add/role
  change/remove, moving a project between workspaces, org user creation,
  and session revocation.

Each entry records `actor_email` (denormalized, so it stays readable if
the account is later deleted), the action, the resource type/ID, the
workspace (nil for an org-level action like user management), and a short
detail string. **Never pruned** -- unlike `health_checks` or
`resource_usage_snapshots`, an audit trail that quietly expires isn't one.

Two read endpoints, matched to the two-axis model above:

- `GET /api/workspaces/{id}/audit-log` -- viewer+ in that workspace (any
  member, not just its admins: accountability for what happened in your
  own workspace is exactly the kind of thing a plain member should be
  able to see). Scoped to only that workspace's events.
- `GET /api/admin/audit-log` -- owner-only, every event across every
  workspace, including org-level ones with no `workspace_id` at all.

GitHub PATs aren't included (see the "Known limitation" above -- the same
reason they aren't workspace-scoped for authorization means there's no
single workspace to attribute a PAT change to either).

## Deleting a user

Two guardrails on `DELETE /api/admin/users/{id}` (owner-only to call at
all, via `RequireOwner`):

- You can't delete your **own** account (`400`).
- You can't delete the **last remaining owner** (`400`) -- checked by
  counting owners only when the target being deleted is themselves an
  owner, so this never blocks deleting a member.

Together these mean the box can never end up with zero owners and nobody
able to grant owner access to anyone else.

## How it's enforced under the hood

Session validation (`internal/auth`) loads the caller's global role in the
same query as the session lookup itself (`GetSessionByTokenHash` joins
`sessions` to `users`), so role-gating costs no extra round trip.
`RequireAuth` middleware stashes the role in request context; `RequireOwner`
checks it for host-level routes exactly as before.

Workspace roles add a second layer, `internal/auth/workspace.go`:

- `auth.HasWorkspaceRole(ctx, store, minRole, workspaceID)` is the core
  check -- a global owner always passes; otherwise it looks up the
  caller's `workspace_members` row and compares rank (`viewer < editor <
  admin`).
- `auth.RequireWorkspaceRole(store, minRole, resolve)` wraps it as
  middleware for routes with a single resource ID to resolve a workspace
  from (e.g. `{deploymentID}` -> `internal/store`'s
  `WorkspaceIDForDeployment`, one hop; `{serviceID}` -> two hops through
  `deployments` -> `projects`). See `internal/api/workspace_resolvers.go`
  for every resolver and `internal/api/router.go` for exactly which routes
  use which minimum role.
- A handful of routes have no single resource ID to hang middleware off
  (`listProjects` spans every workspace; `createProject`'s target
  workspace is in the JSON body; `setProjectWorkspace` involves *two*
  workspaces, source and destination) and call `HasWorkspaceRole` inline
  instead -- same check, same effect.

**Upgrading an existing install**: `internal/db/migrations/0012_workspace_role_backfill.sql`
backfills a `workspace_members` row for every (workspace, user) pair that
doesn't already have one -- existing owners become admin, existing members
become editor -- so upgrading never silently drops anyone from access they
already had. New workspaces/users after the upgrade get their membership
from `CreateWorkspace` (creator becomes admin) and `CreateUser` (auto-joins
the default workspace as editor) instead of the migration.

On the frontend, `web/src/userContext.tsx`'s `useIsOwner()` (global) and
`web/src/workspaceContext.tsx`'s `useWorkspaceRole(workspaceId)`
(per-workspace, derived from the already-loaded workspace list, no extra
fetch) both exist side by side -- purely cosmetic either way, since the API
enforces the real boundary regardless of what the UI shows or hides.
