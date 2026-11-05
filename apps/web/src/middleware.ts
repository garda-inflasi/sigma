/**
 * Next.js Middleware — penjagaan rute di edge dan header keamanan.
 *
 * Keputusan akses (login, kedaluwarsa, wajib ganti kata sandi, izin per rute)
 * ada di `decideEdge` (lib/rbac/edge-guard.ts, murni dan teruji). Middleware ini
 * hanya menerjemahkannya menjadi respons. Klaim JWT dibaca tanpa verifikasi tanda
 * tangan (edge tidak memegang rahasia): ini lapisan UX, API Go tetap otoritatif.
 *
 * Catatan: Next.js 16 menamai ulang konvensi ini menjadi `proxy.ts`; berkas ini
 * masih berfungsi (deprecated). Ganti nama bersamaan dengan upgrade runtime Node.
 */

import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { ACCESS_COOKIE, REFRESH_COOKIE } from "@/lib/bff/cookie-policy";
import { decideEdge, type EdgeDecision } from "@/lib/rbac/edge-guard";

function respond(request: NextRequest, decision: EdgeDecision): NextResponse {
  let response: NextResponse;
  switch (decision.kind) {
    case "redirect": {
      const url = new URL(decision.path, request.url);
      for (const [key, value] of Object.entries(decision.query ?? {})) {
        url.searchParams.set(key, value);
      }
      response = NextResponse.redirect(url);
      break;
    }
    case "json":
      response = NextResponse.json(decision.body, { status: decision.status });
      break;
    default:
      response = NextResponse.next();
  }
  if ("clearAccessCookie" in decision && decision.clearAccessCookie) {
    response.cookies.delete(ACCESS_COOKIE);
  }
  return response;
}

function withSecurityHeaders(response: NextResponse): NextResponse {
  response.headers.set("X-Content-Type-Options", "nosniff");
  response.headers.set("X-Frame-Options", "DENY");
  response.headers.set("X-XSS-Protection", "0");
  response.headers.set("Referrer-Policy", "strict-origin-when-cross-origin");
  // Strict-Transport-Security sengaja tidak dikirim di sini: edge tidak tahu
  // apakah request datang lewat TLS (Caddy yang mengakhiri TLS) dan HSTS lewat
  // HTTP memaksa browser menaikkan http://localhost ke https://localhost.
  // Caddy mengirim HSTS hanya untuk request HTTPS.
  return response;
}

export function middleware(request: NextRequest) {
  const decision = decideEdge({
    pathname: request.nextUrl.pathname,
    accessToken: request.cookies.get(ACCESS_COOKIE)?.value,
    refreshToken: request.cookies.get(REFRESH_COOKIE)?.value,
    nowSeconds: Math.floor(Date.now() / 1000),
  });
  return withSecurityHeaders(respond(request, decision));
}

export const config = {
  matcher: [
    /*
     * Cocokkan semua path kecuali:
     * - _next/static, _next/image, favicon.ico
     * - sw.js (service worker), manifest.webmanifest, icons/
     */
    "/((?!_next/static|_next/image|favicon.ico|sw.js|manifest.webmanifest|icons/).*)",
  ],
};
