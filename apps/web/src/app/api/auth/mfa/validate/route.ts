import { NextRequest } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import { clearMfaChallengeCookie, getMfaChallenge } from "@/lib/bff/cookies";
import { checkLoginRateLimit } from "@/lib/bff/rate-limit";
import { validateRedirectUrl } from "@/lib/bff/safe-redirect";
import { errorResponse, jsonResponse } from "@/lib/bff/security-headers";
import {
  adalahSesi,
  attachSession,
  tujuanSesi,
} from "@/lib/bff/session-response";

/**
 * Langkah kedua login: menukar token tantangan (cookie dari /api/auth/login) dan kode OTP atau
 * kode cadangan dengan sesi penuh. Setara dengan login, jadi dibatasi laju yang sama.
 */
export async function POST(request: NextRequest) {
  if (!checkLoginRateLimit(request)) {
    return errorResponse("Terlalu banyak percobaan. Coba lagi nanti.", 429);
  }

  try {
    const { code, redirect } = (await request.json()) as {
      code?: string;
      redirect?: string;
    };
    if (!code?.trim()) {
      return errorResponse("Kode wajib diisi.", 400);
    }

    const challenge = await getMfaChallenge();
    if (!challenge) {
      return errorResponse("Sesi verifikasi berakhir, masuk ulang.", 401);
    }

    const result = await authApi.mfaValidate({
      mfa_token: challenge,
      code: code.trim(),
    });
    if (!adalahSesi(result)) {
      return errorResponse("Kesalahan server internal.", 500);
    }

    const response = jsonResponse({
      user: result.user,
      mfa_setup_diwajibkan: result.mfa_setup_diwajibkan ?? false,
      tujuan: tujuanSesi(result, validateRedirectUrl(redirect ?? null) ?? "/"),
    });
    await attachSession(response, result);
    clearMfaChallengeCookie(response);
    return response;
  } catch (error) {
    if (error instanceof Error && "status" in error) {
      const apiError = error as { status: number; message: string };
      const response = errorResponse(apiError.message, apiError.status);
      // Token tantangan yang tidak berlaku lagi tidak berguna: paksa masuk ulang.
      if (apiError.status === 401 && /masuk ulang/i.test(apiError.message))
        clearMfaChallengeCookie(response);
      return response;
    }
    return errorResponse("Kesalahan server internal.", 500);
  }
}
