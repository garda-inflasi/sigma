/**
 * Riwayat perubahan peran satu pengguna (`GET /v1/users/{id}/riwayat-peran`, dibaca API dari audit trail):
 * bentuk respons API dan pemetaannya ke model tampilan.
 */

import { ROLES, type Role } from "@/lib/rbac/roles";

/** Satu perubahan peran sebagaimana dikirim backend Go. */
export interface PerubahanPeranResponse {
  /** ID baris audit sumbernya. */
  id: string;
  waktu: string;
  aksi: string;
  /** Null untuk peran awal saat akun dibuat. */
  dari: string | null;
  ke: string;
  /** Null bila aksinya tanpa pelaku; `id` null bila akun pelaku sudah dihapus. */
  aktor: { id: string | null; nama: string | null } | null;
}

export interface RiwayatPeranResponse {
  items: PerubahanPeranResponse[];
  /** True bila ada entri lebih lama di luar batas jumlah. */
  terpotong: boolean;
}

export interface EntriRiwayatPeran {
  /** ID baris audit (kunci daftar dan tautan ke detail audit). */
  id: string;
  waktu: string;
  /** Judul ringkas jenis perubahan. */
  judul: string;
  /** Label peran sebelumnya; null untuk peran awal. */
  dari: string | null;
  ke: string;
  /** Pelaku siap tampil. */
  aktor: string;
}

export interface RiwayatPeran {
  entri: EntriRiwayatPeran[];
  terpotong: boolean;
}

const JUDUL_AKSI: Record<string, string> = {
  CREATE_USER: "Akun dibuat",
  ASSIGN_ROLE: "Peran diganti",
  UPDATE_USER: "Data pengguna diubah",
};

/** Judul perubahan; aksi yang belum dikenal ditampilkan apa adanya supaya tetap bisa dilacak. */
export function judulAksiPeran(aksi: string, dari: string | null): string {
  if (aksi === "UPDATE_USER" && dari === null) return "Peran awal ditetapkan";
  return JUDUL_AKSI[aksi] ?? aksi;
}

const adaPeran = (id: string): id is Role =>
  Object.prototype.hasOwnProperty.call(ROLES, id);

/** Label peran untuk tampilan; id yang tak dikenal (peran lama) ditampilkan apa adanya. */
export function labelPeran(id: string): string {
  return adaPeran(id) ? ROLES[id].label : id;
}

/** Pelaku siap tampil: nama, "(akun dihapus)" bila akunnya sudah tiada, atau "Sistem" tanpa pelaku. */
export function namaAktor(aktor: PerubahanPeranResponse["aktor"]): string {
  if (!aktor || (aktor.id === null && !aktor.nama)) return "Sistem";
  if (!aktor.nama) return "Pengguna tidak dikenal";
  return aktor.id === null ? `${aktor.nama} (akun dihapus)` : aktor.nama;
}

/** Tautan ke audit trail yang disaring pada pengguna ini (halaman /audit membaca parameter kueri ini). */
export function tautanAuditPengguna(userId: string): string {
  return `/audit?${new URLSearchParams({ entitas: "pengguna", entitas_id: userId })}`;
}

/** RiwayatPeranResponse (API) → model tampilan. Urutan dari server (terbaru dulu) dipertahankan. */
export function toRiwayatPeran(res: RiwayatPeranResponse): RiwayatPeran {
  return {
    terpotong: res.terpotong,
    entri: res.items.map((p) => ({
      id: p.id,
      waktu: p.waktu,
      judul: judulAksiPeran(p.aksi, p.dari),
      dari: p.dari === null ? null : labelPeran(p.dari),
      ke: labelPeran(p.ke),
      aktor: namaAktor(p.aktor),
    })),
  };
}
