import { NextRequest, NextResponse } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import { setMfaChallengeCookie } from "@/lib/bff/cookies";
import { ukurBatasLogin } from "@/lib/bff/rate-limit";
import {
  masukanBatasLaju,
  masukanDariGalatApi,
  responsGalat,
  susunJawabanGalat,
} from "@/lib/bff/galat-auth";
import { paramDariGalat, type MasukanGalat } from "@/lib/auth/pesan-login";
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
  const contentType = request.headers.get("content-type") || "";
  const isFormPost = !contentType.includes("application/json");

  // Semua kegagalan memakai tabel pesan dan kode tunggal (lib/auth/pesan-login): JSON untuk fetch, alihan
  // /masuk?error=…&kode=…&sampai=… untuk kiriman form (halaman masuk menampilkan hitung mundurnya).
  const gagal = (masukan: MasukanGalat) => {
    const jawaban = susunJawabanGalat(masukan, "masuk");
    if (isFormPost) {
      return applySecurityHeaders(
        NextResponse.redirect(
          urlPublik(request, `/masuk?${paramDariGalat(jawaban.galat)}`),
        ),
      );
    }
    return responsGalat(jawaban);
  };

  // Rate limiting
  const batas = ukurBatasLogin(request);
  if (!batas.diizinkan) return gagal(masukanBatasLaju(batas.tungguDetik));

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
      return gagal({ status: 400, pesan: "Email dan password wajib diisi." });
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
      return gagal({ status: 500, pesan: "Kesalahan server internal." });
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
    return gagal(masukanDariGalatApi(error));
  }
}
