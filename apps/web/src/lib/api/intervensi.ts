/**
 * Pemetaan kartu intervensi dari representasi API Go (snake_case) ke model UI.
 * Endpoint: GET /v1/intervensi, PATCH /v1/intervensi/{id}/tahap.
 */

import type { KartuIntervensi, Prioritas } from "@/lib/data/types";
import type { Siklus } from "@/lib/design/tokens";

export interface KartuResponse {
  id: string;
  /** Enum basis data: NORMAL | TEKANAN | SURPLUS. */
  siklus: string;
  tahap: string;
  judul: string;
  komoditas_kode: string;
  kabupaten_kode: string;
  prioritas: Prioritas;
  tenggat: string;
  rekomendasi_ai: boolean;
  progres: { selesai: number; total: number };
  penanggung_jawab: { id: string; nama: string }[];
  versi: number;
  dieskalasi_pada: string | null;
}

/** Hasil aksi massal pada satu kartu (POST /v1/intervensi/bulk). */
export interface HasilKartu {
  id: string;
  berhasil: boolean;
  alasan?: string;
}

export interface BulkKartuResult {
  berhasil: number;
  gagal: number;
  hasil: HasilKartu[];
}

export type AksiKartu = "tugaskan" | "eskalasi";

/** Ringkasan satu kalimat hasil aksi massal untuk toast; alasan kegagalan pertama disertakan. */
export function ringkasHasilKartu(label: string, r: BulkKartuResult): { judul: string; deskripsi?: string } {
  const gagal = r.hasil.filter((h) => !h.berhasil);
  const alasan = [...new Set(gagal.map((h) => h.alasan).filter(Boolean))].join("; ");
  return {
    judul: `${label}: ${r.berhasil} kartu`,
    deskripsi: r.gagal ? `${r.gagal} kartu dilewati${alasan ? ` (${alasan})` : ""}.` : "Tercatat di audit trail.",
  };
}

const SIKLUS: readonly Siklus[] = ["normal", "tekanan", "surplus"];

/** "Rina Safitri, S.T., M.T." → "RS": huruf awal dua kata pertama yang diawali huruf. */
export function inisial(nama: string): string {
  return nama
    .split(/\s+/)
    .filter((kata) => /^[A-Za-z]/.test(kata))
    .slice(0, 2)
    .map((kata) => kata[0])
    .join("")
    .toUpperCase();
}

export function toKartu(r: KartuResponse): KartuIntervensi {
  const siklus = r.siklus.toLowerCase();
  return {
    id: r.id,
    siklus: SIKLUS.find((s) => s === siklus) ?? "normal",
    tahap: r.tahap,
    judul: r.judul,
    komoditasKode: r.komoditas_kode,
    kabupatenKode: r.kabupaten_kode,
    prioritas: r.prioritas,
    penanggungJawab: r.penanggung_jawab.map((p) => ({
      nama: p.nama,
      inisial: inisial(p.nama),
    })),
    tenggat: r.tenggat,
    rekomendasiAi: r.rekomendasi_ai,
    progres: { selesai: r.progres.selesai, total: r.progres.total },
    versi: r.versi,
    dieskalasi: r.dieskalasi_pada !== null,
  };
}
