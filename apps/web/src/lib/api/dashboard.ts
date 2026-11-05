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
    sumber: SelHeatmap["sumber"];
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
      sumber: s.sumber,
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

export interface PembandingHargaResponse {
  pasangan: number;
  sepakat: number;
  batas_pct: number;
  items: {
    komoditas_kode: string;
    komoditas_nama: string;
    kabupaten_kode: string;
    kabupaten_nama: string;
    tanggal: string;
    harga_lapangan: number;
    harga_eksternal: number;
    selisih_pct: number;
    sepakat: boolean;
    demo: boolean;
  }[];
}

/** Validasi silang harga: lapangan (tervalidasi) vs eksternal (hasil ingestion) pada hari yang sama. */
export interface PembandingHarga {
  /** Jumlah pasangan komoditas-wilayah yang dibandingkan, dan yang selisihnya dalam batas. */
  pasangan: number;
  sepakat: number;
  batasPct: number;
  /** Selisih terbesar lebih dulu. */
  items: {
    komoditasNama: string;
    kabupatenNama: string;
    tanggal: string;
    hargaLapangan: number;
    hargaEksternal: number;
    selisihPct: number;
    sepakat: boolean;
    demo: boolean;
  }[];
}

export function toPembandingHarga(p: PembandingHargaResponse): PembandingHarga {
  return {
    pasangan: p.pasangan,
    sepakat: p.sepakat,
    batasPct: p.batas_pct,
    items: p.items.map((i) => ({
      komoditasNama: i.komoditas_nama,
      kabupatenNama: namaRingkas(i.kabupaten_kode, i.kabupaten_nama),
      tanggal: i.tanggal,
      hargaLapangan: i.harga_lapangan,
      hargaEksternal: i.harga_eksternal,
      selisihPct: i.selisih_pct,
      sepakat: i.sepakat,
      demo: i.demo,
    })),
  };
}

export interface CuacaResponse {
  kabupaten_kode: string;
  kabupaten_nama: string;
  tanggal: string;
  curah_hujan_mm: number;
  suhu_c: number;
  kelembapan: number;
  anomali: boolean;
  sumber: string;
  demo: boolean;
}

/** Cuaca harian terbaru satu kabupaten/kota; anomali = hujan lebat (>= 50 mm/hari). */
export interface CuacaWilayah {
  kode: string;
  nama: string;
  tanggal: string;
  curahHujanMm: number;
  suhuC: number;
  kelembapan: number;
  anomali: boolean;
  sumber: string;
  demo: boolean;
}

export function toCuacaWilayah(c: CuacaResponse): CuacaWilayah {
  return {
    kode: c.kabupaten_kode,
    nama: namaRingkas(c.kabupaten_kode, c.kabupaten_nama),
    tanggal: c.tanggal,
    curahHujanMm: c.curah_hujan_mm,
    suhuC: c.suhu_c,
    kelembapan: c.kelembapan,
    anomali: c.anomali,
    sumber: c.sumber,
    demo: c.demo,
  };
}
