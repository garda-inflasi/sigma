import type { NextResponse } from "next/server";
import { destinasiSetelahMasuk } from "@/lib/auth/mfa";
import type { SessionResponse } from "./api-client";
import { setAccessTokenCookie, setRefreshTokenCookie } from "./cookies";
import { generateCSRFToken, setCSRFCookie } from "./csrf";

/** Memasang sesi penuh (token akses, token segar, dan token CSRF) pada respons. */
export async function attachSession(
  response: NextResponse,
  session: Pick<SessionResponse, "access_token" | "refresh_token">,
): Promise<void> {
  setAccessTokenCookie(response, session.access_token);
  setRefreshTokenCookie(response, session.refresh_token);
  setCSRFCookie(response, await generateCSRFToken());
}

/** Tujuan setelah sesi terbit, mengikuti kewajiban ganti kata sandi dan setup MFA. */
export function tujuanSesi(
  session: Pick<SessionResponse, "user" | "mfa_setup_diwajibkan">,
  redirectTo: string,
): string {
  return destinasiSetelahMasuk(
    session.user,
    session.mfa_setup_diwajibkan,
    redirectTo,
  );
}

/** Apakah hasil login/validasi sudah membawa sesi penuh. */
export function adalahSesi(result: {
  access_token?: string;
  refresh_token?: string;
  user?: unknown;
}): result is SessionResponse {
  return !!result.access_token && !!result.refresh_token && !!result.user;
}
