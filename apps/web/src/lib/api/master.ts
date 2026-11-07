/**
 * Pemetaan master data dari representasi API Go (snake_case) ke model UI.
 * Endpoint: GET /v1/master/{komoditas,wilayah,pasar} dan POST /v1/master/komoditas/bulk.
 */

import type {
  MasterKomoditasRow,
  MasterPasarRow,
  MasterWilayahRow,
} from "@/lib/data/types";
import { formatWilayah } from "./cold-storage";
import {
  queryString,
  toPerubahanMaster,
  toRiwayatVersi,
  type PerubahanMaster,
  type PerubahanMasterResponse,
  type RiwayatVersi,
  type RiwayatVersiResponse,
} from "./master-perubahan";

export interface MasterKomoditasResponse {
  kode: string;
  nama: string;
  kategori: string;
  satuan: string;
  harga_acuan: number;
  pilot: boolean;
  shelf_life_hari: number;
  versi: number;
  status: string;
}

export interface MasterWilayahResponse {
  kode: string;
  nama: string;
  jenis: string;
  ibukota: string;
  lat: number;
  lng: number;
  provinsi_kode: string;
}

export interface MasterPasarResponse {
  id: string;
  nama: string;
  kabupaten_kode: string;
  lat: number;
  lng: number;
  radius_meter: number;
  aktif: boolean;
  /** Baru sejak 04-B; server lama tidak mengirimnya. */
  kecamatan_kode?: string | null;
  kecamatan_nama?: string | null;
  versi?: number;
}

const STATUS = ["AKTIF", "MENUNGGU_PERSETUJUAN", "DIARSIPKAN"] as const;

function toStatus(status: string): MasterKomoditasRow["status"] {
  return (STATUS as readonly string[]).includes(status)
    ? (status as MasterKomoditasRow["status"])
    : "AKTIF";
}

export function toMasterKomoditasRow(
  k: MasterKomoditasResponse,
): MasterKomoditasRow {
  return {
    id: k.kode,
    kode: k.kode,
    nama: k.nama,
    kategori: k.kategori,
    satuan: k.satuan,
    hargaAcuan: k.harga_acuan,
    pilot: k.pilot,
    shelfLifeHari: k.shelf_life_hari,
    versi: k.versi,
    status: toStatus(k.status),
  };
}

export function toMasterWilayahRow(w: MasterWilayahResponse): MasterWilayahRow {
  return {
    id: w.kode,
    kode: w.kode,
    nama: formatWilayah(w.nama),
    ibukota: w.ibukota,
    lat: w.lat,
    lng: w.lng,
  };
}

/** `namaKabupaten` memetakan kode kabupaten → nama untuk kolom kabupaten/kota pasar. */
export function toMasterPasarRow(
  p: MasterPasarResponse,
  namaKabupaten: ReadonlyMap<string, string>,
): MasterPasarRow {
  return {
    id: p.id,
    nama: p.nama,
    kabupaten: namaKabupaten.get(p.kabupaten_kode) ?? p.kabupaten_kode,
    kabupatenKode: p.kabupaten_kode,
    lat: p.lat,
    lng: p.lng,
    radiusMeter: p.radius_meter,
    aktif: p.aktif,
    kecamatanKode: p.kecamatan_kode ?? null,
    kecamatanNama: p.kecamatan_nama ?? null,
    versi: p.versi ?? 1,
  };
}

/** Aksi massal master komoditas (POST /v1/master/komoditas/bulk). */
export type MasterBulkAksi = "ajukan_perubahan" | "setujui" | "arsipkan";

export interface MasterBulkResponse {
  berhasil: number;
  gagal: number;
  rincian: { kode: string; berhasil: boolean; pesan?: string }[];
}

/** Hasil aksi massal: jumlah dan tiap kode yang ditolak server beserta alasannya. */
export interface MasterBulkHasil {
  berhasil: number;
  gagal: number;
  ditolak: { kode: string; pesan: string }[];
}

export function toMasterBulkHasil(r: MasterBulkResponse): MasterBulkHasil {
  return {
    berhasil: r.berhasil,
    gagal: r.gagal,
    ditolak: (r.rincian ?? []).filter((x) => !x.berhasil).map((x) => ({ kode: x.kode, pesan: x.pesan ?? "Ditolak" })),
  };
}

/** Teks "KODE: alasan" untuk paling banyak `maks` kode yang ditolak, sisanya diringkas. */
export function ringkasDitolak(ditolak: MasterBulkHasil["ditolak"], maks = 3): string {
  const tampil = ditolak.slice(0, maks).map((d) => `${d.kode}: ${d.pesan}`);
  if (ditolak.length > maks) tampil.push(`dan ${ditolak.length - maks} lainnya`);
  return tampil.join("; ");
}

/** Kata kerja lampau tiap aksi, untuk pesan hasil. */
export const MASTER_BULK_LABEL: Record<MasterBulkAksi, string> = {
  ajukan_perubahan: "diajukan",
  setujui: "disetujui",
  arsipkan: "diarsipkan",
};

/* ── Daftar berhalaman (X-Total-Count) ──────────────────────── */

/** Satu halaman daftar beserta jumlah seluruh baris yang cocok. */
export interface Halaman<T> {
  rows: T[];
  total: number;
}

/**
 * Halaman dari respons server. Tanpa header total (server lama) total diperkirakan dari halaman ini: cukup
 * untuk menampilkan baris yang ada dan menonaktifkan "berikutnya" saat halaman tidak penuh.
 */
export function toHalaman<R, T>(
  data: R[],
  total: number | null,
  page: number,
  limit: number,
  petakan: (r: R) => T,
): Halaman<T> {
  const rows = data.map(petakan);
  const perkiraan = (Math.max(1, page) - 1) * limit + rows.length + (rows.length >= limit ? 1 : 0);
  return { rows, total: total ?? perkiraan };
}

/* ── Kecamatan, satuan, pedagang ────────────────────────────── */

export interface KecamatanResponse {
  kode: string;
  nama: string;
  kabupaten_kode: string;
  kabupaten_nama: string;
  kode_dagri?: string | null;
}

export interface KecamatanRow {
  id: string;
  kode: string;
  nama: string;
  kabupatenKode: string;
  /** Nama resmi dari server (mis. "KOTA BANDA ACEH") diringkas seperti kolom wilayah. */
  kabupatenNama: string;
  kodeDagri: string | null;
}

export function toKecamatanRow(k: KecamatanResponse): KecamatanRow {
  return {
    id: k.kode,
    kode: k.kode,
    nama: k.nama,
    kabupatenKode: k.kabupaten_kode,
    kabupatenNama: formatWilayah(k.kabupaten_nama),
    kodeDagri: k.kode_dagri ?? null,
  };
}

export interface KecamatanParams {
  kabupatenKode?: string;
  q?: string;
  page?: number;
  limit?: number;
}

export function toKecamatanQuery(p: KecamatanParams = {}): string {
  return queryString({ kabupaten_kode: p.kabupatenKode, q: p.q?.trim(), page: p.page, limit: p.limit });
}

export interface SatuanResponse {
  kode: string;
  nama: string;
  jumlah_komoditas: number;
}

export interface SatuanRow {
  id: string;
  kode: string;
  nama: string;
  jumlahKomoditas: number;
}

export function toSatuanRow(s: SatuanResponse): SatuanRow {
  return { id: s.kode, kode: s.kode, nama: s.nama, jumlahKomoditas: s.jumlah_komoditas };
}

export interface PedagangResponse {
  id: string | number;
  pasar_id: string;
  pasar_nama: string;
  kode: string;
  nama?: string | null;
  aktif: boolean;
  jumlah_entri: number;
}

export interface PedagangRow {
  id: string;
  pasarId: string;
  pasarNama: string;
  kode: string;
  /** null = pedagang belum diberi nama (hanya kode). */
  nama: string | null;
  aktif: boolean;
  jumlahEntri: number;
}

export function toPedagangRow(p: PedagangResponse): PedagangRow {
  return {
    id: String(p.id),
    pasarId: p.pasar_id,
    pasarNama: p.pasar_nama,
    kode: p.kode,
    nama: p.nama?.trim() ? p.nama : null,
    aktif: p.aktif,
    jumlahEntri: p.jumlah_entri,
  };
}

export interface PedagangParams {
  pasarId?: string;
  q?: string;
  aktif?: boolean;
  page?: number;
  limit?: number;
}

export function toPedagangQuery(p: PedagangParams = {}): string {
  return queryString({ pasar_id: p.pasarId, q: p.q?.trim(), aktif: p.aktif, page: p.page, limit: p.limit });
}

/* ── Detail ─────────────────────────────────────────────────── */

export interface KomoditasDetailResponse extends MasterKomoditasResponse {
  /** Terbaru dahulu. */
  riwayat_versi?: RiwayatVersiResponse[] | null;
  perubahan_menunggu?: PerubahanMasterResponse | null;
}

export interface KomoditasDetail {
  komoditas: MasterKomoditasRow;
  riwayat: RiwayatVersi[];
  menunggu: PerubahanMaster | null;
}

export function toKomoditasDetail(d: KomoditasDetailResponse): KomoditasDetail {
  return {
    komoditas: toMasterKomoditasRow(d),
    riwayat: (d.riwayat_versi ?? []).map(toRiwayatVersi),
    menunggu: d.perubahan_menunggu ? toPerubahanMaster(d.perubahan_menunggu) : null,
  };
}

export interface PasarDetailResponse extends MasterPasarResponse {
  kabupaten_nama: string;
  jumlah_pedagang: number;
}

export interface PasarDetail {
  pasar: MasterPasarRow;
  jumlahPedagang: number;
}

export function toPasarDetail(d: PasarDetailResponse): PasarDetail {
  const nama = formatWilayah(d.kabupaten_nama);
  return {
    pasar: toMasterPasarRow(d, new Map([[d.kabupaten_kode, nama]])),
    jumlahPedagang: d.jumlah_pedagang,
  };
}

export interface WilayahDetailResponse extends MasterWilayahResponse {
  jumlah_kecamatan: number;
  jumlah_pasar: number;
}

export interface WilayahDetail {
  wilayah: MasterWilayahRow;
  jenis: string;
  jumlahKecamatan: number;
  jumlahPasar: number;
}

export function toWilayahDetail(d: WilayahDetailResponse): WilayahDetail {
  return {
    wilayah: toMasterWilayahRow(d),
    jenis: d.jenis,
    jumlahKecamatan: d.jumlah_kecamatan,
    jumlahPasar: d.jumlah_pasar,
  };
}
