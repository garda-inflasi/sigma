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

type Env = Record<string, string | undefined>;

/** Apakah cookie sesi harus diberi atribut `Secure`. */
export function cookieSecure(env: Env = process.env): boolean {
  const explicit = env.COOKIE_SECURE?.trim().toLowerCase();
  if (explicit === "true") return true;
  if (explicit === "false") return false;
  return env.NODE_ENV === "production";
}
