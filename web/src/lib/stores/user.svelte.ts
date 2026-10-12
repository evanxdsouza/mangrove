import type { CurrentUser } from "../api";

// The signed-in user, set once by the root layout after the session check.
// UX-only: the real enforcement boundary is the backend (see
// internal/auth.RequireOwner and the owner checks in internal/api), so a
// stale or tampered client-side role can hide a button but never grants
// real access.
export const session = $state<{ user: CurrentUser | null }>({ user: null });

export function currentUser(): CurrentUser {
  if (!session.user) throw new Error("currentUser() called before the session loaded");
  return session.user;
}

export function isOwner(): boolean {
  return session.user?.role === "owner";
}
