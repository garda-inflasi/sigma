import { NextRequest, NextResponse } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import {
  getAccessToken,
  clearTokenCookies,
} from "@/lib/bff/cookies";
import { applySecurityHeaders } from "@/lib/bff/security-headers";

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

    // Build response and clear cookies on it
    const response = NextResponse.json({ success: true });
    clearTokenCookies(response);

    return applySecurityHeaders(response);
  } catch {
    const resp = NextResponse.json(
      { error: true, message: "Gagal logout." },
      { status: 500 }
    );
    return applySecurityHeaders(resp);
  }
}
