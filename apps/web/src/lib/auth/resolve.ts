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
  /**
   * Izin efektif dari klaim `permissions` token (izin peran dari basis data + izin langsung yang berlaku saat
   * terbit). Undefined bila klaim tidak ada (mode mock/dev): pemeriksaan izin jatuh ke matriks bawaan peran.
   */
  permissions?: readonly string[];
}

/** Klaim `permissions` token → daftar izin; undefined bila bukan larik string. */
export function izinDariKlaim(klaim: unknown): readonly string[] | undefined {
  return Array.isArray(klaim) && klaim.every((p) => typeof p === "string") ? (klaim as string[]) : undefined;
}

export const DEV_ROLE_COOKIE = "garda_dev_role";

/**
 * Sesi demo lewat cookie `garda_dev_role` HANYA di pengembangan. Di production
 * cookie itu diabaikan: tanpa JWT sah, sesi null (gagal tertutup). Sebelumnya
 * flag ini selalu true sehingga siapa pun yang menyetel cookie bisa menjadi
 * peran apa saja di sisi UI (API Go tetap menolak, tetapi UI tidak boleh terbuka).
 */
export function devAuthEnabled(env: Record<string, string | undefined> = process.env): boolean {
  return env.NODE_ENV !== "production";
}

export const SESSION_COOKIE = "garda_access";

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
      const { payload } = await jwtVerify(token, new TextEncoder().encode(secret), { issuer: "garda-auth", audience: "garda-api" });
      if (isRole(payload.role) && typeof payload.sub === "string") {
        return {
          id: payload.sub,
          nama: String(payload.name ?? ""),
          email: String(payload.email ?? ""),
          role: payload.role,
          lembaga: String(payload.org ?? ""),
          kabupatenKode: typeof payload.kab === "string" ? payload.kab : undefined,
          permissions: izinDariKlaim(payload.permissions),
        };
      }
    } catch {
      return null;
    }
  }
  if (devAuthEnabled()) {
    const r = getCookie(DEV_ROLE_COOKIE);
    return demoUser(isRole(r) ? r : "MANAJER_PROGRAM");
  }
  return null;
}
