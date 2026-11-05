/**
 * Tujuan pengalihan setelah masuk harus berupa path internal. Parameter `lanjut`
 * berasal dari URL (dapat dikendalikan penyerang), jadi `//evil.com`, `/\evil.com`,
 * dan URL absolut ditolak untuk mencegah open redirect.
 */
export function safeInternalPath(
  raw: string | null | undefined,
  fallback = "/",
): string {
  if (!raw || !raw.startsWith("/")) return fallback;
  if (raw.startsWith("//") || raw.startsWith("/\\")) return fallback;
  // eslint-disable-next-line no-control-regex
  if (/[\u0000-\u001f\u007f]/.test(raw)) return fallback;
  return raw;
}
