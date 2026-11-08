/**
 * Pemangkasan isian profil yang mengikuti Go (`strings.TrimSpace` dan `unicode.IsSpace`), bukan `trim()` dan `\s`
 * JavaScript. Himpunan spasinya berbeda tipis (JavaScript menyertakan U+FEFF dan tidak menyertakan U+0085),
 * sedangkan aturan nama dan telepon harus memutuskan hal yang sama di web dan API.
 */

/** Karakter spasi menurut `unicode.IsSpace` (properti White_Space), siap disisipkan ke dalam kelas karakter regex. */
export const SPASI_GO =
  "\\t\\n\\v\\f\\r \\u0085\\u00a0\\u1680\\u2000-\\u200a\\u2028\\u2029\\u202f\\u205f\\u3000";

const SPASI_DI_TEPI = new RegExp(`^[${SPASI_GO}]+|[${SPASI_GO}]+$`, "g");

/** Membuang spasi di awal dan akhir (seperti `strings.TrimSpace`). */
export function pangkas(teks: string): string {
  return teks.replace(SPASI_DI_TEPI, "");
}
