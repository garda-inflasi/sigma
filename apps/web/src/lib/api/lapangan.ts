/**
 * Pemetaan tugas lapangan dari representasi API Go (snake_case) ke model UI.
 * Endpoint: GET /v1/lapangan/tugas/saya, GET /v1/lapangan/tugas/{id}, POST /v1/lapangan/tugas/bulk.
 */

import { apiFetch, MODE_MOCK } from "@/lib/api/client";
import type { TipeHarga, TugasLapangan } from "@/lib/data/types";
import type { EntriLokal } from "@/lib/stores/field-queue-store";

type VersiServer = NonNullable<EntriLokal["versiServer"]>;

export interface TugasLapanganResponse {
  /** ID BIGINT sebagai string desimal ("42"). */
  id: string;
  /** ID lama (mis. TGS-mahasiswa_1-1) bila tugas dari sebelum migrasi 000111; null untuk tugas baru. */
  kode?: string | null;
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

/** GET /v1/lapangan/tugas/{id}: tugas ditambah nama dan status terisi tiap komoditas. */
export interface TugasDetailResponse extends TugasLapanganResponse {
  rincian_komoditas: { kode: string; nama: string; terisi: boolean }[];
}

export interface RincianKomoditasTugas {
  kode: string;
  nama: string;
  /** Benar bila sudah ada entri harga tertaut ke tugas untuk komoditas ini. */
  terisi: boolean;
}

export interface TugasDetail extends TugasLapangan {
  rincian: RincianKomoditasTugas[];
}

export function toTugasDetail(t: TugasDetailResponse): TugasDetail {
  return { ...toTugasLapangan(t), rincian: t.rincian_komoditas.map((r) => ({ kode: r.kode, nama: r.nama, terisi: r.terisi })) };
}

/**
 * Tautan ke form input untuk mengisi tugas; dipakai daftar dan detail agar parameternya selalu sama. Parameter
 * `tugas` hanya label yang dibaca manusia ("Tugas TGS-mahasiswa_1-1: 2/4 komoditas"), jadi memakai kode tampilan bila ada
 * (`kode ?? id`); server menerima keduanya sebagai rujukan tugas.
 */
export function hrefIsiTugas(t: Pick<TugasLapangan, "id" | "kode" | "pasarId" | "komoditas" | "tipe">): string {
  return `/lapangan/input?pasar=${t.pasarId}&komoditas=${t.komoditas.join(",")}&tipe=${t.tipe}&tugas=${encodeURIComponent(t.kode ?? t.id)}`;
}

export function toTugasLapangan(t: TugasLapanganResponse): TugasLapangan {
  return {
    id: t.id,
    ...(t.kode ? { kode: t.kode } : {}),
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

/** Isi POST /v1/lapangan/harga dari entri antrean perangkat (foto menyusul di 07-C). */
export function toKirimHargaBody(e: EntriLokal) {
  return {
    komoditas_kode: e.komoditasKode,
    pasar_id: e.pasarId,
    tipe: e.tipe,
    harga: e.harga,
    kualitas: e.kualitas,
    kode_pedagang: e.kodePedagang,
    catatan: e.catatan,
    dicatat_pada: e.dicatatPada,
    lat: e.lat,
    lng: e.lng,
    pertahankan: e.pertahankan || undefined,
  };
}

/** Isi 409: harga petugas lain yang bentrok. */
export interface BentrokResponse {
  versi_server?: { id: string; harga: number; dicatat_pada: string; petugas: string };
}

export function toVersiServer(payload: unknown): VersiServer | undefined {
  const versi = (payload as { data?: BentrokResponse } | undefined)?.data?.versi_server;
  return versi ? { harga: versi.harga, dicatatPada: versi.dicatat_pada, petugas: versi.petugas } : undefined;
}

export interface DataAcuanResponse {
  versi: string;
  toleransi: number;
  komoditas: { kode: string; nama: string; satuan: string; harga_acuan: number }[];
  pasar: { id: string; nama: string; kabupaten_kode: string; lat: number; lng: number; radius_meter: number }[];
  harga_acuan: { komoditas_kode: string; kabupaten_kode: string; tipe: TipeHarga; median: number; bawah: number; atas: number; sampel: number }[];
}

export interface Acuan {
  median: number;
  bawah: number;
  atas: number;
}

const RASIO_PRODUSEN = 0.72;

/** Rentang wajar satu komoditas: median resmi kabupaten bila ada, selain itu harga acuan master (produsen 72%). */
export function cariAcuan(data: DataAcuanResponse, kode: string, kabupatenKode: string, tipe: TipeHarga): Acuan | undefined {
  const median = data.harga_acuan.find((h) => h.komoditas_kode === kode && h.kabupaten_kode === kabupatenKode && h.tipe === tipe);
  if (median) return { median: median.median, bawah: median.bawah, atas: median.atas };
  const dasar = data.komoditas.find((k) => k.kode === kode)?.harga_acuan;
  if (!dasar) return undefined;
  const nilai = tipe === "PRODUSEN" ? dasar * RASIO_PRODUSEN : dasar;
  const bulat = (n: number) => Math.round(n / 100) * 100;
  return { median: Math.round(nilai), bawah: bulat(nilai * (1 - data.toleransi)), atas: bulat(nilai * (1 + data.toleransi)) };
}

export interface KodeEtikResponse {
  versi: string;
  judul: string;
  isi: string;
  sudah_setuju: boolean;
  disetujui_pada?: string;
}

export interface PersetujuanKodeEtikResponse {
  id: string;
  versi: string;
  disetujui_pada: string;
}

export interface KinerjaPetugasResponse {
  petugas_id: string;
  nama_petugas: string;
  kabupaten_kode?: string | null;
  total_entri: number;
  entri_valid: number;
  entri_ditolak: number;
  rasio_penolakan_pct: number;
  total_tugas: number;
  tugas_selesai: number;
  tugas_tepat_waktu: number;
  ketepatan_tenggat_pct: number;
}

const MOCK_KODE_ETIK: KodeEtikResponse = {
  versi: "v1.0",
  judul: "Kode Etik Pencacah Harga Pangan Lapangan (TPID Aceh)",
  isi:
    "1. Kejujuran & Akurasi Data: Petugas wajib mencatat harga riil yang terjadi di pasar sesuai satuan, kualitas, dan waktu pencatatan sebenarnya tanpa manipulasi.\n" +
    "2. Kehadiran Fisik (Geofence): Pencacahan dilakukan langsung di lokasi pasar yang ditugaskan serta melampirkan foto bukti kondisi komoditas atau papan harga bila diperlukan.\n" +
    "3. Kerahasiaan Narasumber: Identitas pedagang atau responden pasar dijaga kerahasiaannya.\n" +
    "4. Bebas Konflik Kepentingan: Petugas dilarang menerima imbalan untuk memengaruhi angka pencatatan.\n" +
    "5. Kepatuhan Jadwal: Tugas survei diselesaikan sesuai tenggat waktu yang ditetapkan.",
  sudah_setuju: false,
};

export async function fetchKodeEtik(): Promise<KodeEtikResponse> {
  if (MODE_MOCK) {
    return MOCK_KODE_ETIK;
  }
  return apiFetch<KodeEtikResponse>("/v1/lapangan/kode-etik");
}

export async function setujuiKodeEtik(versi: string): Promise<PersetujuanKodeEtikResponse> {
  if (MODE_MOCK) {
    return { id: "1", versi, disetujui_pada: new Date().toISOString() };
  }
  return apiFetch<PersetujuanKodeEtikResponse>("/v1/lapangan/kode-etik/setuju", {
    method: "POST",
    body: JSON.stringify({ versi }),
  });
}

export async function fetchKinerjaPetugas(): Promise<KinerjaPetugasResponse[]> {
  if (MODE_MOCK) {
    return [];
  }
  return apiFetch<KinerjaPetugasResponse[]>("/v1/lapangan/kinerja");
}
