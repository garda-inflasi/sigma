/**
 * CSRF protection utilities for the BFF layer.
 * Uses double-submit cookie pattern:
 * - Server sets a CSRF token cookie (`__Host-csrf`)
 * - Client reads the cookie and sends it as `X-CSRF-Token` header
 * - BFF validates that the header matches the cookie
 */

import { NextResponse } from "next/server";

const isDev = true; // Always use simple cookie names (Caddy terminates TLS)
const CSRF_COOKIE = isDev ? "garda_csrf" : "__Host-csrf";
const CSRF_HEADER = "x-csrf-token";

/**
 * Generates a cryptographically random CSRF token.
 * Uses Web Crypto API (available in Edge/Node 18+).
 */
export async function generateCSRFToken(): Promise<string> {
  const buf = new Uint8Array(32);
  crypto.getRandomValues(buf);
  return Array.from(buf)
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

/**
 * Sets the CSRF token as a non-HttpOnly cookie on the given response.
 * Called by the BFF login handler after successful authentication.
 */
export function setCSRFCookie(response: NextResponse, token: string): void {
  response.cookies.set(CSRF_COOKIE, token, {
    httpOnly: false, // Must be readable by JS for the double-submit pattern
    secure: !isDev,
    sameSite: "lax",
    path: "/",
    maxAge: 60 * 60 * 24, // 24 hours
  });
}

/**
 * Validates the CSRF token from the request header against the cookie.
 * Returns true if valid, false otherwise.
 */
export async function validateCSRF(
  request: Request
): Promise<boolean> {
  const { cookies } = await import("next/headers");
  const jar = await cookies();
  const cookieToken = jar.get(CSRF_COOKIE)?.value;
  const headerToken = request.headers.get(CSRF_HEADER);

  if (!cookieToken || !headerToken) {
    return false;
  }

  // Constant-time comparison to prevent timing attacks
  return constantTimeEqual(cookieToken, headerToken);
}

/**
 * Validates Origin/Referer header as a second layer of CSRF defense.
 * Only for state-changing requests (POST, PUT, PATCH, DELETE).
 */
export function validateOrigin(request: Request): boolean {
  const origin = request.headers.get("origin");
  const referer = request.headers.get("referer");

  // Origin is preferred; fall back to Referer if Origin is absent
  const source = origin || referer;
  if (!source) {
    return false;
  }

  try {
    const url = new URL(source);
    // Allow only the application's own origin
    const allowedOrigins = [
      process.env.NEXT_PUBLIC_APP_URL || "http://localhost:3000",
    ];
    return allowedOrigins.some(
      (allowed) => url.origin === allowed
    );
  } catch {
    return false;
  }
}

/**
 * Constant-time string comparison to prevent timing attacks.
 */
function constantTimeEqual(a: string, b: string): boolean {
  if (a.length !== b.length) {
    return false;
  }
  let result = 0;
  for (let i = 0; i < a.length; i++) {
    result |= a.charCodeAt(i) ^ b.charCodeAt(i);
  }
  return result === 0;
}
