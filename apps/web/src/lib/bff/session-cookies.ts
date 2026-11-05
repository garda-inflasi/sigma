/**
 * Cookie sesi untuk pemulihan (refresh): access token baru, refresh token pengganti bila ada, token CSRF bila
 * hilang, dan penanda pemulihan untuk navigasi.
 */
import type { NextResponse } from "next/server";
import {
  CSRF_COOKIE,
  RESUME_COOKIE,
  RESUME_COOKIE_MAX_AGE,
  cookieSecure,
} from "./cookie-policy";
import { setAccessTokenCookie, setRefreshTokenCookie } from "./cookies";
import { generateCSRFToken, setCSRFCookie } from "./csrf";

export interface SesiBaru {
  accessToken: string;
  /** Kosong pada pemutaran ulang dalam jendela toleransi: cookie refresh yang ada dibiarkan. */
  refreshToken?: string;
}

/**
 * Memasang hasil refresh pada respons. Cookie refresh hanya ditimpa bila server menerbitkan token
 * pengganti: pada pemutaran ulang (tab kedua) token pemenang sudah ada di cookie bersama dan tidak boleh
 * tertimpa. Token CSRF (umur 24 jam, lebih pendek dari sesi 7 hari) dibuat ulang hanya bila sudah hilang,
 * supaya tab lain yang sedang mengirim permintaan tidak mendadak memakai nilai usang.
 */
export async function pasangSesiBaru(
  response: NextResponse,
  sesi: SesiBaru,
  csrfSudahAda: boolean,
): Promise<void> {
  setAccessTokenCookie(response, sesi.accessToken);
  if (sesi.refreshToken) setRefreshTokenCookie(response, sesi.refreshToken);
  if (!csrfSudahAda) setCSRFCookie(response, await generateCSRFToken());
}

export function csrfSudahAda(request: {
  cookies: { get(name: string): { value: string } | undefined };
}): boolean {
  return Boolean(request.cookies.get(CSRF_COOKIE)?.value);
}

/** Penanda pemulihan 20 detik (lihat RESUME_COOKIE); dipasang hanya oleh GET /api/auth/refresh. */
export function setResumeCookie(response: NextResponse): void {
  response.cookies.set(RESUME_COOKIE, "1", {
    httpOnly: true,
    secure: cookieSecure(),
    sameSite: "lax",
    path: "/",
    maxAge: RESUME_COOKIE_MAX_AGE,
  });
}
