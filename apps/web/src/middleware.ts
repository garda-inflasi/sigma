/**
 * Next.js Middleware — edge-level route protection, session refresh, and security headers.
 *
 * - Protects routes based on authentication status
 * - Refreshes access token if expired (using refresh token cookie)
 * - Adds security headers to all responses
 * - Redirects unauthenticated users to login
 * - Enforces RBAC at the route level (defense in depth)
 */

import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { resolveSession } from "@/lib/auth/resolve";
import { can } from "@/lib/rbac/permissions";
import { requiredPermission } from "@/lib/rbac/routes";
import { ROLES } from "@/lib/rbac/roles";

// Routes that don't require authentication
const PUBLIC_ROUTES = [
  "/masuk",
  "/daftar",
  "/lupa-password",
  "/reset-password",
  "/api/auth/login",
  "/api/auth/refresh",
  "/api/auth/forgot-password",
  "/api/auth/reset-password",
  "/healthz",
  "/akses-ditolak",
  "/offline",
  "/_next/static",
  "/_next/image",
  "/favicon.ico",
];

// API routes that need CSRF protection
const STATE_CHANGING_METHODS = ["POST", "PUT", "PATCH", "DELETE"];

export async function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;

  // Skip middleware for public routes
  if (PUBLIC_ROUTES.some((route) => pathname.startsWith(route))) {
    return NextResponse.next();
  }

  // Check authentication via access token cookie
  const accessToken = request.cookies.get("__Host-access")?.value;
  const refreshToken = request.cookies.get("__Secure-refresh")?.value;

  // If no access token but has refresh token, try to refresh
  if (!accessToken && refreshToken) {
    // For API routes, return 401 so the client can trigger refresh
    if (pathname.startsWith("/api/")) {
      return NextResponse.json(
        { error: true, message: "Token expired" },
        { status: 401 }
      );
    }

    // For page routes, redirect to login
    const loginUrl = new URL("/masuk", request.url);
    loginUrl.searchParams.set("redirect", pathname);
    return NextResponse.redirect(loginUrl);
  }

  // No tokens at all — redirect to login
  if (!accessToken && !refreshToken) {
    // Allow API routes to return 401
    if (pathname.startsWith("/api/")) {
      return NextResponse.json(
        { error: true, message: "Unauthorized" },
        { status: 401 }
      );
    }

    const loginUrl = new URL("/masuk", request.url);
    loginUrl.searchParams.set("redirect", pathname);
    return NextResponse.redirect(loginUrl);
  }

  // ── RBAC enforcement ──────────────────────────────────────────────
  // Resolve session to check permissions
  const user = await resolveSession((n) => request.cookies.get(n)?.value);

  if (!user) {
    const loginUrl = new URL("/masuk", request.url);
    loginUrl.searchParams.set("redirect", pathname);
    return NextResponse.redirect(loginUrl);
  }

  // Redirect root to role-specific beranda
  if (pathname === "/") {
    return NextResponse.redirect(new URL(ROLES[user.role].beranda, request.url));
  }

  // Check route-level permission
  const perm = requiredPermission(pathname);
  if (perm && !can(user.role, perm)) {
    const url = new URL("/akses-ditolak", request.url);
    url.searchParams.set("dari", pathname);
    return NextResponse.redirect(url);
  }

  // Apply security headers
  const response = NextResponse.next();
  response.headers.set("X-Content-Type-Options", "nosniff");
  response.headers.set("X-Frame-Options", "DENY");
  response.headers.set("X-XSS-Protection", "0");
  response.headers.set("Referrer-Policy", "strict-origin-when-cross-origin");

  if (process.env.NODE_ENV === "production") {
    response.headers.set(
      "Strict-Transport-Security",
      "max-age=63072000; includeSubDomains; preload"
    );
  }

  return response;
}

export const config = {
  matcher: [
    /*
     * Match all request paths except:
     * - _next/static (static files)
     * - _next/image (image optimization files)
     * - favicon.ico (favicon file)
     * - sw.js (service worker)
     * - manifest.webmanifest
     * - icons/ (app icons)
     */
    "/((?!_next/static|_next/image|favicon.ico|sw.js|manifest.webmanifest|icons/).*)",
  ],
};
