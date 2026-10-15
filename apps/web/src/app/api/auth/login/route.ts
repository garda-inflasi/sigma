import { NextRequest } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import {
  setAccessTokenCookie,
  setRefreshTokenCookie,
} from "@/lib/bff/cookies";
import { setCSRFCookie, generateCSRFToken } from "@/lib/bff/csrf";
import { checkLoginRateLimit } from "@/lib/bff/rate-limit";
import { jsonResponse, errorResponse } from "@/lib/bff/security-headers";

export async function POST(request: NextRequest) {
  // Rate limiting
  if (!checkLoginRateLimit(request)) {
    return errorResponse("Terlalu banyak percobaan login. Coba lagi nanti.", 429);
  }

  try {
    const body = await request.json();
    const { email, password } = body;

    if (!email || !password) {
      return errorResponse("Email dan password wajib diisi.", 400);
    }

    // Call Go backend
    const result = await authApi.login({ email, password });

    // Set HttpOnly cookies
    await setAccessTokenCookie(result.access_token);
    await setRefreshTokenCookie(result.refresh_token);

    // Set CSRF token (readable by JS for double-submit pattern)
    const csrfToken = await generateCSRFToken();
    await setCSRFCookie(csrfToken);

    // Return only user data — no tokens in response body
    return jsonResponse({
      user: result.user,
    });
  } catch (error) {
    if (error instanceof Error && "status" in error) {
      const apiError = error as { status: number; message: string };
      return errorResponse(apiError.message, apiError.status);
    }
    return errorResponse("Kesalahan server internal.", 500);
  }
}
