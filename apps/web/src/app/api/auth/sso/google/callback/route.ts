import { NextRequest, NextResponse } from "next/server";
import { setMfaChallengeCookie } from "@/lib/bff/cookies";
import { checkRateLimit, getRateLimitKey } from "@/lib/bff/rate-limit";
import { applySecurityHeaders } from "@/lib/bff/security-headers";
import {
  adalahSesi,
  attachSession,
  tujuanSesi,
} from "@/lib/bff/session-response";
import { ssoApi } from "@/lib/bff/sso-api";
import {
  bacaParameterCallback,
  clearSsoFlowCookie,
  galatSsoDariApi,
  galatSsoDariPenyedia,
  readSsoFlow,
  urlLangkahOtp,
  urlMasukGalat,
} from "@/lib/bff/sso";

const BATAS_CALLBACK_PER_MENIT = 10;

/** Pengalihan relatif: browser menyelesaikannya terhadap origin publik yang sedang dikunjungi. */
function alihkan(path: string): NextResponse {
  return new NextResponse(null, { status: 302, headers: { Location: path } });
}

/**
 * Penutup setiap jawaban callback: cookie alur dihapus apa pun hasilnya (binding tidak pernah dipakai dua
 * kali), tidak boleh di-cache, dan Referer tidak membawa query `code`/`state` ke halaman berikutnya.
 */
function selesai(response: NextResponse): NextResponse {
  clearSsoFlowCookie(response);
  response.headers.set("Cache-Control", "no-store");
  applySecurityHeaders(response);
  response.headers.set("Referrer-Policy", "no-referrer");
  return response;
}

/**
 * GET /api/auth/sso/google/callback?code=…&state=…
 *
 * Tujuan pengalihan Google. Menyerahkan code, state, dan binding (dari cookie alur) ke API Go yang
 * menukar kode, memverifikasi ID token, dan memetakan identitas ke akun yang sudah ada. Hasilnya diperlakukan
 * persis seperti login kata sandi: cookie sesi lalu ke beranda atau `lanjut`, atau langkah OTP untuk akun
 * ber-MFA. Galat berakhir di `/masuk?error=<kode>`.
 */
export async function GET(request: NextRequest) {
  const flow = readSsoFlow(request);
  const lanjut = flow?.lanjut ?? "/";

  if (
    !checkRateLimit(
      `${getRateLimitKey(request)}:sso-callback`,
      BATAS_CALLBACK_PER_MENIT,
    )
  ) {
    return selesai(alihkan(urlMasukGalat("sso_terlalu_banyak", lanjut)));
  }

  const params = request.nextUrl.searchParams;

  // Penyedia menolak atau pengguna membatalkan: tidak ada kode untuk ditukar.
  const galatPenyedia = params.get("error");
  if (galatPenyedia) {
    return selesai(
      alihkan(urlMasukGalat(galatSsoDariPenyedia(galatPenyedia), lanjut)),
    );
  }

  // Tanpa cookie alur callback tidak terikat ke browser ini (login CSRF atau cookie kedaluwarsa).
  const callback = bacaParameterCallback(params);
  if (!flow || !callback) {
    return selesai(alihkan(urlMasukGalat("sso_sesi_tidak_valid", lanjut)));
  }

  try {
    const result = await ssoApi.callback(
      { code: callback.code, state: callback.state, binding: flow.binding },
      request.headers.get("x-forwarded-for"),
    );

    // Akun ber-MFA: belum ada sesi. Token tantangan memakai cookie dan langkah OTP yang sama dengan login.
    if (result.mfa_required && result.mfa_token) {
      const response = alihkan(urlLangkahOtp(lanjut));
      setMfaChallengeCookie(response, result.mfa_token);
      return selesai(response);
    }
    if (!adalahSesi(result)) {
      return selesai(alihkan(urlMasukGalat("sso_gagal", lanjut)));
    }

    const response = alihkan(tujuanSesi(result, lanjut));
    await attachSession(response, result);
    return selesai(response);
  } catch (error) {
    return selesai(alihkan(urlMasukGalat(galatSsoDariApi(error), lanjut)));
  }
}
