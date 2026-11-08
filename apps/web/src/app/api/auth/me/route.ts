import type { NextRequest, NextResponse } from "next/server";
import { decodeJwtPayload } from "@/lib/auth/edge-claims";
import {
  authApi,
  type MeUserResponse,
  type UpdateProfilRequest,
} from "@/lib/bff/api-client";
import { getAccessToken, getRefreshToken } from "@/lib/bff/cookies";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { tukarRefreshToken } from "@/lib/bff/refresh-session";
import {
  errorResponse,
  jsonResponse,
  privateJsonResponse,
} from "@/lib/bff/security-headers";
import { csrfSudahAda, pasangSesiBaru } from "@/lib/bff/session-cookies";

export async function GET(request: NextRequest) {
  try {
    const accessToken = await getAccessToken();
    if (!accessToken) {
      return errorResponse("Tidak terautentikasi.", 401);
    }

    const user = await authApi.me(accessToken);
    return jsonResponse({ user });
  } catch (error) {
    if (error instanceof Error && "status" in error) {
      const apiError = error as { status: number; message: string };
      return errorResponse(apiError.message, apiError.status);
    }
    return errorResponse("Kesalahan server internal.", 500);
  }
}

const PESAN_BADAN_TIDAK_VALID = "Badan permintaan tidak valid.";

/**
 * Isian yang boleh diubah sendiri (`nama`, `telepon`); field lain dibuang di sini dan tidak pernah sampai ke
 * API. Nilai kosong (`null`) berarti tidak diubah, sama seperti field yang tidak dikirim. Mengembalikan
 * pesan galat bila badannya bukan objek atau isiannya bukan teks.
 */
function bacaPerubahan(badan: unknown): UpdateProfilRequest | string {
  if (badan === null || typeof badan !== "object" || Array.isArray(badan)) {
    return PESAN_BADAN_TIDAK_VALID;
  }
  const { nama, telepon } = badan as Record<string, unknown>;
  const perubahan: UpdateProfilRequest = {};
  if (nama !== undefined && nama !== null) {
    if (typeof nama !== "string") return "Nama harus berupa teks.";
    perubahan.nama = nama;
  }
  if (telepon !== undefined && telepon !== null) {
    if (typeof telepon !== "string") return "Nomor telepon harus berupa teks.";
    perubahan.telepon = telepon;
  }
  return perubahan;
}

/**
 * Nama di topbar berasal dari klaim `name` access token, yang baru berubah bila token diterbitkan ulang.
 * Bila klaim itu berbeda dari nama sesudah PATCH, refresh token ditukar dan cookie access dan refresh baru
 * dipasang pada respons (klien lalu memuat ulang sesi lewat `router.refresh()`). Profil sudah tersimpan,
 * jadi penukaran yang gagal tidak menggagalkan permintaan dan tidak menghapus sesi: klaim menyusul pada
 * pemulihan sesi berikutnya.
 */
async function selaraskanNamaSesi(
  request: NextRequest,
  response: NextResponse,
  accessToken: string,
  user: Pick<MeUserResponse, "nama">,
): Promise<void> {
  if (decodeJwtPayload(accessToken)?.name === user.nama) return;
  const refreshToken = await getRefreshToken();
  if (!refreshToken) return;
  const hasil = await tukarRefreshToken(refreshToken, authApi);
  if (hasil.ok) await pasangSesiBaru(response, hasil, csrfSudahAda(request));
}

/** PATCH /api/auth/me: mengubah nama dan/atau telepon milik sendiri (izin sesi yang sah cukup). */
export async function PATCH(request: NextRequest) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  let badan: unknown;
  try {
    badan = await request.json();
  } catch {
    return errorResponse(PESAN_BADAN_TIDAK_VALID, 400);
  }
  const perubahan = bacaPerubahan(badan);
  if (typeof perubahan === "string") return errorResponse(perubahan, 400);

  try {
    const user = await authApi.updateMe(session.accessToken, perubahan);
    const response = privateJsonResponse({ user });
    await selaraskanNamaSesi(request, response, session.accessToken, user);
    return response;
  } catch (error) {
    return handleRouteError(error);
  }
}
