/**
 * Security headers for the BFF API routes.
 * Applied to all Route Handler responses.
 */

import { NextResponse } from "next/server";

/**
 * Applies security headers to a NextResponse.
 */
export function applySecurityHeaders(response: NextResponse): NextResponse {
  response.headers.set("X-Content-Type-Options", "nosniff");
  response.headers.set("X-Frame-Options", "DENY");
  response.headers.set("X-XSS-Protection", "0");
  response.headers.set("Referrer-Policy", "strict-origin-when-cross-origin");
  response.headers.set(
    "Permissions-Policy",
    "camera=(), microphone=(), geolocation=()"
  );

  // Strict-Transport-Security is deliberately omitted here. Next.js cannot
  // tell whether the request arrived over TLS (Caddy terminates it), so
  // emitting HSTS would also send it over plain HTTP — which makes the
  // browser rewrite http://localhost to https://localhost and breaks the
  // app. Caddy owns this header and emits it for HTTPS requests only.

  return response;
}

/**
 * Creates a JSON response with security headers.
 */
export function jsonResponse<T>(
  data: T,
  status: number = 200
): NextResponse {
  const response = NextResponse.json(data, { status });
  return applySecurityHeaders(response);
}

/**
 * Creates an error response with security headers.
 */
export function errorResponse(
  message: string,
  status: number = 400
): NextResponse {
  const response = NextResponse.json(
    { error: true, message },
    { status }
  );
  return applySecurityHeaders(response);
}
