import { NextRequest } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import { clearMfaChallengeCookie, getMfaChallenge } from "@/lib/bff/cookies";
import { ukurBatasLogin } from "@/lib/bff/rate-limit";
import { validateRedirectUrl } from "@/lib/bff/safe-redirect";
import { KODE_GALAT } from "@/lib/auth/pesan-login";
import {
  masukanDariGalatApi,
  responsBatasLaju,
  responsGalat,
  susunJawabanGalat,
} from "@/lib/bff/galat-auth";
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
  const batas = ukurBatasLogin(request);
  if (!batas.diizinkan) return responsBatasLaju(batas.tungguDetik, "mfa");

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
      return responsGalat(
        susunJawabanGalat({ status: 401, kode: KODE_GALAT.MFA_SESI_BERAKHIR }, "mfa"),
      );
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
    const jawaban = susunJawabanGalat(masukanDariGalatApi(error), "mfa");
    const response = responsGalat(jawaban);
    // Token tantangan yang tidak berlaku lagi tidak berguna: paksa masuk ulang.
    if (jawaban.galat.ulangMasuk) clearMfaChallengeCookie(response);
    return response;
  }
}
