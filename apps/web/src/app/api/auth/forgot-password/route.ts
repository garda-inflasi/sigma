import { NextRequest } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import { responsBatasLaju } from "@/lib/bff/galat-auth";
import { getRateLimitKey, ukurBatasLaju } from "@/lib/bff/rate-limit";
import { jsonResponse, errorResponse } from "@/lib/bff/security-headers";

export async function POST(request: NextRequest) {
  // Rate limiting
  const batas = ukurBatasLaju(getRateLimitKey(request), 3, 60_000);
  if (!batas.diizinkan) return responsBatasLaju(batas.tungguDetik, "lupa");

  try {
    const body = await request.json();
    const { email } = body;

    if (!email) {
      return errorResponse("Email wajib diisi.", 400);
    }

    // Always return success to prevent email enumeration
    await authApi.forgotPassword({ email }).catch(() => {});

    return jsonResponse({
      message:
        "Jika email terdaftar, tautan reset password akan dikirim.",
    });
  } catch {
    return jsonResponse({
      message:
        "Jika email terdaftar, tautan reset password akan dikirim.",
    });
  }
}
