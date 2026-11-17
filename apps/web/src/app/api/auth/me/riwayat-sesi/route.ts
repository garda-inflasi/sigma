import type { NextRequest } from "next/server";
import { PARAM_RIWAYAT_SESI, authApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { privateJsonResponse } from "@/lib/bff/security-headers";

/** GET /api/auth/me/riwayat-sesi — sesi masuk sendiri, aktif dan yang sudah berakhir. */
export async function GET(request: NextRequest) {
  const session = await requireSession(request);
  if (session.error) return session.error;

  try {
    const qs = new URLSearchParams();
    for (const kunci of PARAM_RIWAYAT_SESI) {
      const nilai = request.nextUrl.searchParams.get(kunci);
      if (nilai) qs.set(kunci, nilai);
    }
    return privateJsonResponse(await authApi.riwayatSesi(session.accessToken, qs.toString()));
  } catch (error) {
    return handleRouteError(error);
  }
}
