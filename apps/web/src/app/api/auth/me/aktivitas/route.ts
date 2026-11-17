import type { NextRequest } from "next/server";
import { PARAM_AKTIVITAS_SAYA, authApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { privateJsonResponse } from "@/lib/bff/security-headers";

/**
 * GET /api/auth/me/aktivitas — riwayat aktivitas akun sendiri (jejak audit). Hanya parameter filter yang dikenal
 * diteruskan; akun selalu dari token di API Go, jadi tidak ada parameter yang bisa memilih akun lain.
 */
export async function GET(request: NextRequest) {
  const session = await requireSession(request);
  if (session.error) return session.error;

  try {
    const qs = new URLSearchParams();
    for (const kunci of PARAM_AKTIVITAS_SAYA) {
      const nilai = request.nextUrl.searchParams.get(kunci);
      if (nilai) qs.set(kunci, nilai);
    }
    return privateJsonResponse(await authApi.aktivitasSaya(session.accessToken, qs.toString()));
  } catch (error) {
    return handleRouteError(error);
  }
}
