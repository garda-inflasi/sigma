/**
 * Pemetaan entri harga dari representasi API Go (snake_case) ke model UI.
 * Endpoint: GET /v1/harga, /v1/harga/saya, /v1/harga/ringkasan, /v1/harga/tren.
 */

import type {
  EntriHarga,
  Kualitas,
  StatusEntri,
  TipeHarga,
} from "@/lib/data/types";
import { formatWilayah } from "./cold-storage";

export interface EntriHargaResponse {
  id: string;
  komoditas_kode: string;
  komoditas_nama: string;
  pasar_id: string;
  pasar_nama: string;
  kabupaten_kode: string;
  /** Nama resmi, mis. "KABUPATEN ACEH BESAR". */
  wilayah: string;
  tipe: TipeHarga;
  harga: number;
  satuan: string;
  kualitas: Kualitas;
  petugas_id: string | null;
  petugas_nama: string | null;
  dicatat_pada: string;
  status: StatusEntri;
  deviasi_pct: number;
  skor_anomali: number;
  ada_foto: boolean;
  dalam_geofence: boolean;
}

export interface RingkasanHarga {
  draf: number;
  diajukan: number;
  divalidasi: number;
  dipublikasi: number;
  ditolak: number;
  /** Entri dengan deviasi mutlak di atas 25%. */
  anomali: number;
}

/** Penyaring daftar entri; diterjemahkan server (status, wilayah, komoditas, tanggal). */
export interface HargaParams {
  status?: StatusEntri;
  /** Kode kabupaten/kota. */
  wilayah?: string;
  komoditas?: string;
  /** YYYY-MM-DD, waktu Aceh. */
  dari?: string;
  sampai?: string;
  page?: number;
  limit?: number;
}

/** Entri yang dimuat sekaligus oleh tabel validasi (batas server 1000). */
export const HARGA_PAGE_LIMIT = 500;

export function toEntriHarga(r: EntriHargaResponse): EntriHarga {
  return {
    id: r.id,
    komoditasKode: r.komoditas_kode,
    komoditasNama: r.komoditas_nama,
    pasarId: r.pasar_id,
    pasarNama: r.pasar_nama,
    kabupatenKode: r.kabupaten_kode,
    wilayah: formatWilayah(r.wilayah),
    tipe: r.tipe,
    harga: r.harga,
    satuan: r.satuan,
    kualitas: r.kualitas,
    petugasId: r.petugas_id ?? "",
    petugasNama: r.petugas_nama ?? "",
    dicatatPada: r.dicatat_pada,
    status: r.status,
    deviasiPct: r.deviasi_pct,
    skorAnomali: r.skor_anomali,
    adaFoto: r.ada_foto,
    dalamGeofence: r.dalam_geofence,
  };
}

/** Query string daftar entri dengan nama parameter kontrak API; nilai kosong dibuang. */
export function toHargaQuery(p: HargaParams = {}): string {
  const qs = new URLSearchParams();
  if (p.status) qs.set("status", p.status);
  if (p.wilayah) qs.set("wilayah", p.wilayah);
  if (p.komoditas) qs.set("komoditas", p.komoditas);
  if (p.dari) qs.set("dari", p.dari);
  if (p.sampai) qs.set("sampai", p.sampai);
  if (p.page) qs.set("page", String(p.page));
  qs.set("limit", String(p.limit ?? HARGA_PAGE_LIMIT));
  return `?${qs.toString()}`;
}

/** Penyaring yang sama dengan server, untuk data mock. */
export function filterMockEntri(
  rows: EntriHarga[],
  p: HargaParams = {},
): EntriHarga[] {
  return rows
    .filter(
      (e) =>
        (!p.status || e.status === p.status) &&
        (!p.wilayah || e.kabupatenKode === p.wilayah) &&
        (!p.komoditas || e.komoditasKode === p.komoditas),
    )
    .slice(0, p.limit ?? HARGA_PAGE_LIMIT);
}

/** Ringkasan per status dari daftar entri (data mock). */
export function ringkasanDariEntri(rows: EntriHarga[]): RingkasanHarga {
  const n = (s: StatusEntri) => rows.filter((e) => e.status === s).length;
  return {
    draf: n("DRAF"),
    diajukan: n("DIAJUKAN"),
    divalidasi: n("DIVALIDASI"),
    dipublikasi: n("DIPUBLIKASI"),
    ditolak: n("DITOLAK"),
    anomali: rows.filter((e) => Math.abs(e.deviasiPct) > 25).length,
  };
}
