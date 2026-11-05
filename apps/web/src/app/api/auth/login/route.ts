import { NextRequest, NextResponse } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import { setMfaChallengeCookie } from "@/lib/bff/cookies";
import { checkLoginRateLimit } from "@/lib/bff/rate-limit";
import { validateRedirectUrl } from "@/lib/bff/safe-redirect";
import { applySecurityHeaders } from "@/lib/bff/security-headers";
import {
  adalahSesi,
  attachSession,
  tujuanSesi,
} from "@/lib/bff/session-response";

/** Alamat absolut di origin yang sedang dikunjungi (di belakang Caddy host/protokol datang dari header). */
function urlPublik(request: NextRequest, path: string): string {
  const host = request.headers.get("host") || "localhost";
  const protocol = request.headers.get("x-forwarded-proto") || "http";
  return `${protocol}://${host}${path}`;
}

export async function POST(request: NextRequest) {
  // Rate limiting
  if (!checkLoginRateLimit(request)) {
    const resp = NextResponse.json(
      {
        error: true,
        message: "Terlalu banyak percobaan login. Coba lagi nanti.",
      },
      { status: 429 },
    );
    return applySecurityHeaders(resp);
  }

  const contentType = request.headers.get("content-type") || "";
  const isFormPost = !contentType.includes("application/json");

  const gagal = (message: string, status: number) => {
    if (isFormPost) {
      return applySecurityHeaders(
        NextResponse.redirect(
          urlPublik(request, `/masuk?error=${encodeURIComponent(message)}`),
        ),
      );
    }
    return applySecurityHeaders(
      NextResponse.json({ error: true, message }, { status }),
    );
  };

  try {
    // Handle both JSON and form data
    let email: string;
    let password: string;
    let redirectParam: string | null =
      request.nextUrl.searchParams.get("redirect");

    if (!isFormPost) {
      const body = await request.json();
      email = body.email;
      password = body.password;
    } else {
      const formData = await request.formData();
      email = formData.get("email") as string;
      password = formData.get("password") as string;
      redirectParam =
        redirectParam ?? (formData.get("redirect") as string | null);
    }

    if (!email || !password) {
      return gagal("Email dan password wajib diisi.", 400);
    }

    const redirectTo = validateRedirectUrl(redirectParam) ?? "/";
    const result = await authApi.login({ email, password });

    // Akun ber-MFA: belum ada sesi. Token tantangan disimpan di cookie httpOnly dan pengguna diminta kode OTP.
    if (result.mfa_required && result.mfa_token) {
      const response = isFormPost
        ? NextResponse.redirect(
            urlPublik(
              request,
              `/masuk?mfa=1&lanjut=${encodeURIComponent(redirectTo)}`,
            ),
          )
        : NextResponse.json({ mfa_required: true });
      setMfaChallengeCookie(response, result.mfa_token);
      return applySecurityHeaders(response);
    }
    if (!adalahSesi(result)) {
      return gagal("Kesalahan server internal.", 500);
    }

    const destination = tujuanSesi(result, redirectTo);
    const response = isFormPost
      ? NextResponse.redirect(urlPublik(request, destination))
      : NextResponse.json({
          user: result.user,
          mfa_setup_diwajibkan: result.mfa_setup_diwajibkan ?? false,
        });
    await attachSession(response, result);

    return applySecurityHeaders(response);
  } catch (error) {
    if (error instanceof Error && "status" in error) {
      const apiError = error as { status: number; message: string };
      return gagal(apiError.message, apiError.status);
    }
    return gagal("Kesalahan server internal.", 500);
  }
}
