/**
 * Mode server untuk DataTable: tabel yang menerima SATU halaman dari server (pencarian, urutan, filter, dan
 * paginasi dijalankan di sana). Berkas ini berisi tipe kontrak dan fungsi murni; komponennya ada di data-table.tsx.
 */

import type { SortingState } from "@tanstack/react-table";

/**
 * Kontrak mode server. Pemanggil memegang keadaan (mis. di URL) dan memuat halaman yang sesuai; DataTable
 * hanya menampilkan `data` (halaman ini), melaporkan interaksi, dan membersihkan pilihan baris tiap kali
 * `kunciKueri` berubah.
 *
 * Arti "pilih semua" di mode ini: HANYA baris di halaman yang tampil. Pilihan tidak pernah mencakup halaman lain
 * dan dibersihkan begitu halaman, urutan, pencarian, atau filter berubah, sehingga aksi massal tidak pernah
 * mengenai baris yang tidak terlihat.
 */
export interface ServerTable {
  /** Jumlah seluruh baris yang cocok dengan kueri (semua halaman). */
  total: number;
  /** Halaman yang tampil, mulai dari 1. */
  page: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  onPageSizeChange: (pageSize: number) => void;
  /** Kata kunci yang berlaku di server. */
  search: string;
  /** Dipanggil sesudah jeda ketik (atau Enter) dengan kata kunci yang sudah dipangkas. */
  onSearchChange: (search: string) => void;
  /** Jeda ketik sebelum kata kunci dikirim; bawaan 300 ms. */
  searchDelayMs?: number;
  /** Urutan yang berlaku (id kolom = nilai `sort` di API). */
  sorting: SortingState;
  /** Klik kepala kolom yang bisa diurutkan; pemanggil memutuskan siklus urutannya. */
  onSortToggle: (columnId: string) => void;
  /** Identitas kueri yang sedang tampil; pilihan baris dibersihkan bila berubah. */
  kunciKueri: string;
  /** True selama kueri baru dimuat (baris lama masih tampil sebagai placeholder). */
  isFetching?: boolean;
}

/** Jumlah halaman untuk `total` baris; minimal 1. */
export function jumlahHalamanTabel(total: number, pageSize: number): number {
  return Math.max(1, Math.ceil(Math.max(0, total) / Math.max(1, pageSize)));
}

/** Rentang baris yang tampil ("21–40 dari 90"): from 0 bila tidak ada baris. */
export function rentangBaris(
  total: number,
  page: number,
  pageSize: number,
): { from: number; to: number } {
  if (total <= 0) return { from: 0, to: 0 };
  const from = (Math.max(1, page) - 1) * pageSize + 1;
  return {
    from: Math.min(from, total),
    to: Math.min(total, Math.max(1, page) * pageSize),
  };
}

/**
 * Kalimat yang menjelaskan cakupan "pilih semua" di mode server, ditampilkan saat semua baris halaman terpilih
 * sementara masih ada baris di halaman lain; null bila tidak perlu (halaman belum terpilih semua, atau hanya satu halaman).
 */
export function catatanPilihanHalaman(
  terpilih: number,
  barisHalaman: number,
  total: number,
  entitas: string,
): string | null {
  if (barisHalaman <= 0 || terpilih < barisHalaman || total <= barisHalaman)
    return null;
  const lain = total - barisHalaman;
  const f = (n: number) => n.toLocaleString("id-ID");
  return `Seluruh ${f(barisHalaman)} ${entitas} di halaman ini terpilih. ${f(lain)} ${entitas} di halaman lain tidak ikut terpilih.`;
}
