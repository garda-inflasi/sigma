import { NextRequest } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import { setAccessTokenCookie, setRefreshTokenCookie } from "@/lib/bff/cookies";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { errorResponse, jsonResponse } from "@/lib/bff/security-headers";

/**
 * Memverifikasi kode pertama dan mengaktifkan MFA. Sesi lama dicabut server, jadi sesi baru dipasang
 * di sini; kode cadangan dikembalikan sekali untuk ditampilkan kepada pengguna.
 */
export async function POST(request: NextRequest) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  try {
    const { code } = (await request.json()) as { code?: string };
    if (!code?.trim()) {
      return errorResponse("Kode wajib diisi.", 400);
    }

    const hasil = await authApi.mfaVerify(session.accessToken, code.trim());
    const response = jsonResponse({ kode_cadangan: hasil.kode_cadangan });
    setAccessTokenCookie(response, hasil.access_token);
    setRefreshTokenCookie(response, hasil.refresh_token);
    return response;
  } catch (error) {
    return handleRouteError(error);
  }
}
