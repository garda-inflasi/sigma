/**
 * Cookie utilities for the BFF layer.
 * Sets HttpOnly, Secure, SameSite cookies for tokens.
 */

import { cookies } from "next/headers";

const ACCESS_COOKIE = "__Host-access";
const REFRESH_COOKIE = "__Secure-refresh";

interface CookieOptions {
  maxAge?: number;
  path?: string;
  domain?: string;
}

/**
 * Sets the access token as an HttpOnly cookie.
 * Uses __Host- prefix for cookie hardening (requires Secure, path=/, no domain).
 */
export async function setAccessTokenCookie(
  token: string,
  maxAge: number = 15 * 60 // 15 minutes default
): Promise<void> {
  const jar = await cookies();
  jar.set(ACCESS_COOKIE, token, {
    httpOnly: true,
    secure: true,
    sameSite: "lax",
    path: "/",
    maxAge,
  });
}

/**
 * Sets the refresh token as an HttpOnly cookie.
 * Uses __Secure- prefix (cannot use __Host- because path is restricted to /api/auth).
 */
export async function setRefreshTokenCookie(
  token: string,
  maxAge: number = 7 * 24 * 60 * 60 // 7 days default
): Promise<void> {
  const jar = await cookies();
  jar.set(REFRESH_COOKIE, token, {
    httpOnly: true,
    secure: true,
    sameSite: "strict",
    path: "/api/auth",
    maxAge,
  });
}

/**
 * Clears both token cookies (for logout).
 */
export async function clearTokenCookies(): Promise<void> {
  const jar = await cookies();
  jar.set(ACCESS_COOKIE, "", {
    httpOnly: true,
    secure: true,
    sameSite: "lax",
    path: "/",
    maxAge: 0,
  });
  jar.set(REFRESH_COOKIE, "", {
    httpOnly: true,
    secure: true,
    sameSite: "strict",
    path: "/api/auth",
    maxAge: 0,
  });
}

/**
 * Gets the access token from the cookie.
 */
export async function getAccessToken(): Promise<string | undefined> {
  const jar = await cookies();
  return jar.get(ACCESS_COOKIE)?.value;
}

/**
 * Gets the refresh token from the cookie.
 */
export async function getRefreshToken(): Promise<string | undefined> {
  const jar = await cookies();
  return jar.get(REFRESH_COOKIE)?.value;
}
