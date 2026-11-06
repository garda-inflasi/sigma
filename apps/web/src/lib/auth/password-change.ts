/**
 * Alur wajib ganti kata sandi di sisi web. Penegakan sebenarnya ada di API Go
 * (403 PASSWORD_CHANGE_REQUIRED); di sini hanya mengalihkan pengguna ke halaman
 * ganti kata sandi supaya tidak berhadapan dengan galat 403 di setiap halaman.
 */

import { diBawahPath } from "./path-segmen";

export const PASSWORD_CHANGE_PATH = "/ganti-password";
export const PASSWORD_CHANGE_REQUIRED_CODE = "PASSWORD_CHANGE_REQUIRED";

/** Path (beserta anak path-nya) yang tetap terbuka bagi akun yang wajib mengganti kata sandi. */
const EXEMPT_PATHS = [PASSWORD_CHANGE_PATH, "/api/auth"];

/** Cocok menurut batas segmen: `/ganti-password` dan `/ganti-password/...`, bukan `/ganti-passwordX`. */
export function isPasswordChangeExempt(pathname: string): boolean {
  return EXEMPT_PATHS.some((path) => diBawahPath(pathname, path));
}


/** Tujuan setelah masuk: halaman ganti kata sandi (membawa tujuan semula) atau tujuan biasa. */
export function postLoginDestination(
  wajibGantiPassword: boolean,
  redirectTo: string,
): string {
  if (!wajibGantiPassword) return redirectTo;
  return `${PASSWORD_CHANGE_PATH}?lanjut=${encodeURIComponent(redirectTo)}`;
}
