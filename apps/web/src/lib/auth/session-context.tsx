"use client";

import { createContext, useContext, type ReactNode } from "react";
import { can, type Permission } from "@/lib/rbac/permissions";
import type { SessionUser } from "./resolve";

const SessionCtx = createContext<SessionUser | null>(null);

export function SessionProvider({ user, children }: { user: SessionUser | null; children: ReactNode }) {
  return <SessionCtx.Provider value={user}>{children}</SessionCtx.Provider>;
}

export function useSession(): SessionUser {
  const u = useContext(SessionCtx);
  if (!u) throw new Error("useSession dipakai di luar sesi yang valid");
  return u;
}

export function useOptionalSession() {
  return useContext(SessionCtx);
}

export function useCan(perm: Permission) {
  return can(useContext(SessionCtx)?.role, perm);
}

/** Render anak hanya bila peran memiliki izin. */
export function Can({ perm, children, fallback = null }: { perm: Permission; children: ReactNode; fallback?: ReactNode }) {
  return useCan(perm) ? <>{children}</> : <>{fallback}</>;
}
