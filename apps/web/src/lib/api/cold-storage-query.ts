/**
 * Parameter daftar cold storage. Modul murni tanpa dependensi server/browser:
 * dipakai klien browser (→ BFF) untuk menyusun URL, dan Route Handler BFF
 * (→ backend Go) untuk memvalidasi serta meneruskannya.
 */

export const COLD_STORAGE_SORTS = [
  "sisa_masa_simpan",
  "komoditas",
  "fasilitas",
  "volume",
  "masuk",
  "risiko",
  "pemilik",
] as const;
export type ColdStorageSort = (typeof COLD_STORAGE_SORTS)[number];

/** Batas yang sama dengan validasi di backend Go (dto.ColdStorageRequest). */
export const COLD_STORAGE_MAX_LIMIT = 100;
export const COLD_STORAGE_MAX_SEARCH = 100;
const MAX_FASILITAS_ID = 255;

export interface ColdStorageParams {
  /** Hanya stok pada fasilitas ini. */
  fasilitasId?: string;
  /** Cari pada nama komoditas atau pemilik. */
  q?: string;
  sort?: ColdStorageSort;
  desc?: boolean;
  page?: number;
  limit?: number;
}

/**
 * Menyusun query string dengan nama parameter kontrak API
 * (cold_storage_id, q, sort, order, page, limit). Nilai kosong dibuang.
 */
export function toQueryString(p: ColdStorageParams): string {
  const qs = new URLSearchParams();
  if (p.fasilitasId) qs.set("cold_storage_id", p.fasilitasId);
  const q = p.q?.trim();
  if (q) qs.set("q", q);
  if (p.sort) qs.set("sort", p.sort);
  if (p.desc !== undefined) qs.set("order", p.desc ? "desc" : "asc");
  if (p.page) qs.set("page", String(p.page));
  if (p.limit) qs.set("limit", String(p.limit));
  return qs.toString();
}

export type ParseResult =
  { ok: true; params: ColdStorageParams } | { ok: false; message: string };

const INTEGER = /^\d+$/;

/**
 * Memvalidasi query string permintaan BFF. Hanya parameter yang dikenal yang
 * diteruskan ke backend; yang lain diabaikan.
 */
export function parseColdStorageQuery(sp: URLSearchParams): ParseResult {
  const params: ColdStorageParams = {};

  const fasilitasId = sp.get("cold_storage_id")?.trim();
  if (fasilitasId) {
    if (fasilitasId.length > MAX_FASILITAS_ID)
      return { ok: false, message: "ID fasilitas terlalu panjang." };
    params.fasilitasId = fasilitasId;
  }

  const q = sp.get("q")?.trim();
  if (q) {
    if (q.length > COLD_STORAGE_MAX_SEARCH) {
      return {
        ok: false,
        message: `Kata kunci maksimal ${COLD_STORAGE_MAX_SEARCH} karakter.`,
      };
    }
    params.q = q;
  }

  const sort = sp.get("sort");
  if (sort) {
    if (!(COLD_STORAGE_SORTS as readonly string[]).includes(sort))
      return { ok: false, message: "Kolom pengurutan tidak dikenal." };
    params.sort = sort as ColdStorageSort;
  }

  const order = sp.get("order");
  if (order) {
    if (order !== "asc" && order !== "desc")
      return { ok: false, message: "Arah pengurutan harus asc atau desc." };
    params.desc = order === "desc";
  }

  const page = sp.get("page");
  if (page) {
    if (!INTEGER.test(page) || Number(page) < 1)
      return { ok: false, message: "Halaman harus bilangan bulat positif." };
    params.page = Number(page);
  }

  const limit = sp.get("limit");
  if (limit) {
    if (
      !INTEGER.test(limit) ||
      Number(limit) < 1 ||
      Number(limit) > COLD_STORAGE_MAX_LIMIT
    ) {
      return {
        ok: false,
        message: `Jumlah per halaman harus 1–${COLD_STORAGE_MAX_LIMIT}.`,
      };
    }
    params.limit = Number(limit);
  }

  return { ok: true, params };
}

/** Aksi massal yang diterima backend dan izin yang dibutuhkan masing-masing. */
export const COLD_STORAGE_BULK_ACTIONS = [
  "rencanakan_distribusi",
  "rekomendasi_olahan",
] as const;
export type ColdStorageBulkAction = (typeof COLD_STORAGE_BULK_ACTIONS)[number];

/** Batas jumlah lot dalam satu aksi massal (sama dengan backend). */
export const COLD_STORAGE_MAX_BULK_IDS = 500;
