/**
 * Audit trail: pemetaan respons API Go (snake_case) ke model UI, kueri filter
 * sisi server dengan paginasi cursor, dan padanannya untuk mode mock.
 */

import { ROLES, type Role } from "@/lib/rbac/roles";
import type { AuditRow } from "@/lib/data/types";

/** Satu baris audit sebagaimana dikirim backend Go. */
export interface AuditLogResponse {
  id: string;
  waktu: string;
  aktor_id?: string;
  aktor_nama?: string;
  aktor_email?: string;
  aktor_peran?: string;
  aksi: string;
  entitas: string;
  entitas_id: string;
  hasil: string;
  ip?: string;
  user_agent?: string;
  correlation_id?: string;
}

/** Satu halaman `GET /v1/audit`; `next_cursor` kosong pada halaman terakhir. */
export interface AuditPageResponse {
  items: AuditLogResponse[];
  next_cursor?: string;
}

/** Filter yang dikirim ke server. `dari`/`sampai` berformat YYYY-MM-DD (sampai inklusif). */
export interface AuditFilter {
  dari?: string;
  sampai?: string;
  aktor?: string;
  aksi?: string;
  entitas?: string;
  hasil?: AuditRow["hasil"];
}

export interface AuditPage {
  rows: AuditRow[];
  nextCursor?: string;
}

export const AUDIT_PAGE_SIZE = 50;

/** Aksi yang dicatat backend; dipakai sebagai pilihan filter. */
export const AUDIT_AKSI = [
  "LOGIN",
  "LOGOUT",
  "CHANGE_PASSWORD",
  "RESET_PASSWORD",
  "RESET_PASSWORD_ADMIN",
  "CREATE_USER",
  "UPDATE_USER",
  "DELETE_USER",
  "ASSIGN_ROLE",
  "BULK_aktifkan",
  "BULK_nonaktifkan",
  "BULK_wajibkan_mfa",
  "BULK_akhiri_sesi",
  "BULK_rencanakan_distribusi",
  "BULK_rekomendasi_olahan",
  "VALIDASI_HARGA",
  "PUBLIKASI_HARGA",
  "AKUI_EWS",
  "PINDAH_TAHAP",
  "SETUJUI_INTERVENSI",
  "UBAH_AMBANG_EWS",
  "EKSPOR_DATA",
  "AJUKAN_MASTER",
] as const;

const isRole = (peran: string): peran is Role => peran in ROLES;

/** AuditLogResponse (API) → AuditRow (UI). */
export function toAuditRow(a: AuditLogResponse): AuditRow {
  return {
    id: a.id,
    waktu: a.waktu,
    aktor: a.aktor_nama ?? a.aktor_email ?? "Sistem",
    peran: a.aktor_peran
      ? isRole(a.aktor_peran)
        ? ROLES[a.aktor_peran].label
        : a.aktor_peran
      : "",
    aksi: a.aksi,
    entitas: a.entitas,
    entitasId: a.entitas_id,
    hasil: a.hasil === "GAGAL" || a.hasil === "DITOLAK" ? a.hasil : "BERHASIL",
    ip: a.ip ?? "",
  };
}

export function toAuditPage(res: AuditPageResponse): AuditPage {
  return {
    rows: res.items.map(toAuditRow),
    nextCursor: res.next_cursor || undefined,
  };
}

/** Query string `GET /v1/audit` dari filter dan cursor; nilai kosong dibuang. */
export function toAuditQuery(
  filter: AuditFilter,
  cursor?: string,
  limit = AUDIT_PAGE_SIZE,
): string {
  const params = new URLSearchParams({ limit: String(limit) });
  for (const [key, value] of Object.entries({ ...filter, cursor })) {
    if (value) params.set(key, value);
  }
  return `?${params}`;
}

/**
 * Padanan sisi klien dari filter + paginasi server untuk mode mock. Cursor
 * mock adalah posisi baris berikutnya pada daftar terurut menurun.
 */
export function queryMockAudit(
  all: AuditRow[],
  filter: AuditFilter,
  cursor?: string,
  limit = AUDIT_PAGE_SIZE,
): AuditPage {
  const aktor = filter.aktor?.trim().toLowerCase();
  const dari = filter.dari ? Date.parse(`${filter.dari}T00:00:00`) : undefined;
  const sampai = filter.sampai
    ? Date.parse(`${filter.sampai}T00:00:00`) + 24 * 3_600_000
    : undefined;

  const cocok = all
    .filter((r) => {
      const waktu = Date.parse(r.waktu);
      return (
        (dari === undefined || waktu >= dari) &&
        (sampai === undefined || waktu < sampai) &&
        (!aktor || r.aktor.toLowerCase().includes(aktor)) &&
        (!filter.aksi || r.aksi === filter.aksi) &&
        (!filter.entitas || r.entitas === filter.entitas) &&
        (!filter.hasil || r.hasil === filter.hasil)
      );
    })
    .sort((a, b) => Date.parse(b.waktu) - Date.parse(a.waktu));

  const start = cursor ? Number(cursor) : 0;
  const end = start + limit;
  return {
    rows: cocok.slice(start, end),
    nextCursor: end < cocok.length ? String(end) : undefined,
  };
}
