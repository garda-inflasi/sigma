/**
 * Masuk dengan Google (OIDC) di sisi web: kode galat yang dibawa `/masuk?error=<kode>` beserta pesan
 * yang terbaca, dan alamat awal alur. Logika sebenarnya (state, nonce, PKCE, verifikasi ID token,
 * pemetaan akun) ada di API Go; BFF (`lib/bff/sso.ts`) hanya menjembatani cookie dan pengalihan.
 */

/** Rute BFF yang memulai alur (GET): mengalihkan browser ke Google. */
export const SSO_GOOGLE_START_PATH = "/api/auth/sso/google";

/** Rute BFF tujuan pengalihan Google; harus sama dengan SSO_GOOGLE_REDIRECT_URL di API. */
export const SSO_GOOGLE_CALLBACK_PATH = "/api/auth/sso/google/callback";

/** Kode galat pada `?error=` untuk SSO; selalu berawalan `sso_`. */
export const SSO_ERROR_PREFIX = "sso_";

const PESAN_GALAT_SSO: Record<string, string> = {
  sso_tidak_aktif: "Masuk dengan Google belum diaktifkan di lingkungan ini.",
  sso_tidak_tersedia:
    "Google tidak dapat dihubungi saat ini. Coba lagi sebentar lagi atau masuk dengan kata sandi.",
  sso_sesi_tidak_valid:
    "Sesi masuk dengan Google tidak valid atau sudah kedaluwarsa. Silakan mulai lagi.",
  sso_verifikasi_gagal: "Verifikasi identitas Google gagal. Silakan coba lagi.",
  sso_email_belum_terverifikasi:
    "Email akun Google ini belum terverifikasi, sehingga tidak dapat dipakai untuk masuk.",
  sso_domain_ditolak:
    "Akun Google ini bukan dari domain yang diizinkan. Gunakan akun institusi Anda.",
  sso_akun_tidak_ditemukan:
    "Tidak ada akun Garda Inflasi untuk email Google ini. Hubungi Administrator Sistem untuk dibuatkan akun.",
  sso_akun_bentrok:
    "Akun ini sudah tertaut ke akun Google yang berbeda. Hubungi Administrator Sistem.",
  sso_akun_nonaktif: "Akun tidak aktif, hubungi administrator.",
  sso_akun_terkunci:
    "Akun terkunci sementara karena terlalu banyak percobaan. Coba lagi nanti.",
  sso_dibatalkan: "Masuk dengan Google dibatalkan.",
  sso_terlalu_banyak:
    "Terlalu banyak percobaan masuk dengan Google. Coba lagi nanti.",
  sso_gagal: "Masuk dengan Google gagal. Silakan coba lagi.",
};

/** Pesan umum untuk kode `sso_*` yang tidak dikenal (mis. dari tautan lama). */
const PESAN_SSO_UMUM = PESAN_GALAT_SSO.sso_gagal;

/** Apakah nilai `?error=` adalah kode galat SSO. */
export function adalahKodeGalatSso(kode: string | null | undefined): boolean {
  return typeof kode === "string" && kode.startsWith(SSO_ERROR_PREFIX);
}

/**
 * Pesan terbaca untuk kode galat SSO, atau null bila `kode` bukan kode SSO (pemanggil lalu memakai
 * perilaku lama: nilai `?error=` ditampilkan apa adanya). Kode `sso_*` yang tak dikenal diberi pesan umum
 * supaya tautan buatan tidak menampilkan teks sembarang.
 */
export function pesanGalatSso(kode: string | null | undefined): string | null {
  if (!adalahKodeGalatSso(kode)) return null;
  return Object.hasOwn(PESAN_GALAT_SSO, kode as string)
    ? PESAN_GALAT_SSO[kode as string]
    : PESAN_SSO_UMUM;
}

/** Semua kode galat SSO yang dikenal (untuk tes dan dokumentasi). */
export function kodeGalatSsoDikenal(): string[] {
  return Object.keys(PESAN_GALAT_SSO);
}

/**
 * Alamat tombol "Masuk dengan Google". `lanjut` (path internal yang sudah disanitasi pemanggil) dibawa
 * agar pengguna kembali ke halaman yang dituju; tujuan bawaan "/" tidak perlu dibawa.
 */
export function hrefMasukGoogle(lanjut: string): string {
  return lanjut && lanjut !== "/"
    ? `${SSO_GOOGLE_START_PATH}?lanjut=${encodeURIComponent(lanjut)}`
    : SSO_GOOGLE_START_PATH;
}
