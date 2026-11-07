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
  /** ID BIGINT sebagai string desimal ("1024"). */
  id: string;
  /** ID lama (mis. HRG-D00-00-01-K) bila entri dari sebelum migrasi 000111; null untuk entri baru. */
  kode?: string | null;
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
  alasan_penolakan?: string | null;
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
  /** "skor": skor anomali tertinggi lebih dulu (prioritas antrean validasi). */
  urut?: "skor";
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
    ...(r.kode ? { kode: r.kode } : {}),
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
    ...(r.alasan_penolakan ? { alasanPenolakan: r.alasan_penolakan } : {}),
  };
}

/** Query string daftar entri dengan nama parameter kontrak API; nilai kosong dibuang. */
export function toHargaQuery(p: HargaParams = {}): string {
  const qs = new URLSearchParams();
  if (p.status) qs.set("status", p.status);
  if (p.wilayah) qs.set("wilayah", p.wilayah);
  if (p.komoditas) qs.set("komoditas", p.komoditas);
  if (p.urut) qs.set("urut", p.urut);
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
  const hasil = rows
    .filter(
      (e) =>
        (!p.status || e.status === p.status) &&
        (!p.wilayah || e.kabupatenKode === p.wilayah) &&
        (!p.komoditas || e.komoditasKode === p.komoditas),
    )
    .slice(0, p.limit ?? HARGA_PAGE_LIMIT);
  return p.urut === "skor" ? hasil.sort((a, b) => b.skorAnomali - a.skorAnomali) : hasil;
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

/** Sebab sebuah entri dilewati pada aksi massal (kontrak POST /v1/harga/bulk). */
export type SebabLewat = "tidak_ditemukan" | "status_tidak_sesuai" | "pemisahan_tugas" | "bukan_pemilik";

export interface RincianBulk {
  id: string;
  sebab: SebabLewat;
}

const KETERANGAN_SEBAB: Record<SebabLewat, string> = {
  tidak_ditemukan: "tidak ditemukan",
  status_tidak_sesuai: "status tidak sesuai",
  pemisahan_tugas: "dicatat oleh Anda sendiri",
  bukan_pemilik: "bukan entri Anda",
};

/** Ringkasan sebab entri dilewati, mis. "2 status tidak sesuai, 1 dicatat oleh Anda sendiri". */
export function ringkasSebab(rincian: RincianBulk[] | undefined): string {
  const hitung = new Map<SebabLewat, number>();
  for (const r of rincian ?? []) hitung.set(r.sebab, (hitung.get(r.sebab) ?? 0) + 1);
  return [...hitung].map(([sebab, n]) => `${n} ${KETERANGAN_SEBAB[sebab] ?? sebab}`).join(", ");
}

export interface RiwayatStatusResponse {
  dari: StatusEntri | null;
  ke: StatusEntri;
  oleh: string | null;
  oleh_nama: string | null;
  waktu: string;
  alasan: string | null;
}

export interface RiwayatStatus {
  dari?: StatusEntri;
  ke: StatusEntri;
  olehNama?: string;
  waktu: string;
  alasan?: string;
}

export function toRiwayatStatus(r: RiwayatStatusResponse): RiwayatStatus {
  return { dari: r.dari ?? undefined, ke: r.ke, olehNama: r.oleh_nama ?? undefined, waktu: r.waktu, alasan: r.alasan ?? undefined };
}

/** Hasil cek otomatis server (kontrak FlagValidasi). */
export interface FlagValidasiResponse {
  di_luar_rentang: boolean;
  outlier: boolean;
  duplikat: boolean;
  luar_geofence: boolean;
  tanpa_lokasi: boolean;
  bentrok: boolean;
  median?: number;
  jarak_meter?: number;
}

export interface HargaDetailResponse extends EntriHargaResponse {
  lat?: number | null;
  lng?: number | null;
  foto_url?: string | null;
  flag_validasi: FlagValidasiResponse;
}

/** Bukti dan bendera validasi otomatis satu entri, untuk panel detail. */
export interface DetailHarga {
  lat?: number;
  lng?: number;
  fotoUrl?: string;
  median?: number;
  jarakMeter?: number;
  /** Bendera yang menyala, berurut dari yang paling berat, dengan label siap tampil. */
  bendera: { kode: keyof Omit<FlagValidasiResponse, "median" | "jarak_meter">; label: string }[];
}

const LABEL_BENDERA: [keyof Omit<FlagValidasiResponse, "median" | "jarak_meter">, string][] = [
  ["di_luar_rentang", "Di luar rentang harga wajar"],
  ["outlier", "Menyimpang jauh dari histori harga"],
  ["bentrok", "Berbeda dari harga petugas lain (dipertahankan petugas)"],
  ["duplikat", "Duplikat: pasar, komoditas, dan pedagang sama dalam 3 jam"],
  ["luar_geofence", "Di luar geofence pasar"],
  ["tanpa_lokasi", "Tanpa koordinat GPS"],
];

/** Bendera yang menyala, berurut dari yang paling berat, dengan label siap tampil. */
export function toBendera(flag: FlagValidasiResponse): DetailHarga["bendera"] {
  return LABEL_BENDERA.filter(([kode]) => flag[kode]).map(([kode, label]) => ({ kode, label }));
}

export function toDetailHarga(r: HargaDetailResponse): DetailHarga {
  return {
    lat: r.lat ?? undefined,
    lng: r.lng ?? undefined,
    fotoUrl: r.foto_url ?? undefined,
    median: r.flag_validasi.median || undefined,
    jarakMeter: r.flag_validasi.jarak_meter || undefined,
    bendera: toBendera(r.flag_validasi),
  };
}
