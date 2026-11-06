import { galatAuth, type GalatAuth } from "./pesan-login";

/** Panjang token wajar (UUID); nilai jauh di atasnya pasti bukan tautan dari email. */
const MAX_TOKEN_LENGTH = 200;

/** Membaca token dari query halaman; null bila tidak ada atau tak masuk akal. */
export function ambilToken(raw: string | string[] | undefined): string | null {
  const nilai = (Array.isArray(raw) ? raw[0] : raw)?.trim();
  return nilai && nilai.length <= MAX_TOKEN_LENGTH ? nilai : null;
}

/** Galat form atur ulang: bentuk umum galat auth; `tautanBaru` true bila tautan tak bisa dipakai lagi. */
export type GalatReset = GalatAuth;

/** Menerjemahkan jawaban gagal BFF atur ulang menjadi pesan dan tindakan lanjut. */
export function galatReset(
  status: number | undefined,
  pesan: string | undefined,
  info: { kode?: string; detik?: number } = {},
): GalatReset {
  return galatAuth({ status, pesan, ...info }, "reset");
}
