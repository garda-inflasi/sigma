/**
 * Kekuatan kata sandi untuk meter di form ganti sandi. Turunan murni dari kebijakan (`evaluatePassword`) dan
 * panjang, jadi meter tidak pernah menyebut "sedang" atau "kuat" untuk kata sandi yang akan ditolak server:
 *
 *   - lemah  : belum memenuhi seluruh aturan kebijakan;
 *   - sedang : memenuhi semua aturan, panjang di bawah `PANJANG_SANDI_KUAT`;
 *   - kuat   : memenuhi semua aturan dan panjang sekurangnya `PANJANG_SANDI_KUAT`.
 *
 * Ambang 12 dipilih karena kebijakan hanya menuntut 8 karakter: panjang tambahan adalah satu-satunya penambah
 * ketahanan yang tidak bergantung pada pola yang mudah ditebak (kapital di awal, angka dan simbol di akhir).
 * Kata sandi kosong tidak punya kekuatan (null): meter disembunyikan.
 */
import { evaluatePassword } from "./password-policy";

export type KekuatanSandi = "lemah" | "sedang" | "kuat";

/** Panjang minimal kata sandi "kuat" (kebijakan sendiri menuntut `MIN_PASSWORD_LENGTH`). */
export const PANJANG_SANDI_KUAT = 12;

/** Nilai untuk `aria-valuenow` meter (rentang 0..3; kosong tidak ditampilkan). */
export const SKOR_KEKUATAN: Record<KekuatanSandi, 1 | 2 | 3> = {
  lemah: 1,
  sedang: 2,
  kuat: 3,
};

export const LABEL_KEKUATAN: Record<KekuatanSandi, string> = {
  lemah: "Lemah",
  sedang: "Sedang",
  kuat: "Kuat",
};

export function kekuatanSandi(password: string): KekuatanSandi | null {
  if (password === "") return null;
  if (!evaluatePassword(password).every((aturan) => aturan.terpenuhi))
    return "lemah";
  return password.length >= PANJANG_SANDI_KUAT ? "kuat" : "sedang";
}
