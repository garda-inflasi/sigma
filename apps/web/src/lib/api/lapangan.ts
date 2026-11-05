/**
 * Pemetaan tugas lapangan dari representasi API Go (snake_case) ke model UI.
 * Endpoint: GET /v1/lapangan/tugas/saya, POST /v1/lapangan/tugas/bulk.
 */

import type { TipeHarga, TugasLapangan } from "@/lib/data/types";

export interface TugasLapanganResponse {
  id: string;
  judul: string;
  pasar_id: string;
  pasar_nama: string;
  komoditas: string[];
  tenggat: string;
  status: TugasLapangan["status"];
  target: number;
  /** Jumlah komoditas yang sudah tercatat terhadap target. */
  terisi: number;
  tipe: TipeHarga;
}

export function toTugasLapangan(t: TugasLapanganResponse): TugasLapangan {
  return {
    id: t.id,
    judul: t.judul,
    pasarId: t.pasar_id,
    pasarNama: t.pasar_nama,
    komoditas: t.komoditas,
    tenggat: t.tenggat,
    status: t.status,
    target: t.target,
    terisi: t.terisi,
    tipe: t.tipe,
  };
}
