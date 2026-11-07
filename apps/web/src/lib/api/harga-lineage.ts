/**
 * Lineage entri harga: asal, validasi otomatis, validasi, publikasi, penolakan, versi, dan rujukan audit
 * (PRD §1.4). Endpoint: GET /v1/harga/:id/lineage dan /v1/harga/:id/versi.
 */

import type {
  AksiVersiHarga,
  LineageHarga,
  StatusEntri,
  TahapLineageHarga,
  VersiEntriHarga,
} from "@/lib/data/types";
import {
  toBendera,
  type FlagValidasiResponse,
  type RiwayatStatusResponse,
} from "./harga";

export interface VersiEntriResponse {
  versi: number;
  aksi: AksiVersiHarga;
  status: StatusEntri;
  harga: number;
  oleh: string | null;
  oleh_nama: string | null;
  waktu: string;
  perubahan: string[];
  /** Salinan lengkap baris entri pada versi ini apa adanya dari basis data. */
  data: Record<string, unknown>;
}

export interface TahapLineageResponse {
  oleh: string | null;
  nama: string | null;
  pada: string | null;
  alasan?: string | null;
}

export interface LineageResponse {
  entri: {
    id: string;
    kode: string | null;
    status: StatusEntri;
    harga: number;
  };
  asal: {
    sumber: string;
    petugas_id: string | null;
    petugas_nama: string | null;
    dicatat_pada: string;
    diterima_pada: string;
    lat: number | null;
    lng: number | null;
    dalam_geofence: boolean;
    kode_pedagang: string | null;
    tugas: {
      id: string;
      kode: string | null;
      judul: string;
      ditautkan_pada: string;
    }[];
  };
  validasi_otomatis: {
    flag_validasi: FlagValidasiResponse;
    skor_anomali: number;
    deviasi_pct: number;
  };
  validasi: TahapLineageResponse | null;
  publikasi: TahapLineageResponse | null;
  penolakan: TahapLineageResponse | null;
  versi_terkini: number;
  versi: VersiEntriResponse[];
  riwayat: RiwayatStatusResponse[];
  audit: {
    id: string;
    waktu: string;
    aksi: string;
    aktor_nama: string | null;
    aktor_peran: string | null;
    hasil: string;
  }[];
}

/** Label aksi versi untuk tampilan. */
export const LABEL_AKSI_VERSI: Record<AksiVersiHarga, string> = {
  INPUT: "Dicatat",
  AJUKAN: "Diajukan",
  VALIDASI: "Divalidasi",
  PUBLIKASI: "Dipublikasi",
  TOLAK: "Ditolak",
  KEMBALIKAN: "Dikembalikan ke draf",
  UBAH: "Diubah",
  MIGRASI: "Data awal",
};

/** Label sumber data entri. */
export const LABEL_SUMBER: Record<string, string> = {
  LAPANGAN: "Lapangan",
  PIHPS: "PIHPS",
  BAPANAS: "Bapanas",
  PORTAL_DAERAH: "Portal daerah",
};

const LABEL_KOLOM: Record<string, string> = {
  status: "status",
  harga: "harga",
  tipe: "tipe",
  satuan: "satuan",
  kualitas: "kualitas",
  sumber: "sumber",
  catatan: "catatan",
  komoditas_kode: "komoditas",
  pasar_id: "pasar",
  kode_pedagang: "pedagang",
  foto_url: "foto",
  lat: "lokasi",
  lng: "lokasi",
  dalam_geofence: "geofence",
  deviasi_pct: "deviasi",
  skor_anomali: "skor anomali",
  flag_validasi: "bendera cek otomatis",
  petugas_id: "pencatat",
  divalidasi_oleh: "validator",
  dipublikasi_oleh: "penerbit",
  ditolak_oleh: "penolak",
  alasan_penolakan: "alasan penolakan",
  idempotency_key: "kunci kiriman",
  kode: "kode",
};

/** Nama kolom → label ringkas; kolom yang tak dikenal ditampilkan dengan garis bawah diganti spasi. */
export function labelKolom(kolom: string): string {
  return LABEL_KOLOM[kolom] ?? kolom.replaceAll("_", " ");
}

/** Daftar kolom yang berubah, siap tampil ("status, validator"); label sama (lat/lng) digabung. */
export function ringkasPerubahan(perubahan: string[]): string {
  return [...new Set(perubahan.map(labelKolom))].join(", ");
}

/** Tautan ke audit trail yang disaring pada entri ini (halaman /audit membaca parameter kueri ini). */
export function tautanAuditEntri(id: string): string {
  return `/audit?${new URLSearchParams({ entitas: "entri_harga", entitas_id: id })}`;
}

export function toVersiEntri(v: VersiEntriResponse): VersiEntriHarga {
  return {
    versi: v.versi,
    aksi: v.aksi,
    status: v.status,
    harga: v.harga,
    ...(v.oleh_nama ? { olehNama: v.oleh_nama } : {}),
    waktu: v.waktu,
    perubahan: v.perubahan ?? [],
  };
}

function toTahap(
  t: TahapLineageResponse | null,
): TahapLineageHarga | undefined {
  if (!t) return undefined;
  return {
    ...(t.nama ? { olehNama: t.nama } : {}),
    ...(t.pada ? { pada: t.pada } : {}),
    ...(t.alasan ? { alasan: t.alasan } : {}),
  };
}

/** Respons lineage API → model tampilan. */
export function toLineageHarga(r: LineageResponse): LineageHarga {
  return {
    ...(r.asal.petugas_nama ? { petugasNama: r.asal.petugas_nama } : {}),
    dicatatPada: r.asal.dicatat_pada,
    diterimaPada: r.asal.diterima_pada,
    sumber: r.asal.sumber,
    ...(r.asal.kode_pedagang ? { kodePedagang: r.asal.kode_pedagang } : {}),
    tugas: r.asal.tugas.map((t) => t.judul),
    bendera: toBendera(r.validasi_otomatis.flag_validasi).map((b) => b.label),
    skorAnomali: r.validasi_otomatis.skor_anomali,
    deviasiPct: r.validasi_otomatis.deviasi_pct,
    validasi: toTahap(r.validasi),
    publikasi: toTahap(r.publikasi),
    penolakan: toTahap(r.penolakan),
    versiTerkini: r.versi_terkini,
    versi: r.versi.map(toVersiEntri),
    aksiAudit: [...new Set(r.audit.map((a) => a.aksi))],
  };
}
