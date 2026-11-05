import type { NextRequest } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { privateJsonResponse } from "@/lib/bff/security-headers";

/** GET /api/auth/sessions — sesi aktif milik pengguna yang sedang masuk. */
export async function GET(request: NextRequest) {
  const session = await requireSession(request);
  if (session.error) return session.error;

  try {
    const sesi = await authApi.sessions(session.accessToken);
    return privateJsonResponse(sesi);
  } catch (error) {
    return handleRouteError(error);
  }
}
