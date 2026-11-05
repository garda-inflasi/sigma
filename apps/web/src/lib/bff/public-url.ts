/**
 * Alamat absolut di origin yang sedang dikunjungi. Di belakang Caddy, host dan protokol publik datang dari
 * header (`request.url` adalah alamat internal Next), jadi redirect harus dibangun dari header itu.
 */
export function urlPublik(
  request: { headers: { get(name: string): string | null } },
  path: string,
): string {
  const host =
    request.headers.get("x-forwarded-host") ||
    request.headers.get("host") ||
    "localhost";
  const protocol = request.headers.get("x-forwarded-proto") || "http";
  return `${protocol}://${host}${path}`;
}
