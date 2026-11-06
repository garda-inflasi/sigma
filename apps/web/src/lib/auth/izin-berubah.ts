/**
 * Menyusul perubahan izin (02-C). Access token membawa salinan izin efektif saat terbit; API Go menegakkan izin
 * dari basis data dan menjawab 401 PERMISSIONS_CHANGED bila salinan itu basi, sehingga refresh senyap (01-E)
 * menerbitkan token baru. Modul ini mendeteksi bahwa refresh barusan MENGUBAH izin dan memberi tahu UI (semua
 * tab) agar sesi di layout root dibaca ulang: sidebar, palet perintah, tombol, dan penjaga halaman ikut berubah
 * tanpa muat ulang manual. Murni dan aman diimpor di server maupun klien.
 */

import { decodeJwtPayload } from "./edge-claims";

/** Izin dalam klaim token; null bila token rusak atau tanpa klaim. */
export function izinDalamToken(token: string | undefined): string[] | null {
  const klaim = decodeJwtPayload(token)?.permissions;
  return Array.isArray(klaim) && klaim.every((p) => typeof p === "string")
    ? (klaim as string[])
    : null;
}

/**
 * Apakah himpunan izin token baru berbeda dari token lama. Tanpa token lama (kedaluwarsa dan cookie-nya sudah
 * hilang) tidak ada pembanding: dianggap tidak berubah, karena memuat ulang UI setiap 15 menit tidak berguna.
 */
export function izinBerubah(
  tokenLama: string | undefined,
  tokenBaru: string | undefined,
): boolean {
  const lama = izinDalamToken(tokenLama);
  const baru = izinDalamToken(tokenBaru);
  if (!lama || !baru) return false;
  const a = new Set(lama);
  const b = new Set(baru);
  return a.size !== b.size || [...a].some((p) => !b.has(p));
}

/** Nama peristiwa jendela dan kunci localStorage (untuk tab lain) yang menandai izin sesi berubah. */
export const PERISTIWA_IZIN_BERUBAH = "garda:izin-berubah";
export const KUNCI_IZIN_BERUBAH = "garda:izin:berubah";

/** Memberi tahu tab ini (peristiwa) dan tab lain (peristiwa storage) bahwa izin sesi berubah. Hanya di browser. */
export function umumkanIzinBerubah(): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(KUNCI_IZIN_BERUBAH, String(Date.now()));
  } catch {
    // Mode privat/penyimpanan penuh: tab lain menyusul lewat refresh senyapnya sendiri.
  }
  window.dispatchEvent(new Event(PERISTIWA_IZIN_BERUBAH));
}
