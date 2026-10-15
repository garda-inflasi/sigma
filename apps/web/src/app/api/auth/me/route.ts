import { NextRequest } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import { getAccessToken } from "@/lib/bff/cookies";
import { jsonResponse, errorResponse } from "@/lib/bff/security-headers";

export async function GET(request: NextRequest) {
  try {
    const accessToken = await getAccessToken();
    if (!accessToken) {
      return errorResponse("Tidak terautentikasi.", 401);
    }

    const user = await authApi.me(accessToken);
    return jsonResponse({ user });
  } catch (error) {
    if (error instanceof Error && "status" in error) {
      const apiError = error as { status: number; message: string };
      return errorResponse(apiError.message, apiError.status);
    }
    return errorResponse("Kesalahan server internal.", 500);
  }
}
