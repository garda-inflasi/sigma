/**
 * Kecocokan path menurut batas segmen: `pathname` tepat sama dengan `dasar` atau berada di bawahnya
 * (`dasar/...`). Awalan tanpa batas segmen salah: `/ganti-passwordX` bukan bagian dari `/ganti-password`.
 */
export function diBawahPath(pathname: string, dasar: string): boolean {
  return pathname === dasar || pathname.startsWith(`${dasar}/`);
}
