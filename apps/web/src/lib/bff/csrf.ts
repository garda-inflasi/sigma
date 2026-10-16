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
 *
 * The browser sets `Origin` to the origin the user actually visited, which is
 * the public origin terminated by the reverse proxy (e.g. `http://localhost`),
 * NOT the internal Next.js origin. We therefore accept:
 *   1. the request's own host, as forwarded by the proxy (`X-Forwarded-Host` /
 *      `Host`), which is inherently same-origin; and
 *   2. an explicit allowlist from `APP_ALLOWED_ORIGINS` (comma-separated) or
 *      `NEXT_PUBLIC_APP_URL`, for deployments where the public origin differs
 *      from the forwarded host.
 */
export function validateOrigin(request: Request): boolean {
  const origin = request.headers.get("origin");
  const referer = request.headers.get("referer");

  // Origin is preferred; fall back to Referer if Origin is absent
  const source = origin || referer;
  if (!source) {
    return false;
  }

  let sourceOrigin: string;
  try {
    sourceOrigin = new URL(source).origin;
  } catch {
    return false;
  }

  const allowed = new Set<string>();

  // Same-origin check against the host the browser actually addressed.
  const forwardedHost =
    request.headers.get("x-forwarded-host") ?? request.headers.get("host");
  if (forwardedHost) {
    const proto =
      request.headers.get("x-forwarded-proto") ??
      new URL(request.url).protocol.replace(":", "");
    // The proxy terminates TLS, so the browser may be on either scheme for the
    // same host; both are same-origin and therefore safe to accept.
    allowed.add(`${proto}://${forwardedHost}`);
    allowed.add(`http://${forwardedHost}`);
    allowed.add(`https://${forwardedHost}`);
  }

  // Explicit allowlist from configuration.
  const configured =
    process.env.APP_ALLOWED_ORIGINS ??
    process.env.NEXT_PUBLIC_APP_URL ??
    "http://localhost:3000";
  for (const entry of configured.split(",")) {
    const trimmed = entry.trim();
    if (!trimmed) continue;
    try {
      allowed.add(new URL(trimmed).origin);
    } catch {
      // Ignore malformed allowlist entries rather than failing open.
    }
  }

  return allowed.has(sourceOrigin);
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
