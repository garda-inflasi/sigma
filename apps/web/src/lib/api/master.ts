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
    radiusMeter: p.radius_meter,
    aktif: p.aktif,
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
