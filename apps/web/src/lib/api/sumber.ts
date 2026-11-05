/**
 * Pemetaan katalog sumber data dari representasi API Go (snake_case) ke model UI.
 * Endpoint: GET /v1/sumber-data, POST /v1/sumber-data/bulk.
 */

import type { EksekusiSumber, SampelSumber, SumberData } from "@/lib/data/types";

export interface SumberDataResponse {
  id: string;
  nama: string;
  penyedia: string;
  jenis: string;
  cakupan: string;
  jadwal_cron: string;
  status: string;
  terakhir_jalan: string | null;
  keberhasilan_7h: number | null;
  latensi_ms: number;
  robots_txt_patuh: boolean;
  atribusi: string;
  url_dasar: string | null;
  bisa_dijalankan: boolean;
}

export interface EksekusiResponse {
  id: number;
  pemicu: EksekusiSumber["pemicu"];
  status: EksekusiSumber["status"];
  mulai: string;
  selesai: string | null;
  durasi_ms: number | null;
  latensi_ms: number | null;
  dokumen: number;
  baris_diambil: number;
  baris_ditolak: number;
  galat: string | null;
  ringkasan: {
    tak_terpetakan?: Record<string, number>;
    tidak_valid?: Record<string, number>;
  };
}

export interface SampelResponse {
  id: number;
  kunci_permintaan: string;
  diambil_pada: string;
  ukuran: number;
  cuplikan: string;
}

export type SumberBulkAksi = "jeda" | "lanjutkan";

const STATUS = ["BELUM_AKTIF", "SEHAT", "TERTUNDA", "GAGAL", "DIJEDA"] as const;
const HARI = ["Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"];

const pad = (n: number) => String(n).padStart(2, "0");
const isAngka = (s: string) => /^\d+$/.test(s);

/**
 * Cron WIB lima kolom → teks Indonesia untuk pola yang dipakai katalog
 * (tiap N menit/jam, harian, mingguan). Pola lain ditampilkan apa adanya.
 */
export function formatJadwal(cron: string): string {
  const f = cron.trim().split(/\s+/);
  if (f.length !== 5) return cron;
  const [menit, jam, tanggal, bulan, hari] = f;
  if (tanggal !== "*" || bulan !== "*") return cron;

  if (jam === "*" && hari === "*") {
    if (menit.startsWith("*/") && isAngka(menit.slice(2)))
      return `Tiap ${menit.slice(2)} menit`;
    if (isAngka(menit)) return "Tiap jam";
  }
  if (
    isAngka(menit) &&
    jam.startsWith("*/") &&
    isAngka(jam.slice(2)) &&
    hari === "*"
  )
    return `Tiap ${jam.slice(2)} jam`;
  if (isAngka(menit) && isAngka(jam)) {
    const waktu = `${pad(Number(jam))}.${pad(Number(menit))}`;
    if (hari === "*") return `Harian ${waktu}`;
    if (isAngka(hari)) return `Mingguan ${HARI[Number(hari) % 7]} ${waktu}`;
  }
  return cron;
}

export function toSumberData(s: SumberDataResponse): SumberData {
  return {
    id: s.id,
    nama: s.nama,
    penyedia: s.penyedia,
    jenis: s.jenis === "SCRAPER" ? "SCRAPER" : "API",
    cakupan: s.cakupan,
    jadwal: formatJadwal(s.jadwal_cron),
    terakhir: s.terakhir_jalan,
    status: (STATUS as readonly string[]).includes(s.status)
      ? (s.status as SumberData["status"])
      : "BELUM_AKTIF",
    keberhasilan7h: s.keberhasilan_7h,
    latensiMs: s.latensi_ms,
    bisaDijalankan: s.bisa_dijalankan,
  };
}

/** Urut menurun menurut jumlah baris, lalu abjad, agar tampilan stabil. */
const hitungan = (m: Record<string, number> | undefined) =>
  Object.entries(m ?? {}).sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]));

export function toEksekusi(e: EksekusiResponse): EksekusiSumber {
  return {
    id: e.id,
    pemicu: e.pemicu,
    status: e.status,
    mulai: e.mulai,
    selesai: e.selesai,
    durasiMs: e.durasi_ms,
    latensiMs: e.latensi_ms,
    dokumen: e.dokumen,
    barisDiambil: e.baris_diambil,
    barisDitolak: e.baris_ditolak,
    galat: e.galat,
    takTerpetakan: hitungan(e.ringkasan?.tak_terpetakan).map(([alias, baris]) => ({ alias, baris })),
    tidakValid: hitungan(e.ringkasan?.tidak_valid).map(([alasan, baris]) => ({ alasan, baris })),
  };
}

export function toSampel(s: SampelResponse): SampelSumber {
  return { id: s.id, kunci: s.kunci_permintaan, diambilPada: s.diambil_pada, ukuran: s.ukuran, cuplikan: s.cuplikan };
}
