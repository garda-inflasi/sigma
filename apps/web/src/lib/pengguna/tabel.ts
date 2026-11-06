/**
 * Keadaan tabel /pengguna yang dijalankan server: pencarian, filter, urutan, dan halaman. Satu bentuk dipakai
 * di tiga tempat supaya tidak pernah menyimpang: URL halaman (reload dan tombol kembali memulihkan tabel),
 * kueri ke API (`GET /v1/users`, lewat BFF), dan kunci cache React Query.
 *
 * Semua fungsi murni. Penguraian URL longgar (nilai asing dibuang, bukan galat) karena URL bisa disunting
 * pengguna; kueri ke API selalu dibangun dari keadaan yang sudah tersaring.
 */

import { ROLE_IDS } from "@/lib/rbac/roles";

/** Kolom urutan yang diterima API (daftar putih yang sama dengan `domain.KolomUrutPengguna` di Go). */
export const KOLOM_URUT = [
  "nama",
  "email",
  "peran",
  "lembaga",
  "status",
  "terakhir_masuk",
  "dibuat_pada",
] as const;
export type KolomUrut = (typeof KOLOM_URUT)[number];
export type ArahUrut = "asc" | "desc";

export const UKURAN_HALAMAN = [10, 25, 50, 100] as const;
export const UKURAN_BAWAAN = 25;
/** Batas panjang kata kunci di API (400 bila lebih). */
export const MAKS_PANJANG_CARI = 100;

export const STATUS_PENGGUNA = ["AKTIF", "NONAKTIF", "MENUNGGU"] as const;
export const LAPIS_PENGGUNA = [
  "STRATEGIS",
  "MANAJERIAL",
  "OPERASIONAL",
  "SISTEM",
] as const;
export type FilterBool = "true" | "false";

export interface FilterPengguna {
  /** Peran persis (id enum). */
  peran?: string;
  status?: string;
  lapis?: string;
  /** "true" = MFA aktif, "false" = belum aktif. */
  mfa?: FilterBool;
  /** "true" = login sedang terkunci (lockout), "false" = tidak. */
  terkunci?: FilterBool;
}

export const KUNCI_FILTER = [
  "peran",
  "status",
  "lapis",
  "mfa",
  "terkunci",
] as const;
export type KunciFilter = (typeof KUNCI_FILTER)[number];

export interface StateTabelPengguna extends FilterPengguna {
  /** Kata kunci yang berlaku (sudah lewat jeda ketik). */
  search: string;
  /** Kolom urutan; kosong = urutan bawaan server (terbaru dibuat dulu). */
  sort?: KolomUrut;
  /** Hanya bermakna bila `sort` ada. */
  arah?: ArahUrut;
  /** Mulai dari 1. */
  page: number;
  limit: number;
}

export const STATE_AWAL: StateTabelPengguna = {
  search: "",
  page: 1,
  limit: UKURAN_BAWAAN,
};

/** Sumber nilai kueri: `URLSearchParams` dan `ReadonlyURLSearchParams` (Next) keduanya cocok. */
export interface SumberParams {
  get(name: string): string | null;
}

const dalam = <T extends string>(
  daftar: readonly T[],
  nilai: string | null,
): T | undefined =>
  nilai !== null && (daftar as readonly string[]).includes(nilai)
    ? (nilai as T)
    : undefined;

const boolFilter = (nilai: string | null): FilterBool | undefined =>
  nilai === "true" || nilai === "false" ? nilai : undefined;

function bilanganPositif(nilai: string | null): number | undefined {
  if (nilai === null || !/^\d+$/.test(nilai)) return undefined;
  const n = Number(nilai);
  return Number.isSafeInteger(n) && n >= 1 ? n : undefined;
}

/** Keadaan tabel dari kueri URL; nilai tak dikenal diabaikan (kembali ke bawaan). */
export function parseStateTabel(params: SumberParams): StateTabelPengguna {
  const sort = dalam(KOLOM_URUT, params.get("sort"));
  const limit = bilanganPositif(params.get("limit"));
  const state: StateTabelPengguna = {
    search: (params.get("search") ?? "").trim().slice(0, MAKS_PANJANG_CARI),
    page: bilanganPositif(params.get("page")) ?? 1,
    limit: (UKURAN_HALAMAN as readonly number[]).includes(limit ?? -1)
      ? (limit as number)
      : UKURAN_BAWAAN,
  };
  const peran = dalam(ROLE_IDS, params.get("peran"));
  const status = dalam(STATUS_PENGGUNA, params.get("status"));
  const lapis = dalam(LAPIS_PENGGUNA, params.get("lapis"));
  const mfa = boolFilter(params.get("mfa"));
  const terkunci = boolFilter(params.get("terkunci"));
  if (peran) state.peran = peran;
  if (status) state.status = status;
  if (lapis) state.lapis = lapis;
  if (mfa) state.mfa = mfa;
  if (terkunci) state.terkunci = terkunci;
  if (sort) {
    state.sort = sort;
    state.arah = dalam(["asc", "desc"] as const, params.get("arah")) ?? "asc";
  }
  return state;
}

/**
 * Parameter kueri dari keadaan tabel, dalam urutan tetap. `lengkap` false (URL halaman) membuang nilai bawaan
 * supaya URL pendek; `lengkap` true (API) selalu menyertakan page dan limit.
 */
function susunParams(
  state: StateTabelPengguna,
  lengkap: boolean,
): URLSearchParams {
  const qs = new URLSearchParams();
  const cari = state.search.trim();
  if (cari) qs.set("search", cari);
  for (const kunci of KUNCI_FILTER) {
    const nilai = state[kunci];
    if (nilai) qs.set(kunci, nilai);
  }
  if (state.sort) {
    qs.set("sort", state.sort);
    qs.set("arah", state.arah ?? "asc");
  }
  if (lengkap || state.page > 1) qs.set("page", String(state.page));
  if (lengkap || state.limit !== UKURAN_BAWAAN)
    qs.set("limit", String(state.limit));
  return qs;
}

/** Kueri URL halaman /pengguna (tanpa "?"); kosong bila semuanya bawaan. */
export function queryUrlPengguna(state: StateTabelPengguna): string {
  return susunParams(state, false).toString();
}

/** Kueri `GET /api/pengguna` (BFF → `GET /v1/users`). */
export function queryApiPengguna(state: StateTabelPengguna): string {
  return susunParams(state, true).toString();
}

/** Kunci cache React Query: berubah bila dan hanya bila hasil yang diminta berubah. */
export function kunciCachePengguna(state: StateTabelPengguna): string {
  return queryApiPengguna(state);
}

// ── Transisi keadaan ──────────────────────────────────────────────────

/** Mengubah kata kunci; kembali ke halaman 1. */
export function ubahCari(
  state: StateTabelPengguna,
  search: string,
): StateTabelPengguna {
  return {
    ...state,
    search: search.trim().slice(0, MAKS_PANJANG_CARI),
    page: 1,
  };
}

/** Mengubah satu filter (undefined = semua); kembali ke halaman 1. */
export function ubahFilter(
  state: StateTabelPengguna,
  kunci: KunciFilter,
  nilai: string | undefined,
): StateTabelPengguna {
  const berikut: StateTabelPengguna = { ...state, page: 1 };
  if (nilai === undefined || nilai === "") delete berikut[kunci];
  else Object.assign(berikut, { [kunci]: nilai });
  return berikut;
}

/** Membuang semua filter (kata kunci dan urutan tetap); kembali ke halaman 1. */
export function resetFilter(state: StateTabelPengguna): StateTabelPengguna {
  const berikut: StateTabelPengguna = { ...state, page: 1 };
  for (const kunci of KUNCI_FILTER) delete berikut[kunci];
  return berikut;
}

export function ubahHalaman(
  state: StateTabelPengguna,
  page: number,
): StateTabelPengguna {
  return { ...state, page: Math.max(1, Math.trunc(page) || 1) };
}

export function ubahUkuran(
  state: StateTabelPengguna,
  limit: number,
): StateTabelPengguna {
  const sah = (UKURAN_HALAMAN as readonly number[]).includes(limit)
    ? limit
    : UKURAN_BAWAAN;
  return { ...state, limit: sah, page: 1 };
}

/**
 * Klik pada kepala kolom: kolom lain → naik; naik → turun; turun → kembali ke urutan bawaan server.
 * Kembali ke halaman 1.
 */
export function ubahUrut(
  state: StateTabelPengguna,
  kolom: KolomUrut,
): StateTabelPengguna {
  const berikut: StateTabelPengguna = { ...state, page: 1 };
  if (state.sort !== kolom) {
    berikut.sort = kolom;
    berikut.arah = "asc";
  } else if ((state.arah ?? "asc") === "asc") {
    berikut.arah = "desc";
  } else {
    delete berikut.sort;
    delete berikut.arah;
  }
  return berikut;
}

/** Urutan sebagai SortingState TanStack (kosong = urutan bawaan server). */
export function toSortingState(state: StateTabelPengguna): { id: string; desc: boolean }[] {
  return state.sort ? [{ id: state.sort, desc: state.arah === "desc" }] : [];
}

/** Apakah `id` kolom adalah kolom urutan yang diterima API. */
export function adalahKolomUrut(id: string): id is KolomUrut {
  return (KOLOM_URUT as readonly string[]).includes(id);
}

/** Jumlah filter yang aktif (kata kunci dan urutan tidak dihitung). */
export function jumlahFilterAktif(state: StateTabelPengguna): number {
  return KUNCI_FILTER.filter((k) => state[k] !== undefined).length;
}

/** Banyaknya halaman untuk `total` baris; minimal 1. */
export function jumlahHalaman(total: number, limit: number): number {
  return Math.max(1, Math.ceil(Math.max(0, total) / Math.max(1, limit)));
}

/**
 * Halaman yang seharusnya ditampilkan bila `page` melewati halaman terakhir (mis. setelah baris di halaman
 * terakhir dihapus atau filter dipersempit dari tab lain); null bila halaman sudah sah.
 */
export function koreksiHalaman(
  state: StateTabelPengguna,
  total: number,
): number | null {
  if (total <= 0) return null; // tanpa baris tidak ada halaman yang lebih baik; tampilkan keadaan kosong
  const terakhir = jumlahHalaman(total, state.limit);
  return state.page > terakhir ? terakhir : null;
}
