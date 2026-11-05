import { NextRequest, NextResponse } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import { clearTokenCookies, getRefreshToken } from "@/lib/bff/cookies";
import { tujuanPemulihan, tukarRefreshToken } from "@/lib/bff/refresh-session";
import { urlPublik } from "@/lib/bff/public-url";
import { applySecurityHeaders } from "@/lib/bff/security-headers";
import {
  csrfSudahAda,
  pasangSesiBaru,
  setResumeCookie,
} from "@/lib/bff/session-cookies";

/**
 * POST /api/auth/refresh — dipanggil klien (single-flight) saat access token habis. Publik di edge: ia
 * dilindungi cookie refresh SameSite=Strict yang hanya dikirim ke /api/auth. Jawaban 401 berarti sesi
 * berakhir (cookie dihapus); galat lain bersifat sementara dan cookie dibiarkan.
 */
export async function POST(request: NextRequest) {
  const refreshToken = await getRefreshToken();
  if (!refreshToken) {
    return applySecurityHeaders(
      NextResponse.json(
        { error: true, message: "Refresh token tidak ditemukan." },
        { status: 401 },
      ),
    );
  }

  const hasil = await tukarRefreshToken(refreshToken, authApi);
  if (!hasil.ok) {
    const resp = NextResponse.json(
      { error: true, message: hasil.message },
      { status: hasil.status },
    );
    if (hasil.berakhir) clearTokenCookies(resp);
    return applySecurityHeaders(resp);
  }

  const response = NextResponse.json({
    success: true,
    // False pada pemutaran ulang yang ditoleransi: cookie refresh dari tab pemenang tetap dipakai.
    diputar: Boolean(hasil.refreshToken),
  });
  await pasangSesiBaru(response, hasil, csrfSudahAda(request));
  return applySecurityHeaders(response);
}

/**
 * GET /api/auth/refresh?lanjut=/path — pemulihan sesi saat navigasi halaman. Edge mengalihkan halaman
 * terlindungi tanpa access token ke sini (refresh token tidak terlihat oleh edge karena path cookie-nya
 * /api/auth). Berhasil → cookie baru dan kembali ke `lanjut`; tanpa refresh token atau ditolak → /masuk.
 */
export async function GET(request: NextRequest) {
  const tujuan = tujuanPemulihan(request.nextUrl.searchParams.get("lanjut"));
  const keMasuk = () =>
    NextResponse.redirect(
      urlPublik(request, `/masuk?lanjut=${encodeURIComponent(tujuan)}`),
      303,
    );
  const selesai = (resp: NextResponse) => {
    resp.headers.set("Cache-Control", "no-store");
    return applySecurityHeaders(resp);
  };

  const refreshToken = await getRefreshToken();
  if (!refreshToken) return selesai(keMasuk());

  const hasil = await tukarRefreshToken(refreshToken, authApi);
  if (!hasil.ok) {
    const resp = keMasuk();
    if (hasil.berakhir) clearTokenCookies(resp);
    return selesai(resp);
  }

  const response = NextResponse.redirect(urlPublik(request, tujuan), 303);
  await pasangSesiBaru(response, hasil, csrfSudahAda(request));
  setResumeCookie(response);
  return selesai(response);
}
