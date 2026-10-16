import { NextRequest, NextResponse } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import {
  setAccessTokenCookie,
  setRefreshTokenCookie,
  getRefreshToken,
} from "@/lib/bff/cookies";
import { applySecurityHeaders } from "@/lib/bff/security-headers";

export async function POST(request: NextRequest) {
  try {
    const refreshToken = await getRefreshToken();
    if (!refreshToken) {
      const resp = NextResponse.json(
        { error: true, message: "Refresh token tidak ditemukan." },
        { status: 401 }
      );
      return applySecurityHeaders(resp);
    }

    // Call Go backend
    const result = await authApi.refresh({
      refresh_token: refreshToken,
    });

    // Build response and set cookies on it
    const response = NextResponse.json({ success: true });
    setAccessTokenCookie(response, result.access_token);
    setRefreshTokenCookie(response, result.refresh_token);

    return applySecurityHeaders(response);
  } catch (error) {
    if (error instanceof Error && "status" in error) {
      const apiError = error as { status: number; message: string };
      const resp = NextResponse.json(
        { error: true, message: apiError.message },
        { status: apiError.status }
      );
      return applySecurityHeaders(resp);
    }
    const resp = NextResponse.json(
      { error: true, message: "Kesalahan server internal." },
      { status: 500 }
    );
    return applySecurityHeaders(resp);
  }
}
