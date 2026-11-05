/**
 * Pemetaan kartu intervensi dari representasi API Go (snake_case) ke model UI.
 * Endpoint: GET/POST /v1/intervensi, GET /v1/intervensi/tahap, GET /v1/intervensi/alur,
 * GET /v1/intervensi/{id}/riwayat, PATCH /v1/intervensi/{id}/tahap.
 */

import type { Tahap } from "@/lib/data/siklus";
import type { KartuIntervensi, Prioritas } from "@/lib/data/types";
import type { Siklus } from "@/lib/design/tokens";
import { inisialNama } from "@/lib/nama";
import type { Permission } from "@/lib/rbac/permissions";

export interface KartuResponse {
  id: string;
  /** Enum basis data: NORMAL | TEKANAN | SURPLUS. */
  siklus: string;
  tahap: string;
  judul: string;
  deskripsi: string;
  komoditas_kode: string;
  kabupaten_kode: string;
  prioritas: Prioritas;
  tenggat: string;
  rekomendasi_ai: boolean;
  progres: { selesai: number; total: number };
  penanggung_jawab: { id: string; nama: string }[];
  ews_alert_id: string | null;
  dibuat_pada: string;
  selesai_pada: string | null;
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
      inisial: inisialNama(p.nama),
    })),
    tenggat: r.tenggat,
    rekomendasiAi: r.rekomendasi_ai,
    progres: { selesai: r.progres.selesai, total: r.progres.total },
    versi: r.versi,
    dieskalasi: r.dieskalasi_pada !== null,
    deskripsi: r.deskripsi || undefined,
    ewsAlertId: r.ews_alert_id ?? undefined,
    dibuatPada: r.dibuat_pada,
    selesaiPada: r.selesai_pada ?? undefined,
  };
}

/* ── Definisi tahap (GET /v1/intervensi/tahap) ─────────────────────────── */

export interface TahapResponse {
  /** Enum basis data: NORMAL | TEKANAN | SURPLUS. */
  siklus: string;
  kode_tahap: string;
  urutan: number;
  label: string;
  petunjuk: string;
  izin_dibutuhkan: string;
  sla_hari: number | null;
}

/**
 * Mengelompokkan definisi tahap dari server per siklus (terurut menurut `urutan`).
 * Izin bawaan intervensi.kelola dibiarkan kosong seperti pada SIKLUS_TAHAP; siklus tak dikenal diabaikan.
 */
export function toTahapPerSiklus(rows: TahapResponse[]): Record<Siklus, Tahap[]> {
  const out: Record<Siklus, Tahap[]> = { normal: [], tekanan: [], surplus: [] };
  for (const r of [...rows].sort((a, b) => a.urutan - b.urutan)) {
    const siklus = SIKLUS.find((s) => s === r.siklus.toLowerCase());
    if (!siklus) continue;
    out[siklus].push({
      id: r.kode_tahap,
      judul: r.label,
      petunjuk: r.petunjuk,
      izinMasuk: r.izin_dibutuhkan === "intervensi.kelola" ? undefined : (r.izin_dibutuhkan as Permission),
      slaHari: r.sla_hari ?? undefined,
    });
  }
  return out;
}

/* ── Alur persetujuan (GET /v1/intervensi/alur) ────────────────────────── */

export interface AlurResponse {
  kode: string;
  nama: string;
  jenis: "PINDAH_TAHAP" | "ESKALASI";
  siklus: string | null;
  tahap_tujuan: string | null;
  aktif: boolean;
  langkah: { urutan: number; label: string; izin_dibutuhkan: string }[];
}

export interface Alur {
  kode: string;
  nama: string;
  jenis: AlurResponse["jenis"];
  /** Kosong pada alur eskalasi (berlaku untuk semua siklus). */
  siklus?: Siklus;
  tahapTujuan?: string;
  aktif: boolean;
  langkah: { urutan: number; label: string; izin: string }[];
}

export function toAlur(r: AlurResponse): Alur {
  return {
    kode: r.kode,
    nama: r.nama,
    jenis: r.jenis,
    siklus: SIKLUS.find((s) => s === r.siklus?.toLowerCase()),
    tahapTujuan: r.tahap_tujuan ?? undefined,
    aktif: r.aktif,
    langkah: [...r.langkah].sort((a, b) => a.urutan - b.urutan).map((l) => ({ urutan: l.urutan, label: l.label, izin: l.izin_dibutuhkan })),
  };
}

/** Alur pemindahan aktif yang dilalui kartu saat masuk ke tahap `tahapId`, bila ada. */
export function alurUntukTahap(alur: Alur[], tahapId: string): Alur | undefined {
  return alur.find((a) => a.aktif && a.jenis === "PINDAH_TAHAP" && a.tahapTujuan === tahapId);
}

/** Rantai langkah satu baris, mis. “Persetujuan Dewan Pengarah → Persetujuan Kanwil DJPb”. */
export function ringkasRantai(a: Alur): string {
  return a.langkah.map((l) => l.label).join(" → ");
}

/**
 * Tahap di depan `tahapKini` yang butuh persetujuan beserta alurnya, urut menurut tahap. Dipakai lembar
 * detail agar pengguna tahu langkah persetujuan apa saja yang akan dilalui kartu.
 */
export function persetujuanDiDepan(alur: Alur[], tahapSiklus: Tahap[], tahapKini: string): { tahap: Tahap; alur: Alur }[] {
  const idx = tahapSiklus.findIndex((t) => t.id === tahapKini);
  const out: { tahap: Tahap; alur: Alur }[] = [];
  for (const t of tahapSiklus.slice(idx + 1)) {
    const a = alurUntukTahap(alur, t.id);
    if (a) out.push({ tahap: t, alur: a });
  }
  return out;
}

/* ── Riwayat kartu (GET /v1/intervensi/{id}/riwayat) ───────────────────── */

export type AksiRiwayat = "BUAT" | "PINDAH_TAHAP" | "TUGASKAN" | "ESKALASI" | "AJUKAN" | "SETUJU" | "TOLAK";

export interface RiwayatResponse {
  id: number;
  aksi: AksiRiwayat;
  dari: string | null;
  ke: string | null;
  oleh_id: string | null;
  oleh_nama: string | null;
  waktu: string;
  alasan: string;
}

export interface RiwayatKartu {
  id: number;
  aksi: AksiRiwayat;
  dari?: string;
  ke?: string;
  /** Kosong berarti dilakukan sistem (kartu otomatis, eskalasi SLA). */
  olehNama?: string;
  waktu: string;
  alasan: string;
}

export function toRiwayat(r: RiwayatResponse): RiwayatKartu {
  return {
    id: r.id,
    aksi: r.aksi,
    dari: r.dari ?? undefined,
    ke: r.ke ?? undefined,
    olehNama: r.oleh_nama ?? undefined,
    waktu: r.waktu,
    alasan: r.alasan,
  };
}

/** Kalimat untuk satu peristiwa riwayat; `labelTahap` menerjemahkan kode tahap ke labelnya. */
export function deskripsiRiwayat(r: RiwayatKartu, labelTahap: (kode?: string) => string): string {
  const ke = r.ke ? `“${labelTahap(r.ke)}”` : "";
  switch (r.aksi) {
    case "BUAT":
      return `Kartu dibuat di tahap ${ke}`.trim();
    case "PINDAH_TAHAP":
      return `Dipindahkan dari “${labelTahap(r.dari)}” ke ${ke}`;
    case "TUGASKAN":
      return `${r.olehNama ?? "Pengguna"} menjadi penanggung jawab`;
    case "ESKALASI":
      return "Dieskalasi ke Dewan Pengarah";
    case "AJUKAN":
      return ke ? `Persetujuan diajukan untuk pindah ke ${ke}` : "Telaah eskalasi diajukan";
    case "SETUJU":
      return ke ? `Persetujuan pindah ke ${ke} disetujui` : "Telaah eskalasi disetujui";
    case "TOLAK":
      return ke ? `Persetujuan pindah ke ${ke} ditolak` : "Telaah eskalasi ditolak";
    default:
      return r.aksi;
  }
}

/* ── Buat kartu (POST /v1/intervensi) ──────────────────────────────────── */

export interface BuatIntervensiInput {
  judul: string;
  deskripsi: string;
  siklus: Siklus;
  prioritas: Prioritas;
  komoditasKode: string;
  kabupatenKode: string;
  /** Tanggal (yyyy-mm-dd) dari input tanggal. */
  tenggat: string;
  penanggungJawabIds: string[];
}

export type GalatBuat = Partial<Record<"judul" | "komoditasKode" | "kabupatenKode" | "tenggat", string>>;

/** Validasi sisi klien yang mencerminkan CreateIntervensiRequest; server tetap berwenang. */
export function validasiBuat(input: BuatIntervensiInput): GalatBuat {
  const galat: GalatBuat = {};
  const judul = input.judul.trim();
  if (!judul) galat.judul = "Judul wajib diisi";
  else if (judul.length > 255) galat.judul = "Judul maksimal 255 karakter";
  if (!input.komoditasKode) galat.komoditasKode = "Pilih komoditas";
  if (!input.kabupatenKode) galat.kabupatenKode = "Pilih wilayah";
  if (!/^\d{4}-\d{2}-\d{2}$/.test(input.tenggat) || Number.isNaN(Date.parse(input.tenggat))) galat.tenggat = "Tenggat wajib diisi";
  return galat;
}

export interface BuatIntervensiBody {
  judul: string;
  deskripsi: string;
  siklus: string;
  prioritas: Prioritas;
  komoditas_kode: string;
  kabupaten_kode: string;
  tenggat: string;
  penanggung_jawab_ids: string[];
}

/** Badan permintaan; tenggat jatuh pada akhir hari tanggal terpilih (WIB, zona Aceh). */
export function toBuatBody(input: BuatIntervensiInput): BuatIntervensiBody {
  return {
    judul: input.judul.trim(),
    deskripsi: input.deskripsi.trim(),
    siklus: input.siklus.toUpperCase(),
    prioritas: input.prioritas,
    komoditas_kode: input.komoditasKode,
    kabupaten_kode: input.kabupatenKode,
    tenggat: new Date(`${input.tenggat}T23:59:00+07:00`).toISOString(),
    penanggung_jawab_ids: input.penanggungJawabIds,
  };
}

export interface KartuBaru {
  id: string;
  tahap: string;
}

export type KeputusanPersetujuan = "SETUJU" | "TOLAK";

export interface PersetujuanResponse {
  id: number;
  intervensi_id: string;
  kartu_judul: string;
  kartu_tahap: string;
  jenis: "PINDAH_TAHAP" | "ESKALASI";
  tahap_tujuan: string | null;
  status: "MENUNGGU" | "DISETUJUI" | "DITOLAK";
  langkah_kini: number;
  izin_langkah_kini: string;
  diajukan_oleh_id: string | null;
  diajukan_oleh_nama: string | null;
  diajukan_pada: string;
  catatan: string;
  langkah: {
    urutan: number;
    label: string;
    izin_dibutuhkan: string;
    keputusan: KeputusanPersetujuan | null;
    oleh_id: string | null;
    oleh_nama: string | null;
    catatan: string;
  }[];
}

export interface LangkahPersetujuan {
  urutan: number;
  label: string;
  izin: string;
  keputusan?: KeputusanPersetujuan;
  olehId?: string;
  olehNama?: string;
  catatan: string;
}

export interface Persetujuan {
  id: number;
  kartuId: string;
  kartuJudul: string;
  jenis: PersetujuanResponse["jenis"];
  tahapTujuan?: string;
  status: PersetujuanResponse["status"];
  langkahKini: number;
  izinLangkahKini: string;
  /** Kosong bila diajukan sistem (eskalasi SLA). */
  diajukanOlehId?: string;
  diajukanOleh?: string;
  diajukanPada: string;
  catatan: string;
  langkah: LangkahPersetujuan[];
}

export function toPersetujuan(r: PersetujuanResponse): Persetujuan {
  return {
    id: r.id,
    kartuId: r.intervensi_id,
    kartuJudul: r.kartu_judul,
    jenis: r.jenis,
    tahapTujuan: r.tahap_tujuan ?? undefined,
    status: r.status,
    langkahKini: r.langkah_kini,
    izinLangkahKini: r.izin_langkah_kini,
    diajukanOlehId: r.diajukan_oleh_id ?? undefined,
    diajukanOleh: r.diajukan_oleh_nama ?? undefined,
    diajukanPada: r.diajukan_pada,
    catatan: r.catatan,
    langkah: r.langkah.map((l) => ({
      urutan: l.urutan,
      label: l.label,
      izin: l.izin_dibutuhkan,
      keputusan: l.keputusan ?? undefined,
      olehId: l.oleh_id ?? undefined,
      olehNama: l.oleh_nama ?? undefined,
      catatan: l.catatan,
    })),
  };
}

/**
 * Apakah pengguna boleh memutuskan langkah yang menunggu: permintaan masih terbuka, ia berizin untuk
 * langkah itu, dan ia bukan pengaju atau pemutus langkah lain (pemisahan tugas, ditegakkan juga di server).
 */
export function bisaMemutus(p: Persetujuan, userId: string, berizin: boolean): boolean {
  return p.status === "MENUNGGU" && berizin && p.diajukanOlehId !== userId && !p.langkah.some((l) => l.olehId === userId);
}
