/**
 * Aktivasi akun dari tautan undangan (`/aktivasi?token=…`): token dari query dan galat form. Skema kata sandi
 * sama dengan atur ulang (skemaSandiBaru: kebijakan di password-policy.ts, server memeriksa ulang).
 */
import { galatAuth, type GalatAuth } from "./pesan-login";
import { ambilToken } from "./reset-password";

export { ambilToken as ambilTokenAktivasi };
export { skemaSandiBaru as skemaAktivasi } from "./skema-sandi";

/** Halaman tujuan tautan di email undangan (sama dengan email.AktivasiPath di API Go). */
export const HALAMAN_AKTIVASI = "/aktivasi";

/** Galat form aktivasi; `tautanBaru` true bila tautan tak bisa dipakai lagi dan perlu undangan baru dari admin. */
export type GalatAktivasi = GalatAuth;

/**
 * Menerjemahkan jawaban gagal BFF aktivasi. 401 berarti tautan tidak sah, sudah dipakai, diganti undangan
 * baru, atau kedaluwarsa (pesan server membedakan kedaluwarsa); konteksnya sama dengan atur ulang.
 */
export function galatAktivasi(
  status: number | undefined,
  pesan: string | undefined,
  info: { kode?: string; detik?: number } = {},
): GalatAktivasi {
  return galatAuth({ status, pesan, ...info }, "reset");
}
