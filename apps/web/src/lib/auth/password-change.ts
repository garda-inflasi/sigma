/**
 * Alur wajib ganti kata sandi di sisi web. Penegakan sebenarnya ada di API Go
 * (403 PASSWORD_CHANGE_REQUIRED); di sini hanya mengalihkan pengguna ke halaman
 * ganti kata sandi supaya tidak berhadapan dengan galat 403 di setiap halaman.
 */

export const PASSWORD_CHANGE_PATH = "/ganti-password";
export const PASSWORD_CHANGE_REQUIRED_CODE = "PASSWORD_CHANGE_REQUIRED";

/** Prefiks rute yang tetap terbuka bagi akun yang wajib mengganti kata sandi. */
const EXEMPT_PREFIXES = [PASSWORD_CHANGE_PATH, "/api/auth/"];

export function isPasswordChangeExempt(pathname: string): boolean {
  return EXEMPT_PREFIXES.some((prefix) => pathname.startsWith(prefix));
}


/** Tujuan setelah masuk: halaman ganti kata sandi (membawa tujuan semula) atau tujuan biasa. */
export function postLoginDestination(
  wajibGantiPassword: boolean,
  redirectTo: string,
): string {
  if (!wajibGantiPassword) return redirectTo;
  return `${PASSWORD_CHANGE_PATH}?lanjut=${encodeURIComponent(redirectTo)}`;
}
