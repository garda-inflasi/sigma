/**
 * Pembacaan klaim JWT di edge (middleware) TANPA memverifikasi tanda tangan.
 *
 * Edge tidak memegang rahasia JWT, jadi hasilnya hanya boleh dipakai untuk
 * keputusan UX (mengalihkan pengguna ke halaman yang sesuai). Penegakan sebenarnya
 * tetap di API Go yang memverifikasi tanda tangan, masa berlaku, dan izin.
 * Modul ini murni (tanpa import Next/Node) sehingga aman di runtime edge dan mudah diuji.
 */

import { ROLE_IDS, type Role } from "@/lib/rbac/roles";

export interface EdgeClaims {
  role: Role;
  /** Daftar izin dari klaim `permissions`; null bila klaim tidak ada. */
  permissions: readonly string[] | null;
  /** Detik epoch kedaluwarsa (klaim `exp`); null bila tidak ada. */
  exp: number | null;
  wajibGantiPassword: boolean;
}

function decodeBase64Url(segment: string): string {
  const padded = segment.replace(/-/g, "+").replace(/_/g, "/");
  const binary = atob(padded.padEnd(Math.ceil(padded.length / 4) * 4, "="));
  const bytes = Uint8Array.from(binary, (c) => c.charCodeAt(0));
  return new TextDecoder().decode(bytes);
}

/** Muatan JWT mentah, atau null bila bukan JWT tiga segmen yang dapat dibaca. */
export function decodeJwtPayload(
  token: string | undefined,
): Record<string, unknown> | null {
  if (!token) return null;
  const parts = token.split(".");
  if (parts.length !== 3 || !parts[1]) return null;
  try {
    const payload: unknown = JSON.parse(decodeBase64Url(parts[1]));
    if (
      payload === null ||
      typeof payload !== "object" ||
      Array.isArray(payload)
    )
      return null;
    return payload as Record<string, unknown>;
  } catch {
    return null;
  }
}

const isRole = (v: unknown): v is Role =>
  typeof v === "string" && (ROLE_IDS as readonly string[]).includes(v);

/**
 * Klaim yang dibutuhkan edge. Null bila token rusak atau peran tidak dikenal
 * (token semacam itu tidak mungkin diterbitkan API, jadi diperlakukan sebagai sesi tidak sah).
 */
export function readEdgeClaims(token: string | undefined): EdgeClaims | null {
  const payload = decodeJwtPayload(token);
  if (!payload || !isRole(payload.role)) return null;
  const { permissions, exp, wajib_ganti_password: wajib } = payload;
  return {
    role: payload.role,
    permissions:
      Array.isArray(permissions) &&
      permissions.every((p) => typeof p === "string")
        ? (permissions as string[])
        : null,
    exp: typeof exp === "number" && Number.isFinite(exp) ? exp : null,
    wajibGantiPassword: wajib === true,
  };
}

export function isExpired(
  claims: Pick<EdgeClaims, "exp">,
  nowSeconds: number,
): boolean {
  return claims.exp !== null && claims.exp <= nowSeconds;
}
