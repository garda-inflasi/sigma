/**
 * Nama lengkap profil: dipangkas, 3–100 karakter. Aturannya sama dengan `normalisasiNama` di API Go
 * (usecase/profil.go), begitu pula pesannya. NUL dibuang lebih dulu dan panjang dihitung dalam karakter Unicode
 * (bukan unit UTF-16), seperti hitungan rune di Go, agar emoji atau aksara bersurrogate tidak menggeser batasnya.
 */
import { pangkas } from "./teks";

export const NAMA_MIN = 3;
export const NAMA_MAKS = 100;
export const PESAN_NAMA_PENDEK = `Nama minimal ${NAMA_MIN} karakter`;
export const PESAN_NAMA_PANJANG = `Nama maksimal ${NAMA_MAKS} karakter`;

export type HasilNama =
  { ok: true; nilai: string } | { ok: false; pesan: string };

export function validasiNama(input: string): HasilNama {
  const nilai = pangkas(input.replaceAll("\u0000", ""));
  const panjang = [...nilai].length;
  if (panjang < NAMA_MIN) return { ok: false, pesan: PESAN_NAMA_PENDEK };
  if (panjang > NAMA_MAKS) return { ok: false, pesan: PESAN_NAMA_PANJANG };
  return { ok: true, nilai };
}
