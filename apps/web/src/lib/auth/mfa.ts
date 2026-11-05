/**
 * Alur MFA di sisi web. Penegakan sebenarnya ada di API Go (403 MFA_SETUP_REQUIRED bagi akun yang
 * wajib MFA tetapi belum setup, dan token tantangan untuk akun ber-MFA); di sini hanya mengalihkan
 * pengguna ke halaman yang tepat dan merapikan masukan.
 */
import { PASSWORD_CHANGE_PATH, postLoginDestination } from "./password-change";

export const MFA_SETUP_PATH = "/mfa";
export const MFA_SETUP_REQUIRED_CODE = "MFA_SETUP_REQUIRED";

/** Nama cookie httpOnly berumur pendek yang membawa token tantangan MFA antara login dan langkah OTP. */
export const MFA_CHALLENGE_COOKIE = "garda_mfa";
/** Umur token tantangan di server (5 menit). */
export const MFA_CHALLENGE_MAX_AGE = 5 * 60;

/** Prefiks rute yang tetap terbuka bagi akun yang wajib setup MFA. */
const EXEMPT_PREFIXES = [MFA_SETUP_PATH, PASSWORD_CHANGE_PATH, "/api/auth/"];

export function isMfaSetupExempt(pathname: string): boolean {
  return EXEMPT_PREFIXES.some((prefix) => pathname.startsWith(prefix));
}


/**
 * Tujuan setelah sesi terbit: ganti kata sandi lebih dulu bila diwajibkan, lalu setup MFA bila
 * diwajibkan, selain itu tujuan biasa. Setiap halaman wajib membawa tujuan berikutnya di `lanjut`.
 */
export function destinasiSetelahMasuk(
  user: { wajib_ganti_password?: boolean },
  mfaSetupDiwajibkan: boolean | undefined,
  redirectTo: string,
): string {
  const tujuan = mfaSetupDiwajibkan
    ? `${MFA_SETUP_PATH}?lanjut=${encodeURIComponent(redirectTo)}`
    : redirectTo;
  return postLoginDestination(user.wajib_ganti_password === true, tujuan);
}

const KODE_TOTP = /^[0-9]{6}$/;

/** Merapikan kode yang diketik: kode OTP tanpa spasi, kode cadangan huruf besar dengan tanda hubung. */
export function rapikanKode(input: string): string {
  const bersih = input.replace(/\s+/g, "");
  return KODE_TOTP.test(bersih) ? bersih : bersih.toUpperCase();
}

/** Galat isian kode OTP/cadangan, atau null bila boleh dikirim; server yang menilai kebenarannya. */
export function validasiKode(input: string): string | null {
  const kode = rapikanKode(input);
  if (!kode) return "Kode wajib diisi.";
  return kode.length > 32 ? "Kode terlalu panjang." : null;
}

export interface GalatMfa {
  pesan: string;
  /** True bila sesi verifikasi tak bisa dilanjutkan dan pengguna harus mengulang dari kata sandi. */
  ulangMasuk: boolean;
}

/** Menerjemahkan jawaban gagal BFF langkah OTP menjadi pesan dan tindakan lanjut. */
export function galatLangkahOtp(status: number | undefined, pesan: string | undefined): GalatMfa {
  if (status === 401 && /masuk ulang/i.test(pesan ?? "")) {
    return { pesan: "Sesi verifikasi berakhir. Masuk ulang dengan kata sandi Anda.", ulangMasuk: true };
  }
  if (status === 401) return { pesan: "Kode tidak valid. Periksa kode atau tunggu kode berikutnya.", ulangMasuk: false };
  if (status === 429) return { pesan: "Terlalu banyak percobaan. Akun terkunci sementara, coba lagi nanti.", ulangMasuk: false };
  if (status !== undefined && status < 500 && pesan) return { pesan, ulangMasuk: false };
  return { pesan: "Tidak dapat terhubung ke server. Coba lagi.", ulangMasuk: false };
}

/** Teks unduhan kode cadangan: satu kode per baris dengan catatan penyimpanan. */
export function teksKodeCadangan(kode: string[], email: string): string {
  return [
    `Kode cadangan MFA Garda Inflasi (${email})`,
    "Setiap kode hanya dapat dipakai sekali. Simpan di tempat aman, terpisah dari kata sandi.",
    "",
    ...kode,
    "",
  ].join("\n");
}
