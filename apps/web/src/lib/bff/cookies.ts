/**
 * Cookie utilities for the BFF layer.
 * Sets HttpOnly, SameSite cookies for tokens; the Secure flag follows
 * `cookieSecure()` (on in production, `COOKIE_SECURE` overrides).
 *
 * IMPORTANT: These functions accept a NextResponse and set cookies on it.
 * Do NOT use cookies() from next/headers in Route Handlers — cookies set
 * that way may not be included in the response.
 */

import { NextResponse } from "next/server";
import { MFA_CHALLENGE_COOKIE, MFA_CHALLENGE_MAX_AGE } from "@/lib/auth/mfa";
import { ACCESS_COOKIE, REFRESH_COOKIE, cookieSecure } from "./cookie-policy";

interface CookieOptions {
  maxAge?: number;
  path?: string;
  domain?: string;
}

/**
 * Sets the access token as an HttpOnly cookie on the given response.
 */
export function setAccessTokenCookie(
  response: NextResponse,
  token: string,
  maxAge: number = 15 * 60 // 15 minutes default
): void {
  response.cookies.set(ACCESS_COOKIE, token, {
    httpOnly: true,
    secure: cookieSecure(),
    sameSite: "lax",
    path: "/",
    maxAge,
  });
}

/**
 * Sets the refresh token as an HttpOnly cookie on the given response.
 */
export function setRefreshTokenCookie(
  response: NextResponse,
  token: string,
  maxAge: number = 7 * 24 * 60 * 60 // 7 days default
): void {
  response.cookies.set(REFRESH_COOKIE, token, {
    httpOnly: true,
    secure: cookieSecure(),
    sameSite: "strict",
    path: "/api/auth",
    maxAge,
  });
}

/**
 * Menyimpan token tantangan MFA (lima menit) sebagai cookie httpOnly yang hanya dikirim ke /api/auth,
 * sehingga tidak tampil di URL maupun terbaca skrip halaman.
 */
export function setMfaChallengeCookie(response: NextResponse, token: string): void {
  response.cookies.set(MFA_CHALLENGE_COOKIE, token, {
    httpOnly: true,
    secure: cookieSecure(),
    sameSite: "strict",
    path: "/api/auth",
    maxAge: MFA_CHALLENGE_MAX_AGE,
  });
}

export function clearMfaChallengeCookie(response: NextResponse): void {
  response.cookies.set(MFA_CHALLENGE_COOKIE, "", {
    httpOnly: true,
    secure: cookieSecure(),
    sameSite: "strict",
    path: "/api/auth",
    maxAge: 0,
  });
}

/**
 * Clears both token cookies on the given response (for logout).
 */
export function clearTokenCookies(response: NextResponse): void {
  response.cookies.set(ACCESS_COOKIE, "", {
    httpOnly: true,
    secure: cookieSecure(),
    sameSite: "lax",
    path: "/",
    maxAge: 0,
  });
  response.cookies.set(REFRESH_COOKIE, "", {
    httpOnly: true,
    secure: cookieSecure(),
    sameSite: "strict",
    path: "/api/auth",
    maxAge: 0,
  });
}

/**
 * Gets the access token from the cookie.
 * This still uses cookies() from next/headers for server-side reads.
 */
export async function getAccessToken(): Promise<string | undefined> {
  const { cookies } = await import("next/headers");
  const jar = await cookies();
  return jar.get(ACCESS_COOKIE)?.value;
}

/** Token tantangan MFA dari cookie, bila masih ada. */
export async function getMfaChallenge(): Promise<string | undefined> {
  const { cookies } = await import("next/headers");
  const jar = await cookies();
  return jar.get(MFA_CHALLENGE_COOKIE)?.value;
}

/**
 * Gets the refresh token from the cookie.
 */
export async function getRefreshToken(): Promise<string | undefined> {
  const { cookies } = await import("next/headers");
  const jar = await cookies();
  return jar.get(REFRESH_COOKIE)?.value;
}
