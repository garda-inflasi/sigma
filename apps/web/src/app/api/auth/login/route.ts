import { NextRequest, NextResponse } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import {
  setAccessTokenCookie,
  setRefreshTokenCookie,
} from "@/lib/bff/cookies";
import { setCSRFCookie, generateCSRFToken } from "@/lib/bff/csrf";
import { checkLoginRateLimit } from "@/lib/bff/rate-limit";
import { applySecurityHeaders } from "@/lib/bff/security-headers";

export async function POST(request: NextRequest) {
  // Rate limiting
  if (!checkLoginRateLimit(request)) {
    const resp = NextResponse.json(
      { error: true, message: "Terlalu banyak percobaan login. Coba lagi nanti." },
      { status: 429 }
    );
    return applySecurityHeaders(resp);
  }

  try {
    // Handle both JSON and form data
    let email: string;
    let password: string;

    const contentType = request.headers.get("content-type") || "";
    if (contentType.includes("application/json")) {
      const body = await request.json();
      email = body.email;
      password = body.password;
    } else {
      const formData = await request.formData();
      email = formData.get("email") as string;
      password = formData.get("password") as string;
    }

    if (!email || !password) {
      const resp = NextResponse.json(
        { error: true, message: "Email dan password wajib diisi." },
        { status: 400 }
      );
      return applySecurityHeaders(resp);
    }

    // Call Go backend
    const result = await authApi.login({ email, password });

    // Get redirect from form data or query params
    const redirectTo = request.nextUrl.searchParams.get("redirect") || "/";

    // Build response — if form POST, redirect instead of returning JSON
    const isFormPost = !contentType.includes("application/json");
    // Use the Host header to construct the correct redirect URL
    const host = request.headers.get("host") || "localhost";
    const protocol = request.headers.get("x-forwarded-proto") || "http";
    const redirectUrl = `${protocol}://${host}${redirectTo}`;
    const response = isFormPost
      ? NextResponse.redirect(redirectUrl)
      : NextResponse.json({ user: result.user });

    // Set HttpOnly cookies on the response object
    setAccessTokenCookie(response, result.access_token);
    setRefreshTokenCookie(response, result.refresh_token);

    // Set CSRF token (readable by JS for double-submit pattern)
    const csrfToken = await generateCSRFToken();
    setCSRFCookie(response, csrfToken);

    return applySecurityHeaders(response);
  } catch (error) {
    if (error instanceof Error && "status" in error) {
      const apiError = error as { status: number; message: string };
      const contentType = request.headers.get("content-type") || "";
      const isFormPost = !contentType.includes("application/json");
      
      if (isFormPost) {
        // For form POST, redirect back to login with error
        const host = request.headers.get("host") || "localhost";
        const protocol = request.headers.get("x-forwarded-proto") || "http";
        const loginUrl = `${protocol}://${host}/masuk?error=${encodeURIComponent(apiError.message)}`;
        const resp = NextResponse.redirect(loginUrl);
        return applySecurityHeaders(resp);
      }
      
      const resp = NextResponse.json(
        { error: true, message: apiError.message },
        { status: apiError.status }
      );
      return applySecurityHeaders(resp);
    }
    
    const contentType = request.headers.get("content-type") || "";
    const isFormPost = !contentType.includes("application/json");
    
    if (isFormPost) {
      const host = request.headers.get("host") || "localhost";
      const protocol = request.headers.get("x-forwarded-proto") || "http";
      const loginUrl = `${protocol}://${host}/masuk?error=${encodeURIComponent("Kesalahan server internal.")}`;
      const resp = NextResponse.redirect(loginUrl);
      return applySecurityHeaders(resp);
    }
    
    const resp = NextResponse.json(
      { error: true, message: "Kesalahan server internal." },
      { status: 500 }
    );
    return applySecurityHeaders(resp);
  }
}
