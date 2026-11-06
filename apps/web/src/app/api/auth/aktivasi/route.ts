import { NextRequest } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import { responsBatasLaju, tanggapiGalatApi } from "@/lib/bff/galat-auth";
import { ukurBatasLogin } from "@/lib/bff/rate-limit";
import { errorResponse, jsonResponse } from "@/lib/bff/security-headers";

/**
 * Aktivasi akun dari tautan undangan: menetapkan kata sandi pertama. Publik (belum ada sesi); tidak menerbitkan
 * sesi, pengguna masuk lewat /masuk. Satu jendela laju per IP dengan login (5 per menit): token undangan 256-bit
 * tidak bisa ditebak, batas ini mencegah penyalahgunaan endpoint dan memeriksa kebijakan kata sandi berulang-ulang.
 */
export async function POST(request: NextRequest) {
  const batas = ukurBatasLogin(request);
  if (!batas.diizinkan) return responsBatasLaju(batas.tungguDetik, "reset");

  try {
    const body = await request.json();
    const { token, password } = body ?? {};

    if (
      typeof token !== "string" ||
      token === "" ||
      typeof password !== "string" ||
      password === ""
    ) {
      return errorResponse("Token dan kata sandi wajib diisi.", 400);
    }

    await authApi.aktivasi({ token, password });

    return jsonResponse({
      message:
        "Akun diaktifkan. Silakan masuk dengan kata sandi yang baru ditetapkan.",
    });
  } catch (error) {
    return tanggapiGalatApi(error, "reset");
  }
}
