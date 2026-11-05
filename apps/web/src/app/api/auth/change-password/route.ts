import { NextRequest } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import {
  getAccessToken,
  setAccessTokenCookie,
  setRefreshTokenCookie,
} from "@/lib/bff/cookies";
import { validateCSRF, validateOrigin } from "@/lib/bff/csrf";
import { jsonResponse, errorResponse } from "@/lib/bff/security-headers";

export async function POST(request: NextRequest) {
  // CSRF validation
  if (!(await validateCSRF(request)) || !validateOrigin(request)) {
    return errorResponse("CSRF token tidak valid.", 403);
  }

  try {
    const accessToken = await getAccessToken();
    if (!accessToken) {
      return errorResponse("Tidak terautentikasi.", 401);
    }

    const body = await request.json();
    const { old_password, new_password } = body;

    if (!old_password || !new_password) {
      return errorResponse("Password lama dan baru wajib diisi.", 400);
    }

    const session = await authApi.changePassword(accessToken, {
      old_password,
      new_password,
    });

    // Sesi lama dicabut server; pasang sesi baru agar pengguna tidak perlu masuk ulang.
    const response = jsonResponse({ success: true });
    setAccessTokenCookie(response, session.access_token);
    setRefreshTokenCookie(response, session.refresh_token);
    return response;
  } catch (error) {
    if (error instanceof Error && "status" in error) {
      const apiError = error as { status: number; message: string };
      return errorResponse(apiError.message, apiError.status);
    }
    return errorResponse("Kesalahan server internal.", 500);
  }
}
