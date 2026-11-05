/**
 * Pembantu BFF untuk masuk dengan Google (OIDC): cookie alur, tujuan pengalihan, dan pemetaan galat.
 *
 * Rancangan penyimpanan: state, nonce, dan code_verifier PKCE tidak pernah sampai ke browser. API Go
 * menyimpannya (state dan binding hanya sebagai hash, sekali pakai, 10 menit) dan hanya memberi BFF sebuah
 * `binding` acak. Binding disimpan di cookie httpOnly berumur 10 menit dan dikirim kembali pada callback,
 * sehingga callback hanya berlaku di browser yang memulai alur (mencegah login CSRF: korban tidak bisa
 * dipaksa memakai kode/state milik penyerang). Cookie ini WAJIB SameSite=Lax: callback datang dari
 * pengalihan lintas situs (Google), dan cookie Strict tidak ikut terkirim pada navigasi seperti itu.
 */

import "server-only";

import type { NextRequest, NextResponse } from "next/server";
import { safeInternalPath } from "@/lib/auth/safe-path";
import { cookieSecure } from "./cookie-policy";
import { validateRedirectUrl } from "./safe-redirect";

/** Umur cookie alur (sama dengan umur state di API). */
export const SSO_FLOW_MAX_AGE = 10 * 60;

/** Kode `?error=` untuk jawaban galat API Go (kode amplop) yang diketahui. */
const KODE_GALAT_API: Record<string, string> = {
  SSO_NOT_CONFIGURED: "sso_tidak_aktif",
  SSO_UNAVAILABLE: "sso_tidak_tersedia",
  SSO_STATE_INVALID: "sso_sesi_tidak_valid",
  SSO_EXCHANGE_FAILED: "sso_verifikasi_gagal",
  SSO_EMAIL_UNVERIFIED: "sso_email_belum_terverifikasi",
  SSO_DOMAIN_NOT_ALLOWED: "sso_domain_ditolak",
  SSO_ACCOUNT_NOT_FOUND: "sso_akun_tidak_ditemukan",
  SSO_ACCOUNT_CONFLICT: "sso_akun_bentrok",
  SSO_ACCOUNT_INACTIVE: "sso_akun_nonaktif",
  SSO_ACCOUNT_LOCKED: "sso_akun_terkunci",
};

const BINDING = /^[A-Za-z0-9_-]{22,128}$/;
const MAKS_PANJANG_LANJUT = 512;
const MAKS_PANJANG_KODE = 2048;
const MAKS_PANJANG_STATE = 128;

// ── Tujuan setelah masuk ───────────────────────────────────────────────

/**
 * Path internal aman untuk `lanjut`. Gabungan `validateRedirectUrl` (batas origin) dan `safeInternalPath`
 * (menolak `//host`, `/\host`, karakter kontrol, dan URL absolut), ditambah penolakan rute `/api/`
 * (bukan halaman). Apa pun yang meragukan jatuh ke "/".
 */
export function lanjutAman(raw: string | null | undefined): string {
  if (!raw || raw.length > MAKS_PANJANG_LANJUT) return "/";
  const lewatAturanBff = validateRedirectUrl(raw);
  if (!lewatAturanBff) return "/";
  const path = safeInternalPath(lewatAturanBff);
  if (path === "/api" || path.startsWith("/api/") || path.startsWith("/api?"))
    return "/";
  return path;
}

/** Path halaman masuk dengan kode galat SSO dan, bila ada, tujuan semula agar bisa dicoba lagi. */
export function urlMasukGalat(kode: string, lanjut = "/"): string {
  const aman = lanjutAman(lanjut);
  return `/masuk?error=${encodeURIComponent(kode)}${aman === "/" ? "" : `&lanjut=${encodeURIComponent(aman)}`}`;
}

/** Path langkah OTP setelah identitas terbukti (sama dengan login kata sandi). */
export function urlLangkahOtp(lanjut: string): string {
  return `/masuk?mfa=1&lanjut=${encodeURIComponent(lanjutAman(lanjut))}`;
}

// ── Cookie alur ────────────────────────────────────────────────────────

/**
 * Nama dan path cookie alur. Saat cookie Secure memakai awalan `__Host-` (wajib Path=/, tanpa Domain),
 * sehingga subdomain lain tidak dapat menanamkan cookie ini; saat http (lokal/demo) nama biasa dengan
 * path sempit.
 */
export function ssoFlowCookie(secure: boolean = cookieSecure()): {
  name: string;
  path: string;
} {
  return secure
    ? { name: "__Host-garda_sso", path: "/" }
    : { name: "garda_sso", path: "/api/auth/sso/google" };
}

/** Isi cookie: `<binding>.<base64url(lanjut)>`. Binding berkarakter base64url sehingga titik memisahkan dengan tegas. */
export function encodeFlow(binding: string, lanjut: string): string {
  return `${binding}.${Buffer.from(lanjutAman(lanjut), "utf8").toString("base64url")}`;
}

export interface SsoFlow {
  binding: string;
  lanjut: string;
}

/** Membaca isi cookie; null bila bentuknya tidak sah. `lanjut` disanitasi ulang karena cookie dapat diubah pengguna. */
export function decodeFlow(raw: string | null | undefined): SsoFlow | null {
  if (!raw || raw.length > 1024) return null;
  const titik = raw.indexOf(".");
  if (titik < 0) return null;
  const binding = raw.slice(0, titik);
  if (!BINDING.test(binding)) return null;
  let lanjut = "/";
  try {
    lanjut = lanjutAman(
      Buffer.from(raw.slice(titik + 1), "base64url").toString("utf8"),
    );
  } catch {
    lanjut = "/";
  }
  return { binding, lanjut };
}

/** Memasang cookie alur pada respons pengalihan ke Google. */
export function setSsoFlowCookie(
  response: NextResponse,
  binding: string,
  lanjut: string,
): void {
  const { name, path } = ssoFlowCookie();
  response.cookies.set(name, encodeFlow(binding, lanjut), {
    httpOnly: true,
    secure: cookieSecure(),
    sameSite: "lax",
    path,
    maxAge: SSO_FLOW_MAX_AGE,
  });
}

/** Menghapus cookie alur; dipanggil pada setiap hasil callback agar binding tidak pernah dipakai dua kali. */
export function clearSsoFlowCookie(response: NextResponse): void {
  const { name, path } = ssoFlowCookie();
  response.cookies.set(name, "", {
    httpOnly: true,
    secure: cookieSecure(),
    sameSite: "lax",
    path,
    maxAge: 0,
  });
}

/** Alur dari cookie permintaan, atau null bila tidak ada/rusak. */
export function readSsoFlow(request: NextRequest): SsoFlow | null {
  return decodeFlow(request.cookies.get(ssoFlowCookie().name)?.value);
}

// ── Parameter callback dan pemetaan galat ─────────────────────────────

export interface ParameterCallback {
  code: string;
  state: string;
}

/** `code` dan `state` dari query callback, atau null bila hilang, kosong, atau tidak wajar panjangnya. */
export function bacaParameterCallback(
  params: URLSearchParams,
): ParameterCallback | null {
  const code = params.get("code") ?? "";
  const state = params.get("state") ?? "";
  if (!code || !state) return null;
  if (code.length > MAKS_PANJANG_KODE || state.length > MAKS_PANJANG_STATE)
    return null;
  return { code, state };
}

/** Kode galat untuk parameter `error` yang dikirim penyedia (pengguna menolak izin, dsb.). */
export function galatSsoDariPenyedia(error: string | null): string {
  return error === "access_denied" ? "sso_dibatalkan" : "sso_gagal";
}

/**
 * Kode `?error=` untuk galat dari API Go. Kode amplop yang dikenal dipetakan satu-satu; 429 menjadi
 * "terlalu banyak"; galat jaringan (API tidak terjangkau) dan 5xx menjadi "tidak tersedia"; sisanya umum.
 */
export function galatSsoDariApi(error: unknown): string {
  const e = error as { status?: unknown; code?: unknown } | null;
  if (typeof e?.code === "string" && Object.hasOwn(KODE_GALAT_API, e.code)) {
    return KODE_GALAT_API[e.code];
  }
  if (typeof e?.status === "number") {
    if (e.status === 429) return "sso_terlalu_banyak";
    if (e.status >= 500) return "sso_tidak_tersedia";
    return "sso_gagal";
  }
  // Tanpa status: fetch gagal (jaringan, timeout) sebelum mendapat jawaban.
  return "sso_tidak_tersedia";
}

/** URL otorisasi dari API hanya diikuti bila http(s); selain itu null. */
export function tujuanPenyedia(authorizeUrl: string): string | null {
  try {
    const url = new URL(authorizeUrl);
    return url.protocol === "https:" || url.protocol === "http:"
      ? url.toString()
      : null;
  } catch {
    return null;
  }
}
