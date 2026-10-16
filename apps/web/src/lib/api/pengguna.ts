/**
 * Pemetaan antara representasi API (snake_case) dan model UI (camelCase),
 * plus klien CRUD pengguna yang menembus BFF.
 */

import { bffFetch } from "./client";
import { namaWilayah } from "@/lib/data/generators";
import { ROLE_IDS, type Role } from "@/lib/rbac/roles";
import type { PenggunaRow } from "@/lib/data/types";

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
  };
}

export interface CreatePenggunaInput {
  id: string;
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

/** Klien CRUD pengguna — semua panggilan melalui Route Handler BFF. */
export const penggunaApi = {
  list: async (params: { page?: number; limit?: number; search?: string } = {}) => {
    const qs = new URLSearchParams();
    if (params.page) qs.set("page", String(params.page));
    if (params.limit) qs.set("limit", String(params.limit));
    if (params.search) qs.set("search", params.search);
    const query = qs.toString();

    const res = await bffFetch<ListUsersResponse>(
      `/api/pengguna${query ? `?${query}` : ""}`
    );
    return res.items.map(toPenggunaRow);
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

  assignRole: async (id: string, peran: Role) => {
    const res = await bffFetch<UserResponse>(
      `/api/pengguna/${encodeURIComponent(id)}/roles`,
      { method: "PUT", body: JSON.stringify({ peran }) }
    );
    return toPenggunaRow(res);
  },

  bulk: (aksi: BulkAksi, ids: string[]) =>
    bffFetch<BulkResult>("/api/pengguna/bulk", {
      method: "POST",
      body: JSON.stringify({ action: aksi, ids }),
    }),

  roles: () => bffFetch<RoleInfo[]>("/api/rbac/roles"),

  permissions: () => bffFetch<{ id: string }[]>("/api/rbac/permissions"),
};