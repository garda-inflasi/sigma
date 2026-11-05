/**
 * Pemetaan master data dari representasi API Go (snake_case) ke model UI.
 * Endpoint: GET /v1/master/{komoditas,wilayah,pasar}.
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
