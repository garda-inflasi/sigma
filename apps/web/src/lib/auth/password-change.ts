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

/**
 * Membaca klaim `wajib_ganti_password` dari payload JWT tanpa memverifikasi
 * tanda tangan (edge tidak memegang rahasia). Hanya untuk pengalihan UX.
 */
export function requiresPasswordChange(
  accessToken: string | undefined,
): boolean {
  const payload = accessToken?.split(".")[1];
  if (!payload) return false;
  try {
    const json = atob(payload.replace(/-/g, "+").replace(/_/g, "/"));
    return (
      (JSON.parse(json) as { wajib_ganti_password?: unknown })
        .wajib_ganti_password === true
    );
  } catch {
    return false;
  }
}

/** Tujuan setelah masuk: halaman ganti kata sandi (membawa tujuan semula) atau tujuan biasa. */
export function postLoginDestination(
  wajibGantiPassword: boolean,
  redirectTo: string,
): string {
  if (!wajibGantiPassword) return redirectTo;
  return `${PASSWORD_CHANGE_PATH}?lanjut=${encodeURIComponent(redirectTo)}`;
}
