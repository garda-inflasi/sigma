/**
 * Pemetaan antara representasi API (snake_case) dan model UI (camelCase),
 * plus klien CRUD pengguna yang menembus BFF.
 */

import { bffFetch } from "./client";
import { namaWilayah } from "@/lib/data/generators";
import { ROLE_IDS, type Role } from "@/lib/rbac/roles";
import type { PenggunaRow } from "@/lib/data/types";
import { queryApiPengguna, type StateTabelPengguna } from "@/lib/pengguna/tabel";
import { toRiwayatPeran, type RiwayatPeran, type RiwayatPeranResponse } from "@/lib/pengguna/riwayat-peran";

/** Bentuk data pengguna sebagaimana dikirim backend Go. */
export interface UserResponse {
  id: string;
  nama: string;
  email: string;
  peran: string;
  lapis: string;
  lembaga: string;
  kabupaten_kode?: string;
  mfa_aktif: boolean;
  status: string;
  terakhir_masuk?: string;
  dibuat_pada: string;
  /** Akhir kunci login (lockout); hanya ada selama akun terkunci. */
  terkunci_sampai?: string;
}

export interface ListUsersResponse {
  items: UserResponse[];
  total: number;
  page: number;
  limit: number;
}

export interface BulkResult {
  berhasil: number;
  gagal: number;
  errors?: string[];
}

export interface RoleInfo {
  id: string;
  permissions: string[];
}

/** Katalog akses: seluruh izin dan izin tiap peran, sebagaimana dikenal API. */
export interface KatalogAkses {
  izin: string[];
  izinPeran: Record<string, ReadonlySet<string>>;
}

/** RoleInfo[] + daftar izin (API) → KatalogAkses (UI). */
export function toKatalogAkses(roles: RoleInfo[], permissions: { id: string }[]): KatalogAkses {
  return {
    izin: permissions.map((p) => p.id),
    izinPeran: Object.fromEntries(roles.map((r) => [r.id, new Set(r.permissions)])),
  };
}

const STATUSES = ["AKTIF", "NONAKTIF", "MENUNGGU"] as const;
type Status = (typeof STATUSES)[number];

function toRole(peran: string): Role {
  return (ROLE_IDS as readonly string[]).includes(peran)
    ? (peran as Role)
    : "OPS_MAHASISWA";
}

function toStatus(status: string): Status {
  return (STATUSES as readonly string[]).includes(status)
    ? (status as Status)
    : "MENUNGGU";
}

/** Format ISO 8601 → label relatif singkat untuk kolom tabel. */
function toTerakhirMasuk(iso?: string): string {
  if (!iso) return "";
  const then = new Date(iso).getTime();
  return Number.isNaN(then) ? "" : iso;
}

/** UserResponse (API) → PenggunaRow (UI). */
export function toPenggunaRow(u: UserResponse): PenggunaRow {
  return {
    id: u.id,
    nama: u.nama,
    email: u.email,
    role: toRole(u.peran),
    lembaga: u.lembaga,
    wilayah: u.kabupaten_kode ? namaWilayah(u.kabupaten_kode) : "Seluruh Aceh",
    status: toStatus(u.status),
    mfa: u.mfa_aktif,
    terakhirMasuk: toTerakhirMasuk(u.terakhir_masuk),
    terkunciSampai: u.terkunci_sampai,
    dibuatPada: u.dibuat_pada,
  };
}

/** True bila kunci login masih berlaku pada `sekarangMs` (kunci yang sudah lewat tidak ditampilkan). */
export function masihTerkunci(sampai: string | undefined, sekarangMs: number = Date.now()): boolean {
  if (!sampai) return false;
  const waktu = Date.parse(sampai);
  return !Number.isNaN(waktu) && waktu > sekarangMs;
}

/** Hasil POST /v1/users/{id}/buka-kunci. */
export interface BukaKunciResult {
  id: string;
  /** True bila akun memang sedang terkunci ketika dibuka. */
  terkunci_sebelumnya: boolean;
}

/** Pengguna lengkap dari GET /v1/users/{id}: baris daftar ditambah kolom yang tidak ditampilkan di tabel. */
export interface PenggunaDetail extends PenggunaRow {
  /** Kode kabupaten; kosong bila cakupannya seluruh Aceh. */
  kabupatenKode?: string;
  dibuatPada: string;
}

export function toPenggunaDetail(u: UserResponse): PenggunaDetail {
  return { ...toPenggunaRow(u), kabupatenKode: u.kabupaten_kode, dibuatPada: u.dibuat_pada };
}

/** Satu halaman daftar pengguna dari server beserta jumlah seluruh yang cocok dengan filter. */
export interface HalamanPengguna {
  rows: PenggunaRow[];
  total: number;
  page: number;
  limit: number;
}

/** Hasil POST /v1/users/{id}/reset-mfa. */
export interface ResetMfaResult {
  id: string;
  /** False bila pengguna memang tanpa MFA: tidak ada yang berubah dan sesinya tidak dicabut. */
  mfa_aktif_sebelumnya: boolean;
  /** Jumlah sesi pengguna yang diakhiri. */
  sesi_dicabut: number;
}

export interface CreatePenggunaInput {
  /** Opsional: kosong berarti API membuat UUID acak (pengguna.id adalah UUID). Form tidak mengirimnya. */
  id?: string;
  nama: string;
  email: string;
  password: string;
  peran: Role;
  lapis: string;
  lembaga: string;
}

export interface UpdatePenggunaInput {
  nama?: string;
  email?: string;
  peran?: Role;
  lapis?: string;
  lembaga?: string;
  kabupaten_kode?: string;
  status?: Status;
}

export type BulkAksi = "aktifkan" | "nonaktifkan" | "wajibkan_mfa" | "akhiri_sesi";

/** Ukuran halaman saat memuat daftar pengguna (batas maksimum API). */
export const HALAMAN_PENGGUNA = 100;

export function queryDaftarPengguna(page: number, search?: string): string {
  const qs = new URLSearchParams({ page: String(page), limit: String(HALAMAN_PENGGUNA) });
  if (search) qs.set("search", search);
  return qs.toString();
}

/** Masih ada halaman berikutnya? Halaman kosong menghentikan perulangan walau total tidak konsisten. */
export function lanjutHalamanPengguna(res: Pick<ListUsersResponse, "items" | "total" | "page" | "limit">): boolean {
  return res.items.length > 0 && res.page * res.limit < res.total;
}

/** Klien CRUD pengguna — semua panggilan melalui Route Handler BFF. */
export const penggunaApi = {
  /** Seluruh pengguna yang cocok (semua halaman): tabel menyaring dan memaginasi di klien. */
  list: async (params: { search?: string } = {}) => {
    const rows: PenggunaRow[] = [];
    for (let page = 1; ; page++) {
      const res = await bffFetch<ListUsersResponse>(`/api/pengguna?${queryDaftarPengguna(page, params.search)}`);
      rows.push(...res.items.map(toPenggunaRow));
      if (!lanjutHalamanPengguna(res)) return rows;
    }
  },

  /** Satu halaman pengguna: pencarian, filter, urutan, dan paginasi dijalankan server (GET /v1/users). */
  daftar: async (state: StateTabelPengguna): Promise<HalamanPengguna> => {
    const res = await bffFetch<ListUsersResponse>(`/api/pengguna?${queryApiPengguna(state)}`);
    return { rows: res.items.map(toPenggunaRow), total: res.total, page: res.page, limit: res.limit };
  },

  /** Satu pengguna dengan data segar dari server (GET /v1/users/{id}); 404 (ApiError) bila tidak ada. */
  get: async (id: string) => {
    const res = await bffFetch<UserResponse>(`/api/pengguna/${encodeURIComponent(id)}`);
    return toPenggunaDetail(res);
  },

  create: (input: CreatePenggunaInput) =>
    bffFetch<{ success: boolean }>("/api/pengguna", {
      method: "POST",
      body: JSON.stringify(input),
    }),

  update: async (id: string, input: UpdatePenggunaInput) => {
    const res = await bffFetch<UserResponse>(
      `/api/pengguna/${encodeURIComponent(id)}`,
      { method: "PATCH", body: JSON.stringify(input) }
    );
    return toPenggunaRow(res);
  },

  remove: (id: string) =>
    bffFetch<{ id: string }>(`/api/pengguna/${encodeURIComponent(id)}`, {
      method: "DELETE",
    }),

  resetPassword: (id: string, newPassword: string) =>
    bffFetch<{ id: string }>(
      `/api/pengguna/${encodeURIComponent(id)}/reset-password`,
      { method: "POST", body: JSON.stringify({ new_password: newPassword }) }
    ),

  /** Paksa keluar: mengakhiri semua sesi seorang pengguna (access token mereka langsung ditolak). */
  paksaKeluar: (id: string) =>
    bffFetch<{ id: string; dicabut: number }>(
      `/api/pengguna/${encodeURIComponent(id)}/revoke-sessions`,
      { method: "POST" }
    ),

  assignRole: async (id: string, peran: Role) => {
    const res = await bffFetch<UserResponse>(
      `/api/pengguna/${encodeURIComponent(id)}/roles`,
      { method: "PUT", body: JSON.stringify({ peran }) }
    );
    return toPenggunaRow(res);
  },

  /** Reset MFA oleh admin: MFA mati, rahasia dan kode cadangan dihapus, semua sesi pengguna berakhir. */
  resetMfa: (id: string) =>
    bffFetch<ResetMfaResult>(`/api/pengguna/${encodeURIComponent(id)}/reset-mfa`, { method: "POST" }),

  /** Riwayat perubahan peran satu pengguna (dari audit trail), terbaru dulu. */
  riwayatPeran: async (id: string): Promise<RiwayatPeran> => {
    const res = await bffFetch<RiwayatPeranResponse>(`/api/pengguna/${encodeURIComponent(id)}/riwayat-peran`);
    return toRiwayatPeran(res);
  },

  /** Membuka kunci login (lockout) satu pengguna; tercatat di audit. */
  bukaKunci: (id: string) =>
    bffFetch<BukaKunciResult>(
      `/api/pengguna/${encodeURIComponent(id)}/buka-kunci`,
      { method: "POST" }
    ),

  bulk: (aksi: BulkAksi, ids: string[]) =>
    bffFetch<BulkResult>("/api/pengguna/bulk", {
      method: "POST",
      body: JSON.stringify({ action: aksi, ids }),
    }),

  roles: () => bffFetch<RoleInfo[]>("/api/rbac/roles"),

  permissions: () => bffFetch<{ id: string }[]>("/api/rbac/permissions"),

  katalog: async (): Promise<KatalogAkses> => {
    const [roles, permissions] = await Promise.all([penggunaApi.roles(), penggunaApi.permissions()]);
    return toKatalogAkses(roles, permissions);
  },
};