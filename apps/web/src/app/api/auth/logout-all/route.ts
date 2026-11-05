import type { NextRequest } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import { clearTokenCookies } from "@/lib/bff/cookies";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { jsonResponse } from "@/lib/bff/security-headers";

/** POST /api/auth/logout-all — keluar dari semua perangkat, termasuk yang sedang dipakai. */
export async function POST(request: NextRequest) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  try {
    const hasil = await authApi.logoutAll(session.accessToken);
    const response = jsonResponse(hasil);
    clearTokenCookies(response);
    return response;
  } catch (error) {
    return handleRouteError(error);
  }
}
