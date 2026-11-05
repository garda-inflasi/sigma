import { isPasswordValid } from "./password-policy";

/** Panjang token wajar (UUID); nilai jauh di atasnya pasti bukan tautan dari email. */
const MAX_TOKEN_LENGTH = 200;

/** Membaca token dari query halaman; null bila tidak ada atau tak masuk akal. */
export function ambilToken(raw: string | string[] | undefined): string | null {
  const nilai = (Array.isArray(raw) ? raw[0] : raw)?.trim();
  return nilai && nilai.length <= MAX_TOKEN_LENGTH ? nilai : null;
}

/** Galat isian form atur ulang, atau null bila boleh dikirim. */
export function validasiFormReset(
  baru: string,
  konfirmasi: string,
): string | null {
  if (!isPasswordValid(baru))
    return "Kata sandi baru belum memenuhi semua aturan.";
  if (baru !== konfirmasi) return "Konfirmasi kata sandi tidak sama.";
  return null;
}

export interface GalatReset {
  pesan: string;
  /** True bila tautan tidak bisa dipakai lagi (kedaluwarsa, terpakai, atau tidak sah): tawarkan minta tautan baru. */
  tautanBaru: boolean;
}

/** Menerjemahkan jawaban gagal BFF atur ulang menjadi pesan dan tindakan lanjut. */
export function galatReset(
  status: number | undefined,
  pesan: string | undefined,
): GalatReset {
  if (status === 401) {
    return {
      pesan: pesan || "Tautan atur ulang tidak valid atau sudah tidak berlaku.",
      tautanBaru: true,
    };
  }
  if (status === 429)
    return {
      pesan: "Terlalu banyak percobaan. Coba lagi beberapa saat.",
      tautanBaru: false,
    };
  if (status !== undefined && status < 500) {
    return {
      pesan: pesan || "Permintaan tidak dapat diproses.",
      tautanBaru: false,
    };
  }
  return {
    pesan: "Tidak dapat terhubung ke server. Coba lagi.",
    tautanBaru: false,
  };
}
