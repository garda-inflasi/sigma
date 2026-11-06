/**
 * Audit trail: pemetaan respons API Go (snake_case) ke model UI, kueri filter
 * sisi server dengan paginasi cursor, dan padanannya untuk mode mock.
 */

import { ROLES, type Role } from "@/lib/rbac/roles";
import type { AuditDetail, AuditPerubahan, AuditRow } from "@/lib/data/types";
import { API_BASE, ApiError } from "./client";
import { fetchSesi } from "@/lib/auth/sesi-client";

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
  metadata?: Record<string, unknown>;
}

/** `GET /v1/audit/:id`: baris audit beserta diff sebelum/sesudah. */
export interface AuditDetailResponse extends AuditLogResponse {
  perubahan: AuditPerubahan[];
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
  /** ID objek persis (jejak satu objek). */
  entitas_id?: string;
  /** Semua baris dari satu permintaan. */
  correlation_id?: string;
  /** Pencarian bebas pada aksi, entitas, id entitas, aktor, dan correlation id. */
  q?: string;
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
  "UPDATE_ROLE_PERMISSIONS",
  "GRANT_USER_PERMISSION",
  "REVOKE_USER_PERMISSION",
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

/** AuditDetailResponse (API) → AuditDetail (UI). */
export function toAuditDetail(a: AuditDetailResponse): AuditDetail {
  return {
    ...toAuditRow(a),
    userAgent: a.user_agent ?? "",
    correlationId: a.correlation_id ?? "",
    email: a.aktor_email ?? "",
    metadata: a.metadata,
    perubahan: a.perubahan,
  };
}

/** Detail mode mock: baris yang sama tanpa keadaan sebelum/sesudah. */
export function toMockAuditDetail(row: AuditRow): AuditDetail {
  return { ...row, userAgent: "", correlationId: "", email: "", perubahan: [] };
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

/** Query string `GET /v1/audit/export`: hanya filter (server mengabaikan limit/cursor). */
export function toExportQuery(filter: AuditFilter): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(filter)) {
    if (value) params.set(key, value);
  }
  const query = params.toString();
  return query ? `?${query}` : "";
}

export interface HasilEkspor {
  baris: number;
  /** Server memotong hasil pada batas baris; persempit filter. */
  terpotong: boolean;
}

/** Mengunduh CSV audit sesuai filter dari server dan memicu simpan berkas. */
export async function unduhEksporAudit(
  filter: AuditFilter,
): Promise<HasilEkspor> {
  const res = await fetchSesi(
    `${API_BASE}/v1/audit/export${toExportQuery(filter)}`,
    {
      credentials: "include",
      headers: { Accept: "text/csv" },
    },
  );
  if (!res.ok) {
    let pesan = `Ekspor gagal (${res.status})`;
    try {
      const payload = (await res.json()) as { message?: string };
      pesan = payload.message ?? pesan;
    } catch {}
    throw new ApiError(res.status, pesan);
  }
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = Object.assign(document.createElement("a"), {
    href: url,
    download: `audit-trail-${new Date().toISOString().slice(0, 10)}.csv`,
  });
  a.click();
  URL.revokeObjectURL(url);
  return {
    baris: Number(res.headers.get("X-Export-Rows") ?? 0),
    terpotong: res.headers.get("X-Export-Truncated") === "true",
  };
}

/** Halaman daftar yang memuat objek yang dirujuk entri audit (tanpa tautan ke satu baris). */
const HALAMAN_OBJEK: Record<string, string> = {
  pengguna: "/pengguna",
  entriharga: "/data/harga",
  ewsalert: "/ews",
  aturanews: "/ews",
  intervensi: "/intervensi",
  komoditas: "/master/komoditas",
  pasar: "/master/pasar",
  wilayah: "/master/wilayah",
  stokcoldstorage: "/cold-storage",
  policybrief: "/policy-brief",
  sumberdata: "/sumber-data",
};

/**
 * Halaman tempat objek entri audit dikelola, atau undefined bila tak dikenal.
 * Nama entitas dinormalkan (huruf kecil tanpa pemisah) karena backend memakai
 * `entri_harga` sedangkan data mock memakai `EntriHarga`.
 */
export function halamanObjekAudit(entitas: string): string | undefined {
  return HALAMAN_OBJEK[entitas.toLowerCase().replace(/[^a-z0-9]/g, "")];
}

/** Menampilkan nilai JSON diff sebagai teks ringkas. */
export function formatNilaiAudit(nilai: unknown): string {
  if (nilai === undefined) return "";
  return typeof nilai === "string" ? nilai : JSON.stringify(nilai);
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
  const q = filter.q?.trim().toLowerCase();
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
        (!filter.hasil || r.hasil === filter.hasil) &&
        (!filter.entitas_id || r.entitasId === filter.entitas_id) &&
        (!q ||
          [r.aksi, r.entitas, r.entitasId, r.aktor].some((v) =>
            v.toLowerCase().includes(q),
          ))
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
