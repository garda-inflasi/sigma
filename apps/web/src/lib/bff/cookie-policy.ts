/**
 * Kebijakan cookie sesi: nama tetap, atribut `Secure` ditentukan lingkungan.
 *
 * Nama cookie sama di semua lingkungan karena dibaca juga oleh middleware,
 * klien (`garda_csrf`), dan backend Go. `Secure` mengikuti `COOKIE_SECURE`
 * ("true" / "false") bila diisi; bila kosong, aktif di produksi. Set
 * `COOKIE_SECURE=false` hanya untuk akses lewat http:// (mis. demo lokal
 * dengan Docker), karena browser menolak cookie Secure di luar HTTPS.
 */

export const ACCESS_COOKIE = "garda_access";
export const REFRESH_COOKIE = "garda_refresh";
export const CSRF_COOKIE = "garda_csrf";
/**
 * Penanda singkat (20 detik) bahwa sesi baru saja dipulihkan lewat `GET /api/auth/refresh`. Edge memakainya
 * sebagai pengaman loop: bila halaman tetap tanpa access token padahal pemulihan baru terjadi, pengguna
 * dikirim ke /masuk, bukan dipulihkan lagi tanpa akhir.
 */
export const RESUME_COOKIE = "garda_pulih";
export const RESUME_COOKIE_MAX_AGE = 20;

/**
 * Refresh token hanya dikirim browser ke `/api/auth` (path cookie), jadi edge/middleware tidak pernah
 * melihatnya pada navigasi halaman. Pemulihan sesi saat navigasi karena itu lewat redirect ke
 * `GET /api/auth/refresh`, yang menerima cookie itu.
 */
export const REFRESH_COOKIE_PATH = "/api/auth";

type Env = Record<string, string | undefined>;

/** Apakah cookie sesi harus diberi atribut `Secure`. */
export function cookieSecure(env: Env = process.env): boolean {
  const explicit = env.COOKIE_SECURE?.trim().toLowerCase();
  if (explicit === "true") return true;
  if (explicit === "false") return false;
  return env.NODE_ENV === "production";
}
