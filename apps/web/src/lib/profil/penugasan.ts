/**
 * Teks tampilan untuk ringkasan akun dan "Penugasan & wilayah" di profil: status akun, peran, lapis, dan
 * cakupan wilayah. Semuanya hanya-baca; yang menetapkannya administrator.
 */
import type { Tone } from "@/lib/design/tokens";
import {
  LAPIS_META,
  ROLES,
  type Cakupan,
  type Lapis,
  type Role,
} from "@/lib/rbac/roles";

export interface LabelStatus {
  label: string;
  tone: Tone;
}

const STATUS_AKUN: Record<string, LabelStatus> = {
  AKTIF: { label: "Aktif", tone: "normal" },
  MENUNGGU: { label: "Menunggu aktivasi", tone: "waspada" },
  NONAKTIF: { label: "Nonaktif", tone: "netral" },
};

/** Status akun dari API; nilai yang tidak dikenal tampil sebagai kata biasa ("DIBEKUKAN" → "Dibekukan"). */
export function labelStatusAkun(status: string): LabelStatus {
  if (Object.hasOwn(STATUS_AKUN, status)) return STATUS_AKUN[status];
  const kata = status.trim().toLowerCase();
  return {
    label: kata
      ? kata.charAt(0).toUpperCase() + kata.slice(1)
      : "Tidak diketahui",
    tone: "netral",
  };
}

const adalahPeran = (peran: string): peran is Role =>
  Object.hasOwn(ROLES, peran);
const adalahLapis = (lapis: string): lapis is Lapis =>
  Object.hasOwn(LAPIS_META, lapis);

export interface InfoPeran {
  label: string;
  /** Uraian tugas peran; kosong untuk peran yang tidak dikenal. */
  tugas?: string;
}

export function infoPeran(peran: string): InfoPeran {
  return adalahPeran(peran)
    ? { label: ROLES[peran].label, tugas: ROLES[peran].tugas }
    : { label: peran };
}

/** Label lapis akses, mis. "Lapis operasional wilayah". */
export function labelLapis(lapis: string): string {
  return adalahLapis(lapis) ? LAPIS_META[lapis].label : lapis;
}

const KETERANGAN_CAKUPAN_PERAN: Record<Cakupan, string> = {
  SELURUH: "Melihat data seluruh wilayah",
  KABUPATEN: "Data dibatasi pada kabupaten/kota penugasan",
  SENDIRI: "Hanya data yang Anda input sendiri",
};

export interface CakupanWilayah {
  /** Nama kabupaten/kota, atau "Seluruh Aceh" bila tidak dibatasi kabupaten. */
  wilayah: string;
  /** "Cakupan provinsi" atau "Cakupan kabupaten/kota". */
  tingkat: string;
  /** Batas data menurut peran; kosong untuk peran yang tidak dikenal. */
  batasPeran?: string;
}

/**
 * Cakupan wilayah akun: `kabupaten_kode` kosong berarti cakupan provinsi. `namaKabupaten` dari master wilayah;
 * tanpa nama, kodenya yang tampil.
 */
export function cakupanWilayah(
  kabupatenKode: string | undefined,
  namaKabupaten: string | undefined,
  peran: string,
): CakupanWilayah {
  const batasPeran = adalahPeran(peran)
    ? KETERANGAN_CAKUPAN_PERAN[ROLES[peran].cakupan]
    : undefined;
  if (!kabupatenKode)
    return { wilayah: "Seluruh Aceh", tingkat: "Cakupan provinsi", batasPeran };
  return {
    wilayah: namaKabupaten || kabupatenKode,
    tingkat: "Cakupan kabupaten/kota",
    batasPeran,
  };
}
