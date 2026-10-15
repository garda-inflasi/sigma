import { NextRequest } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import {
  setAccessTokenCookie,
  setRefreshTokenCookie,
  getRefreshToken,
} from "@/lib/bff/cookies";
import { jsonResponse, errorResponse } from "@/lib/bff/security-headers";

export async function POST(request: NextRequest) {
  try {
    const refreshToken = await getRefreshToken();
    if (!refreshToken) {
      return errorResponse("Refresh token tidak ditemukan.", 401);
    }

    // Call Go backend
    const result = await authApi.refresh({
      refresh_token: refreshToken,
    });

    // Set new HttpOnly cookies
    await setAccessTokenCookie(result.access_token);
    await setRefreshTokenCookie(result.refresh_token);

    return jsonResponse({ success: true });
  } catch (error) {
    if (error instanceof Error && "status" in error) {
      const apiError = error as { status: number; message: string };
      return errorResponse(apiError.message, apiError.status);
    }
    return errorResponse("Kesalahan server internal.", 500);
  }
}
