import { createContext, useContext, type ReactNode } from "react";
import type { CurrentUser } from "./api";

// Mirrors router.tsx's hand-rolled context pattern -- avoids prop-drilling
// role through every page/component that needs to hide an owner-only
// action. This is UX-only: the actual enforcement boundary is the backend
// (see internal/auth.RequireOwner and the owner checks in internal/api),
// so a stale or tampered client-side role can hide a button but never
// grants real access.
const UserContext = createContext<CurrentUser | null>(null);

export function UserProvider({ user, children }: { user: CurrentUser; children: ReactNode }) {
  return <UserContext.Provider value={user}>{children}</UserContext.Provider>;
}

export function useUser(): CurrentUser {
  const ctx = useContext(UserContext);
  if (!ctx) throw new Error("useUser must be used within a UserProvider");
  return ctx;
}

export function useIsOwner(): boolean {
  return useUser().role === "owner";
}
