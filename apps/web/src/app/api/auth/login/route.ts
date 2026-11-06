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
import type { MasukanGalat } from "@/lib/auth/pesan-login";
import { validateRedirectUrl } from "@/lib/bff/safe-redirect";
import { applySecurityHeaders } from "@/lib/bff/security-headers";
import {
  adalahSesi,
  attachSession,
  tujuanSesi,
} from "@/lib/bff/session-response";

/** Jawaban sukses: halaman tujuan (sudah disaring) yang dibuka form lewat router App Router. */
export interface JawabanMasuk {
  tujuan: string;
  /** True bila langkah berikutnya kode OTP (tujuan = /masuk?mfa=1…); sesi belum terbit. */
  mfa_required?: boolean;
  user?: unknown;
  mfa_setup_diwajibkan?: boolean;
}

/**
 * POST /api/auth/login — hanya JSON (`{ email, password, redirect? }`), dipanggil form masuk secara asinkron.
 *
 * Tidak lagi menerima kiriman form native. Dulu kiriman form dijawab `NextResponse.redirect` (307: metode
 * POST dipertahankan), sehingga halaman /masuk yang tampil sesudah gagal adalah hasil POST dan me-reload-nya
 * memicu dialog "Confirm Form Resubmission". Sekarang semua kegagalan dijawab JSON (kode stabil, sisa detik,
 * Retry-After) dan navigasi dilakukan klien hanya sesudah 2xx.
 */
export async function POST(request: NextRequest) {
  const gagal = (masukan: MasukanGalat) =>
    responsGalat(susunJawabanGalat(masukan, "masuk"));

  if (
    !(request.headers.get("content-type") ?? "").includes("application/json")
  ) {
    return applySecurityHeaders(
      NextResponse.json(
        { error: true, message: "Kirim permintaan sebagai JSON." },
        { status: 415 },
      ),
    );
  }

  // Batas laju dihitung untuk setiap percobaan, termasuk yang ditolak validasi.
  const batas = ukurBatasLogin(request);
  if (!batas.diizinkan) return gagal(masukanBatasLaju(batas.tungguDetik));

  let body: { email?: unknown; password?: unknown; redirect?: unknown };
  try {
    body = await request.json();
  } catch {
    return gagal({
      status: 400,
      pesan: "Isi permintaan bukan JSON yang valid.",
    });
  }
  const email = typeof body.email === "string" ? body.email.trim() : "";
  const password = typeof body.password === "string" ? body.password : "";
  if (!email || !password) {
    return gagal({ status: 400, pesan: "Email dan password wajib diisi." });
  }
  const redirectTo =
    validateRedirectUrl(
      typeof body.redirect === "string" ? body.redirect : null,
    ) ?? "/";

  try {
    const result = await authApi.login({ email, password });

    // Akun ber-MFA: belum ada sesi. Token tantangan disimpan di cookie httpOnly dan pengguna diminta kode OTP.
    if (result.mfa_required && result.mfa_token) {
      const response = NextResponse.json<JawabanMasuk>({
        mfa_required: true,
        tujuan: `/masuk?mfa=1&lanjut=${encodeURIComponent(redirectTo)}`,
      });
      setMfaChallengeCookie(response, result.mfa_token);
      return applySecurityHeaders(response);
    }
    if (!adalahSesi(result)) {
      return gagal({ status: 500, pesan: "Kesalahan server internal." });
    }

    const response = NextResponse.json<JawabanMasuk>({
      tujuan: tujuanSesi(result, redirectTo),
      user: result.user,
      mfa_setup_diwajibkan: result.mfa_setup_diwajibkan ?? false,
    });
    await attachSession(response, result);
    return applySecurityHeaders(response);
  } catch (error) {
    return gagal(masukanDariGalatApi(error));
  }
}
