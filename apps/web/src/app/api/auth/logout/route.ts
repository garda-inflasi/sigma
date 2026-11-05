import { NextRequest, NextResponse } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import {
  getAccessToken,
  getRefreshToken,
  clearTokenCookies,
} from "@/lib/bff/cookies";
import { validateOrigin } from "@/lib/bff/csrf";
import { cabutSesi } from "@/lib/bff/logout-server";
import { applySecurityHeaders } from "@/lib/bff/security-headers";

/**
 * POST /api/auth/logout — mengakhiri hanya sesi ini (perangkat lain tetap masuk). Publik di edge supaya
 * tetap terjangkau saat access token sudah kedaluwarsa; karena itu dilindungi pemeriksaan Origin agar situs
 * lain tidak dapat memaksa pengguna keluar. Cookie selalu dihapus walau pencabutan di server gagal.
 */
export async function POST(request: NextRequest) {
  if (!validateOrigin(request)) {
    return applySecurityHeaders(
      NextResponse.json(
        { error: true, message: "Asal permintaan tidak valid." },
        { status: 403 },
      ),
    );
  }

  try {
    const hasil = await cabutSesi(
      {
        accessToken: await getAccessToken(),
        refreshToken: await getRefreshToken(),
      },
      authApi,
    );

    // Build response and clear cookies on it
    const response = NextResponse.json({ success: true, sesi: hasil });
    clearTokenCookies(response);

    return applySecurityHeaders(response);
  } catch {
    const resp = NextResponse.json(
      { error: true, message: "Gagal logout." },
      { status: 500 },
    );
    return applySecurityHeaders(resp);
  }
}
