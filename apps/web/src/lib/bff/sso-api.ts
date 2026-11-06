/**
 * Panggilan BFF ke endpoint SSO API Go (server ke server). Berkas terpisah dari api-client.ts agar
 * klien auth yang ada tidak disentuh; bentuk amplop dan galatnya sama (`BffApiError` membawa `code`).
 */

import "server-only";

import { BffApiError, type LoginResponse } from "./api-client";

const API_BASE = process.env.GO_API_BASE_URL || "http://localhost:8080/v1";

interface Amplop<T> {
  code?: string;
  message?: string;
  data?: T;
}

async function panggil<T>(
  path: string,
  init: RequestInit,
  timeoutMs: number,
  forwardedFor?: string | null,
  userAgent?: string | null,
): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    cache: "no-store",
    signal: AbortSignal.timeout(timeoutMs),
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
      // IP klien untuk audit dan batas laju di API (Go hanya mempercayai header ini dari jaringan privat).
      ...(forwardedFor ? { "X-Forwarded-For": forwardedFor } : {}),
      // User agent peramban untuk daftar sesi ("Chrome di macOS"); tanpa ini Go mencatat "node" (fetch server)
      // untuk setiap sesi yang lahir dari SSO, berbeda dari sesi masuk kata sandi (lihat headerKlien).
      ...(userAgent ? { "User-Agent": userAgent } : {}),
    },
  });

  const body = (await res.json().catch(() => null)) as Amplop<T> | null;
  if (!res.ok) {
    throw new BffApiError(
      res.status,
      body?.message || `Backend error (${res.status})`,
      body?.code,
    );
  }
  if (
    !body ||
    (body.code !== "200" && body.code !== "201") ||
    body.data === undefined
  ) {
    throw new BffApiError(
      res.status,
      body?.message || "Jawaban API tidak sah",
      body?.code,
    );
  }
  return body.data;
}

export interface SsoAuthorize {
  authorize_url: string;
  binding: string;
}

export const ssoApi = {
  /** POST /auth/sso/google/authorize: URL otorisasi Google dan binding untuk cookie. */
  authorize: (forwardedFor?: string | null, userAgent?: string | null) =>
    panggil<SsoAuthorize>(
      "/auth/sso/google/authorize",
      { method: "POST" },
      15_000,
      forwardedFor,
      userAgent,
    ),

  /** POST /auth/sso/google/callback: sesi penuh atau tantangan MFA (bentuk sama dengan login). */
  callback: (
    body: { code: string; state: string; binding: string },
    forwardedFor?: string | null,
    userAgent?: string | null,
  ) =>
    panggil<LoginResponse>(
      "/auth/sso/google/callback",
      { method: "POST", body: JSON.stringify(body) },
      40_000,
      forwardedFor,
      userAgent,
    ),
};

/**
 * Apakah SSO Google dikonfigurasi di API. Dipanggil halaman masuk di server pada setiap render (bukan
 * flag build): galat atau waktu habis berarti tombol disembunyikan, halaman masuk tidak pernah gagal karenanya.
 */
export async function ssoGoogleAktif(): Promise<boolean> {
  try {
    const status = await panggil<{ google?: boolean }>(
      "/auth/sso/status",
      { method: "GET" },
      2_500,
    );
    return status.google === true;
  } catch {
    return false;
  }
}
