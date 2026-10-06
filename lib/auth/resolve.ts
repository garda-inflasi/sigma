/**
 * Resolusi sesi yang dapat dipakai di proxy.ts maupun Server Component.
 * Produksi: JWT HS256 dari layanan auth Go (setelah SSO + MFA) di cookie httpOnly `garda_session`.
 * Pengembangan: cookie `garda_dev_role` dari pengalih peran untuk meninjau RBAC.
 */
import { jwtVerify } from "jose";
import { USERS } from "@/lib/data/reference";
import { ROLE_IDS, type Role } from "@/lib/rbac/roles";

export interface SessionUser {
  id: string;
  nama: string;
  email: string;
  role: Role;
  lembaga: string;
  kabupatenKode?: string;
}

export const SESSION_COOKIE = "garda_session";
export const DEV_ROLE_COOKIE = "garda_dev_role";
const DEV = process.env.NODE_ENV !== "production";

export function demoUser(role: Role): SessionUser {
  const u = USERS.find((x) => x.role === role) ?? USERS[0];
  return { id: u.id, nama: u.nama, email: u.email, role: u.role, lembaga: u.lembaga, kabupatenKode: u.kabupatenKode };
}

const isRole = (v: unknown): v is Role => typeof v === "string" && (ROLE_IDS as readonly string[]).includes(v);

export async function resolveSession(getCookie: (name: string) => string | undefined): Promise<SessionUser | null> {
  const token = getCookie(SESSION_COOKIE);
  const secret = process.env.GARDA_JWT_SECRET;
  if (token && secret) {
    try {
      const { payload } = await jwtVerify(token, new TextEncoder().encode(secret), { issuer: "garda-auth", audience: "garda-web" });
      if (isRole(payload.role) && typeof payload.sub === "string") {
        return {
          id: payload.sub,
          nama: String(payload.name ?? ""),
          email: String(payload.email ?? ""),
          role: payload.role,
          lembaga: String(payload.org ?? ""),
          kabupatenKode: typeof payload.kab === "string" ? payload.kab : undefined,
        };
      }
    } catch {
      return null;
    }
  }
  if (DEV) {
    const r = getCookie(DEV_ROLE_COOKIE);
    return demoUser(isRole(r) ? r : "MANAJER_PROGRAM");
  }
  return null;
}
