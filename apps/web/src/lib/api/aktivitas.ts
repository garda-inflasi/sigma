/**
 * Riwayat aktivitas dan riwayat sesi akun sendiri (/profil, tab "Aktivitas Akun"): pemetaan respons API
 * (snake_case) ke model UI, label aksi berbahasa Indonesia, dan klien berkursor yang menembus BFF.
 * Akun selalu dari sesi di server; tidak ada parameter yang memilih akun.
 */

import type {
  AktivitasSayaWire,
  HalamanKursorWire,
  RiwayatSesiWire,
} from "@/lib/bff/api-client";
import type { Tone } from "@/lib/design/tokens";
import { bffFetch } from "./client";

export const UKURAN_HALAMAN = 20;

export type HasilAktivitas = AktivitasSayaWire["hasil"];
export type SebagaiAktivitas = AktivitasSayaWire["sebagai"];

export interface Aktivitas {
  id: string;
  waktu: string;
  aksi: string;
  /** Kalimat Indonesia untuk aksi; kode mentah bila aksi belum dikenal. */
  label: string;
  hasil: HasilAktivitas;
  /** "pelaku": akun ini yang melakukan. "sasaran": orang lain bertindak terhadap akun ini. */
  sebagai: SebagaiAktivitas;
  perangkat: string;
  ip: string;
  /** Peran pelaku pada baris sasaran, mis. "ADMIN_SISTEM". */
  olehPeran: string;
}

export interface HalamanAktivitas {
  items: Aktivitas[];
  nextCursor?: string;
}

/** Aksi yang lazim muncul pada akun sendiri, sebagai pilihan filter dan label. */
export const LABEL_AKSI: Record<string, string> = {
  LOGIN: "Masuk",
  LOGOUT: "Keluar",
  LOGOUT_ALL: "Keluar dari semua perangkat",
  SSO_LOGIN: "Masuk dengan SSO",
  SSO_LINK: "Menautkan akun SSO",
  CHANGE_PASSWORD: "Mengganti kata sandi",
  RESET_PASSWORD: "Mengatur ulang kata sandi",
  UPDATE_PROFIL: "Memperbarui profil",
  CREATE_USER: "Membuat pengguna",
  DELETE_USER: "Menghapus pengguna",
  RESET_PASSWORD_ADMIN: "Mengatur ulang kata sandi pengguna",
  UNDANG_PENGGUNA: "Mengundang pengguna",
  REVOKE_SESSION: "Mencabut sesi",
  REVOKE_OTHER_SESSIONS: "Mengeluarkan perangkat lain",
  TOKEN_REUSE: "Token sesi dipakai ulang (sesi dicabut)",
  AKTIVASI_AKUN: "Mengaktifkan akun",
  MFA_SETUP: "Menyiapkan verifikasi 2 langkah",
  MFA_VERIFY: "Mengaktifkan verifikasi 2 langkah",
  MFA_DISABLE: "Menonaktifkan verifikasi 2 langkah",
  RESET_MFA: "Verifikasi 2 langkah diatur ulang admin",
  FORCE_LOGOUT: "Dikeluarkan paksa oleh admin",
  UNLOCK_USER: "Akun dibuka kuncinya",
  UPDATE_USER: "Akun diperbarui admin",
  ASSIGN_ROLE: "Peran diubah admin",
  GRANT_USER_PERMISSION: "Izin tambahan diberikan",
  REVOKE_USER_PERMISSION: "Izin tambahan dicabut",
  HARGA_AJUKAN: "Mengajukan entri harga",
  HARGA_VALIDASI: "Memvalidasi entri harga",
  HARGA_PUBLIKASI: "Mempublikasikan entri harga",
  HARGA_TOLAK: "Menolak entri harga",
  HARGA_KEMBALIKAN: "Mengembalikan entri harga",
  AJUKAN_PERSETUJUAN: "Mengajukan persetujuan intervensi",
  PUTUS_PERSETUJUAN: "Memutuskan persetujuan intervensi",
  PINDAH_TAHAP: "Memindahkan tahap intervensi",
};

/** Aksi yang ditawarkan di filter (urutan tampil). */
export const AKSI_FILTER = Object.keys(LABEL_AKSI);

export function labelAksi(aksi: string): string {
  return LABEL_AKSI[aksi] ?? aksi;
}

export const LABEL_HASIL: Record<HasilAktivitas, { label: string; tone: Tone }> = {
  BERHASIL: { label: "Berhasil", tone: "normal" },
  GAGAL: { label: "Gagal", tone: "defisit" },
  DITOLAK: { label: "Ditolak", tone: "waspada" },
};

export function toAktivitas(w: AktivitasSayaWire): Aktivitas {
  return {
    id: w.id,
    waktu: w.waktu,
    aksi: w.aksi,
    label: labelAksi(w.aksi),
    hasil: w.hasil,
    sebagai: w.sebagai,
    perangkat: w.perangkat?.trim() ?? "",
    ip: w.ip ?? "",
    olehPeran: w.oleh_peran ?? "",
  };
}

export interface FilterAktivitas {
  aksi?: string;
  hasil?: HasilAktivitas;
  sebagai?: SebagaiAktivitas | "semua";
  /** YYYY-MM-DD */
  dari?: string;
  /** YYYY-MM-DD (inklusif) */
  sampai?: string;
}

/** Query string ke BFF: hanya nilai terisi; `semua` pada sebagai tidak dikirim (itu bawaan server). */
export function queryAktivitas(f: FilterAktivitas, cursor?: string): string {
  const qs = new URLSearchParams();
  if (f.aksi) qs.set("aksi", f.aksi);
  if (f.hasil) qs.set("hasil", f.hasil);
  if (f.sebagai && f.sebagai !== "semua") qs.set("sebagai", f.sebagai);
  if (f.dari) qs.set("dari", f.dari);
  if (f.sampai) qs.set("sampai", f.sampai);
  qs.set("limit", String(UKURAN_HALAMAN));
  if (cursor) qs.set("cursor", cursor);
  return qs.toString();
}

export type StatusSesi = RiwayatSesiWire["status"];

export interface RiwayatSesi {
  id: string;
  perangkat: string;
  ip: string;
  mulaiPada: string;
  terakhirDipakai: string;
  berakhirPada?: string;
  status: StatusSesi;
  saatIni: boolean;
}

export interface HalamanRiwayatSesi {
  items: RiwayatSesi[];
  nextCursor?: string;
}

export const LABEL_STATUS_SESI: Record<StatusSesi, { label: string; tone: Tone }> = {
  AKTIF: { label: "Aktif", tone: "normal" },
  DICABUT: { label: "Dicabut", tone: "waspada" },
  KEDALUWARSA: { label: "Kedaluwarsa", tone: "netral" },
};

export type FilterStatusSesi = "semua" | "aktif" | "berakhir";

export function toRiwayatSesi(w: RiwayatSesiWire): RiwayatSesi {
  return {
    id: w.id,
    perangkat: w.perangkat?.trim() || "Perangkat tidak dikenal",
    ip: w.ip ?? "",
    mulaiPada: w.mulai_pada,
    terakhirDipakai: w.terakhir_dipakai_pada,
    berakhirPada: w.berakhir_pada,
    status: w.status,
    saatIni: w.saat_ini === true,
  };
}

export function queryRiwayatSesi(status: FilterStatusSesi, cursor?: string): string {
  const qs = new URLSearchParams();
  if (status !== "semua") qs.set("status", status);
  qs.set("limit", String(UKURAN_HALAMAN));
  if (cursor) qs.set("cursor", cursor);
  return qs.toString();
}

/** Klien riwayat akun sendiri; semua panggilan lewat Route Handler BFF. */
export const aktivitasApi = {
  aktivitas: async (f: FilterAktivitas, cursor?: string): Promise<HalamanAktivitas> => {
    const r = await bffFetch<HalamanKursorWire<AktivitasSayaWire>>(
      `/api/auth/me/aktivitas?${queryAktivitas(f, cursor)}`,
    );
    return { items: (r.items ?? []).map(toAktivitas), nextCursor: r.next_cursor || undefined };
  },

  riwayatSesi: async (status: FilterStatusSesi, cursor?: string): Promise<HalamanRiwayatSesi> => {
    const r = await bffFetch<HalamanKursorWire<RiwayatSesiWire>>(
      `/api/auth/me/riwayat-sesi?${queryRiwayatSesi(status, cursor)}`,
    );
    return { items: (r.items ?? []).map(toRiwayatSesi), nextCursor: r.next_cursor || undefined };
  },
};
