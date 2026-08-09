# Multi-user and roles

Mangrove has two roles: **owner** and **member**. There's no finer-grained
permission system -- every non-owner is a member, full stop.

## How an account is created

- The **very first account**, created via the one-time setup flow when no
  users exist yet, is automatically `owner`.
- Every account after that is created by an existing owner, from the
  dashboard's Admin page (technical mode only) or `POST /api/admin/users`
  -- there's no self-service signup and no outbound invite email. "Invite"
  means an owner sets an initial email/password directly and shares it out
  of band; the new user can change their password afterward via
  `POST /api/auth/change-password`.

## What a member can't do

Everything not listed here works the same for both roles (deploying,
rolling back, viewing logs, editing non-secret env vars, connecting a
GitHub repo, installing templates). A member is blocked from exactly four
things, each enforced **server-side** (the dashboard also hides the
corresponding UI, but the API is the real boundary -- see
`internal/api/roles_test.go` for the tests that hit these routes directly
as a member and assert `403`, independent of what the UI shows):

1. **Deleting** a project or deployment (`DELETE /api/projects/{id}`,
   `DELETE /api/deployments/{id}`).
2. **Managing other users** -- listing, creating, or removing accounts
   (everything under `/api/admin/users`).
3. **Setting a secret env var** (`PUT /api/services/{id}/env/{key}` with
   `is_secret: true`). Non-secret env vars remain open to members.
4. **Changing access control** -- flipping a deployment
   public/private or setting its password
   (`POST /api/deployments/{id}/access`).

The rationale for each: deletion and access-control changes are
destructive/security-relevant actions with no undo; user management is an
obvious privilege-escalation vector if members could grant themselves or
others owner access; secrets are, definitionally, things not everyone with
dashboard access should be able to read or overwrite.

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

Session validation (`internal/auth`) loads the caller's role in the same
query as the session lookup itself (`GetSessionByTokenHash` joins
`sessions` to `users`), so role-gating costs no extra round trip.
`RequireAuth` middleware stashes the role in request context;
`RequireOwner` (a second, stackable middleware) checks it and returns
`403 {"error":"owner role required"}` before the handler runs at all for
routes wrapped in it (see `internal/api/router.go` for exactly which
routes that's applied to). A handful of routes without a matching REST
verb to hang middleware off cleanly (the secret-write path inside
`setEnvVar`, which is otherwise open to members for non-secret writes)
check the role inline instead, but the effect is identical: role comes
from the same context value either way.

On the frontend, `web/src/userContext.tsx` exposes `useIsOwner()` (a small
wrapper over the current user's role, set once at login/session-restore)
so pages can hide owner-only controls -- purely cosmetic, since the API
enforces the real boundary regardless of what the UI shows or hides.
