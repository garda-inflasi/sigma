import { NextRequest, NextResponse } from "next/server";
import { checkRateLimit, getRateLimitKey } from "@/lib/bff/rate-limit";
import { applySecurityHeaders } from "@/lib/bff/security-headers";
import { ssoApi } from "@/lib/bff/sso-api";
import {
  galatSsoDariApi,
  lanjutAman,
  setSsoFlowCookie,
  tujuanPenyedia,
  urlMasukGalat,
} from "@/lib/bff/sso";

/** Awal alur dibatasi per IP agar tidak dipakai membanjiri tabel state di API. */
const BATAS_AWAL_PER_MENIT = 10;

/** Pengalihan relatif: browser menyelesaikannya terhadap origin publik yang sedang dikunjungi. */
function alihkan(path: string): NextResponse {
  return new NextResponse(null, { status: 302, headers: { Location: path } });
}

function tanpaCache(response: NextResponse): NextResponse {
  response.headers.set("Cache-Control", "no-store");
  return applySecurityHeaders(response);
}

/**
 * GET /api/auth/sso/google?lanjut=<path>
 *
 * Meminta URL otorisasi dari API Go (state, nonce, dan PKCE dibuat dan disimpan di sana), menyimpan
 * `binding` di cookie httpOnly SameSite=Lax, lalu mengalihkan browser ke Google. Galat berakhir di
 * `/masuk?error=<kode>`.
 */
export async function GET(request: NextRequest) {
  const lanjut = lanjutAman(request.nextUrl.searchParams.get("lanjut"));

  if (
    !checkRateLimit(
      `${getRateLimitKey(request)}:sso-mulai`,
      BATAS_AWAL_PER_MENIT,
    )
  ) {
    return tanpaCache(alihkan(urlMasukGalat("sso_terlalu_banyak", lanjut)));
  }

  try {
    const { authorize_url, binding } = await ssoApi.authorize(
      request.headers.get("x-forwarded-for"),
      request.headers.get("user-agent"),
    );
    const tujuan = tujuanPenyedia(authorize_url);
    if (!tujuan || !binding) {
      return tanpaCache(alihkan(urlMasukGalat("sso_gagal", lanjut)));
    }
    const response = NextResponse.redirect(tujuan, 302);
    setSsoFlowCookie(response, binding, lanjut);
    return tanpaCache(response);
  } catch (error) {
    return tanpaCache(alihkan(urlMasukGalat(galatSsoDariApi(error), lanjut)));
  }
}
