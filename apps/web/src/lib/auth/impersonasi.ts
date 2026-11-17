/**
 * Impersonasi pada sesi web: membaca klaim `imp` access token menjadi metadata TAMPILAN (banner, chip kembali, menu).
 * Murni (tanpa import Next/Node) sehingga aman di server, klien, dan tes. Otorisasi tidak pernah bergantung pada
 * klaim ini: API Go memverifikasi tanda tangan, mencocokkan catatan `impersonasi_sesi`, dan menegakkan izin akun
 * sasaran pada setiap request (docs/impersonasi.md).
 *
 * Bentuk klaim (kontrak): `imp: { id, by: { user_id, name, email, role }, started_at detik epoch }`.
 * `sub`, `name`, dan `role` token adalah milik akun SASARAN.
 */

import { ROLE_IDS, type Role } from "@/lib/rbac/roles";

export interface PenggunaAsal {
  id: string;
  name: string;
  email: string;
  role: Role;
}

export interface PenggunaSasaran {
  id: string;
  name: string;
  role: Role;
}

/** Impersonasi yang sedang berjalan, diturunkan dari klaim token (hanya tampilan). */
export interface ImpersonasiSesi {
  active: true;
  originalUser: PenggunaAsal;
  targetUser: PenggunaSasaran;
  /** ISO 8601. */
  startedAt: string;
}

const adalahPeran = (v: unknown): v is Role =>
  typeof v === "string" && (ROLE_IDS as readonly string[]).includes(v);

const teksTakKosong = (v: unknown): v is string =>
  typeof v === "string" && v.trim() !== "";

const adalahObjek = (v: unknown): v is Record<string, unknown> =>
  v !== null && typeof v === "object" && !Array.isArray(v);

/**
 * Metadata impersonasi dari muatan JWT yang SUDAH terverifikasi (atau, di edge, dibaca tanpa verifikasi).
 * Klaim yang tidak ada atau cacat dalam bentuk apa pun menghasilkan undefined (sesi biasa), tidak pernah galat:
 * banner yang salah tampil lebih buruk daripada tidak tampil, dan API tetap otoritatif.
 */
export function impersonasiDariKlaim(
  payload: Record<string, unknown>,
): ImpersonasiSesi | undefined {
  const imp = payload.imp;
  if (!adalahObjek(imp)) return undefined;
  const by = imp.by;
  if (!adalahObjek(by)) return undefined;
  const mulai = imp.started_at;
  if (
    !teksTakKosong(imp.id) ||
    !teksTakKosong(by.user_id) ||
    !teksTakKosong(by.name) ||
    !teksTakKosong(by.email) ||
    !adalahPeran(by.role) ||
    typeof mulai !== "number" ||
    !Number.isFinite(mulai) ||
    mulai <= 0
  ) {
    return undefined;
  }
  // Sasaran = pemilik token. Tanpa sub dan peran yang sah token ini bukan sesi yang dapat dipakai.
  if (!teksTakKosong(payload.sub) || !adalahPeran(payload.role)) return undefined;
  const tanggal = new Date(mulai * 1000);
  if (Number.isNaN(tanggal.getTime())) return undefined;

  return {
    active: true,
    originalUser: {
      id: by.user_id,
      name: by.name,
      email: by.email,
      role: by.role,
    },
    targetUser: {
      id: payload.sub,
      name: typeof payload.name === "string" ? payload.name : "",
      role: payload.role,
    },
    startedAt: tanggal.toISOString(),
  };
}
