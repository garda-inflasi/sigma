/**
 * Pemetaan prediksi dari representasi API Go (snake_case) ke model UI.
 * Endpoint: GET /v1/prediksi, /v1/prediksi/{kode}, /v1/prediksi/registri.
 */

import type { FaktorXai, ProyeksiXai } from "@/lib/data/types";

export interface RingkasanResponse {
  komoditas_kode: string;
  komoditas_nama: string;
  kabupaten_kode: string;
  horizon_hari: number;
  harga_kini: number;
  proyeksi: number;
  perubahan_pct: number;
  bawah_pct: number;
  atas_pct: number;
  model_versi: string;
  mape: number;
  sumber: string;
}

export interface TitikResponse {
  tanggal: string;
  aktual?: number;
  proyeksi?: number;
  bawah?: number;
  atas?: number;
}

export interface ProyeksiResponse {
  komoditas_kode: string;
  komoditas_nama: string;
  kabupaten_kode: string;
  kabupaten_nama: string;
  horizon_hari: number;
  harga_kini: number;
  perubahan_pct: number;
  bawah_pct: number;
  atas_pct: number;
  seri: TitikResponse[];
  faktor: FaktorXai[];
  narasi: string;
  model: { nama: string; versi: string; mape: number; dilatih: string };
  rekomendasi: string[];
  sumber: string;
}

export interface ModelResponse {
  id: string;
  komoditas_kode: string;
  komoditas_nama: string;
  nama: string;
  versi: string;
  mape: number;
  drift: number;
  status: string;
  dilatih_pada: string;
  sumber: string;
}

/** Satu baris tabel "Proyeksi seluruh komoditas". */
export interface BarisProyeksi {
  id: string;
  nama: string;
  kini: number;
  proyeksi: number;
  pct: number;
  lo: number;
  hi: number;
  mape: number;
  /** Data contoh dari seed, bukan keluaran model sungguhan. */
  demo: boolean;
}

/** Satu baris tabel "Registri model". */
export interface BarisModel {
  id: string;
  komoditas: string;
  versi: string;
  mape: number;
  drift: number;
  status: "PRODUKSI" | "KANDIDAT" | "ARSIP";
  demo: boolean;
}

const STATUS_MODEL: readonly BarisModel["status"][] = [
  "PRODUKSI",
  "KANDIDAT",
  "ARSIP",
];

export const isDemo = (sumber: string): boolean => sumber === "DEMO";

export function toProyeksiXai(r: ProyeksiResponse): ProyeksiXai {
  return {
    komoditasKode: r.komoditas_kode,
    nama: r.komoditas_nama,
    horizonHari: r.horizon_hari,
    perubahanPct: r.perubahan_pct,
    interval: [r.bawah_pct, r.atas_pct],
    hargaKini: r.harga_kini,
    seri: r.seri.map((t) => ({
      tanggal: t.tanggal,
      ...(t.aktual !== undefined ? { aktual: t.aktual } : {}),
      ...(t.proyeksi !== undefined ? { proyeksi: t.proyeksi } : {}),
      ...(t.bawah !== undefined && t.atas !== undefined
        ? { rentang: [t.bawah, t.atas] as [number, number] }
        : {}),
    })),
    faktor: r.faktor,
    narasi: r.narasi,
    model: r.model,
    rekomendasi: r.rekomendasi,
    demo: isDemo(r.sumber),
  };
}

export function toBarisProyeksi(r: RingkasanResponse): BarisProyeksi {
  return {
    id: r.komoditas_kode,
    nama: r.komoditas_nama,
    kini: r.harga_kini,
    proyeksi: r.proyeksi,
    pct: r.perubahan_pct,
    lo: r.bawah_pct,
    hi: r.atas_pct,
    mape: r.mape,
    demo: isDemo(r.sumber),
  };
}

export function toBarisModel(r: ModelResponse): BarisModel {
  return {
    id: r.id,
    komoditas: r.komoditas_nama,
    versi: r.versi,
    mape: r.mape,
    drift: r.drift,
    status: (STATUS_MODEL as readonly string[]).includes(r.status)
      ? (r.status as BarisModel["status"])
      : "ARSIP",
    demo: isDemo(r.sumber),
  };
}
