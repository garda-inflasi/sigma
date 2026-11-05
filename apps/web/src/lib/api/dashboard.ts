/**
 * Pemetaan dashboard operasional dari representasi API Go (snake_case) ke model UI.
 * Endpoint: GET /v1/dashboard/operasional/{kpi,heatmap,aktivitas-lapangan}.
 */

import { kabupatenByKode } from "@/lib/data/reference";
import type { EntriHarga, KpiOperasional, SelHeatmap } from "@/lib/data/types";
import { formatWilayah } from "./cold-storage";

export interface KpiResponse {
  id: string;
  label: string;
  nilai: number;
  format: KpiOperasional["format"];
  delta: number;
  keterangan_delta: string;
  tren: number[];
  tone: KpiOperasional["tone"];
  href: string;
}

export interface HeatmapResponse {
  kabupaten: { kode: string; nama: string }[];
  komoditas: { kode: string; nama: string; pilot: boolean }[];
  sel: {
    komoditas_kode: string;
    kabupaten_kode: string;
    delta: number;
    harga: number;
  }[];
}

export interface Heatmap {
  kabupaten: { kode: string; nama: string }[];
  komoditas: { kode: string; nama: string; pilot: boolean }[];
  sel: SelHeatmap[];
}

export interface AktivitasLapanganResponse {
  kabupaten_kode: string;
  kabupaten_nama: string;
  jumlah_petugas: number;
  entri: number;
  target: number;
}

/** Capaian entri harga konsumen hari ini terhadap target harian satu kabupaten/kota. */
export interface AktivitasLapangan {
  kode: string;
  nama: string;
  jumlahPetugas: number;
  entri: number;
  target: number;
}

export function toKpiOperasional(k: KpiResponse): KpiOperasional {
  return {
    id: k.id,
    label: k.label,
    nilai: k.nilai,
    format: k.format,
    delta: k.delta,
    keteranganDelta: k.keterangan_delta,
    tren: k.tren,
    tone: k.tone,
    href: k.href,
  };
}

/** Nama ringkas kabupaten/kota ("Aceh Besar"); bila kode tak dikenal, label dari nama resmi. */
const namaRingkas = (kode: string, namaResmi: string) =>
  kabupatenByKode[kode]?.nama ?? formatWilayah(namaResmi);

export function toHeatmap(h: HeatmapResponse): Heatmap {
  return {
    kabupaten: h.kabupaten.map((k) => ({
      kode: k.kode,
      nama: namaRingkas(k.kode, k.nama),
    })),
    komoditas: h.komoditas,
    sel: h.sel.map((s) => ({
      komoditasKode: s.komoditas_kode,
      kabupatenKode: s.kabupaten_kode,
      delta: s.delta,
      harga: s.harga,
    })),
  };
}

export function toAktivitasLapangan(
  a: AktivitasLapanganResponse,
): AktivitasLapangan {
  return {
    kode: a.kabupaten_kode,
    nama: namaRingkas(a.kabupaten_kode, a.kabupaten_nama),
    jumlahPetugas: a.jumlah_petugas,
    entri: a.entri,
    target: a.target,
  };
}

/** Persen capaian, dibatasi 0–100; target nol dianggap belum ada capaian. */
export function capaianPct(
  a: Pick<AktivitasLapangan, "entri" | "target">,
): number {
  return a.target > 0 ? Math.min(100, (a.entri / a.target) * 100) : 0;
}

/** Aktivitas dari entri mock: tiap petugas diharapkan mencatat sekali per komoditas (14 komoditas). */
export function aktivitasMock(
  entri: EntriHarga[],
  petugasPerKabupaten: ReadonlyMap<string, number>,
  komoditasAktif = 14,
): AktivitasLapangan[] {
  return [...petugasPerKabupaten].map(([kode, n]) => ({
    kode,
    nama: kabupatenByKode[kode]?.nama ?? kode,
    jumlahPetugas: n,
    entri: entri.filter(
      (e) => e.kabupatenKode === kode && e.tipe === "KONSUMEN",
    ).length,
    target: n * komoditasAktif,
  }));
}
