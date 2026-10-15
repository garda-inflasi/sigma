import { NextRequest } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import {
  getAccessToken,
  clearTokenCookies,
} from "@/lib/bff/cookies";
import { jsonResponse, errorResponse } from "@/lib/bff/security-headers";

export async function POST(request: NextRequest) {
  try {
    const accessToken = await getAccessToken();
    if (accessToken) {
      // Best-effort: revoke on backend
      try {
        await authApi.logout(accessToken);
      } catch {
        // Ignore backend errors during logout
      }
    }

    // Clear cookies regardless of backend result
    await clearTokenCookies();

    return jsonResponse({ success: true });
  } catch {
    return errorResponse("Gagal logout.", 500);
  }
}
