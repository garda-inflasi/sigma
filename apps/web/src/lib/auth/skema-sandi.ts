/**
 * Skema form kata sandi baru (atur ulang dan ganti), satu sumber pesan untuk validasi di klien. Kebijakan
 * kata sandinya sendiri ada di password-policy.ts; server tetap memeriksa ulang.
 */
import { z } from "zod";
import { isPasswordValid } from "./password-policy";

export const PESAN_SANDI_LEMAH = "Kata sandi baru belum memenuhi semua aturan.";
export const PESAN_KONFIRMASI_BEDA = "Konfirmasi kata sandi tidak sama.";
export const PESAN_SANDI_LAMA_KOSONG = "Kata sandi saat ini wajib diisi.";

const sandiBaru = z.string().refine(isPasswordValid, PESAN_SANDI_LEMAH);
const konfirmasiSama = (n: { baru: string; konfirmasi: string }) => n.baru === n.konfirmasi;

/** Atur ulang lewat tautan email: kata sandi baru + konfirmasinya. */
export const skemaSandiBaru = z
  .object({ baru: sandiBaru, konfirmasi: z.string() })
  .refine(konfirmasiSama, { path: ["konfirmasi"], message: PESAN_KONFIRMASI_BEDA });

/** Ganti kata sandi saat masuk: kata sandi saat ini + kata sandi baru + konfirmasinya. */
export const skemaGantiSandi = z
  .object({ lama: z.string().min(1, PESAN_SANDI_LAMA_KOSONG), baru: sandiBaru, konfirmasi: z.string() })
  .refine(konfirmasiSama, { path: ["konfirmasi"], message: PESAN_KONFIRMASI_BEDA });
