import type { NextRequest } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { jsonResponse } from "@/lib/bff/security-headers";

/** POST /api/auth/sessions/revoke-others — keluar dari perangkat lain (sesi ini tetap hidup). */
export async function POST(request: NextRequest) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  try {
    const hasil = await authApi.revokeOtherSessions(session.accessToken);
    return jsonResponse(hasil);
  } catch (error) {
    return handleRouteError(error);
  }
}
