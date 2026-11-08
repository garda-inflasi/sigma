/**
 * Isian form "Data pribadi": membandingkan apa yang diketik dengan nilai tersimpan, memvalidasinya dengan aturan
 * yang sama seperti API, dan menyusun isian PATCH /auth/me yang hanya memuat field yang berubah. Murni (tanpa
 * React) supaya aturannya teruji tanpa DOM.
 */
import type { MeUserResponse, UpdateProfilRequest } from "@/lib/bff/api-client";
import { validasiNama } from "./nama";
import { formatTelepon, normalisasiTelepon } from "./telepon";

/** Bagian profil tersimpan yang dapat diubah lewat form. */
export type DataDiri = Pick<MeUserResponse, "nama" | "telepon">;

/** Nilai kotak isian apa adanya. */
export interface IsianProfil {
  nama: string;
  telepon: string;
}

/** Hanya field yang sudah diketik pengguna; field lain mengikuti nilai tersimpan. */
export type DrafProfil = Partial<IsianProfil>;

export interface GalatIsian {
  nama?: string;
  telepon?: string;
}

export interface HasilPeriksa {
  /** Hanya field yang valid dan berbeda dari nilai tersimpan; kosong bila tidak ada yang perlu disimpan. */
  perubahan: UpdateProfilRequest;
  galat: GalatIsian;
}

/** Telepon tersimpan dalam bentuk baku; nilai lama yang tidak baku (data warisan) dibandingkan apa adanya. */
function teleponBaku(tersimpan: string | undefined): string {
  if (!tersimpan) return "";
  const hasil = normalisasiTelepon(tersimpan);
  return hasil.ok ? (hasil.nilai ?? "") : tersimpan;
}

/** Nilai kotak isian saat tidak ada draf: nama tersimpan dan telepon dalam format yang mudah dibaca. */
export function isianDari(tersimpan: DataDiri): IsianProfil {
  return {
    nama: tersimpan.nama,
    telepon: tersimpan.telepon ? formatTelepon(tersimpan.telepon) : "",
  };
}

/**
 * Memeriksa draf terhadap nilai tersimpan. Field yang belum disentuh tidak dinilai: nilai tersimpan yang
 * tidak lagi memenuhi aturan (data warisan) tidak boleh menghalangi perubahan field lain. Telepon yang
 * dikosongkan menjadi `""` (hapus nomor) dan hanya dihitung perubahan bila memang ada nomor tersimpan.
 */
export function periksaDraf(
  tersimpan: DataDiri,
  draf: DrafProfil,
): HasilPeriksa {
  const perubahan: UpdateProfilRequest = {};
  const galat: GalatIsian = {};

  if (draf.nama !== undefined) {
    const nama = validasiNama(draf.nama);
    if (!nama.ok) galat.nama = nama.pesan;
    else if (nama.nilai !== tersimpan.nama) perubahan.nama = nama.nilai;
  }

  if (draf.telepon !== undefined) {
    const telepon = normalisasiTelepon(draf.telepon);
    if (!telepon.ok) galat.telepon = telepon.pesan;
    else if ((telepon.nilai ?? "") !== teleponBaku(tersimpan.telepon)) {
      perubahan.telepon = telepon.nilai ?? "";
    }
  }

  return { perubahan, galat };
}

/** Salinan profil dengan perubahan diterapkan, untuk pembaruan optimistis sebelum server menjawab. */
export function terapkanPerubahan(
  profil: MeUserResponse,
  perubahan: UpdateProfilRequest,
): MeUserResponse {
  const hasil: MeUserResponse = { ...profil };
  if (perubahan.nama !== undefined) hasil.nama = perubahan.nama;
  if (perubahan.telepon !== undefined) {
    const telepon = normalisasiTelepon(perubahan.telepon);
    if (telepon.ok) {
      if (telepon.nilai === null) delete hasil.telepon;
      else hasil.telepon = telepon.nilai;
    }
  }
  return hasil;
}
