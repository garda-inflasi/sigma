/**
 * Logika sisi server untuk menukar refresh token (dipakai `POST` dan `GET /api/auth/refresh`, serta logout
 * saat access token sudah kedaluwarsa). Dipisah dari route handler agar dapat diuji tanpa Next.
 */
import "server-only";

import { safeInternalPath } from "@/lib/auth/safe-path";
import type { RefreshTokenResponse } from "./api-client";

export type HasilTukar =
  | { ok: true; accessToken: string; refreshToken?: string }
  | {
      ok: false;
      status: number;
      message: string;
      /**
       * True bila refresh token ditolak server (tidak sah, dicabut, kedaluwarsa, atau dipakai ulang):
       * sesi memang berakhir dan cookie-nya layak dihapus. False untuk galat sementara (Go mati,
       * basis data bermasalah): cookie dibiarkan agar pemulihan bisa dicoba lagi.
       */
      berakhir: boolean;
    };

interface ApiRefresh {
  refresh: (data: { refresh_token: string }) => Promise<RefreshTokenResponse>;
}

/** Menukar refresh token lewat API Go dan memetakan jawabannya; tidak pernah melempar. */
export async function tukarRefreshToken(
  refreshToken: string,
  api: ApiRefresh,
): Promise<HasilTukar> {
  try {
    const hasil = await api.refresh({ refresh_token: refreshToken });
    if (!hasil.access_token) {
      return {
        ok: false,
        status: 502,
        message: "Jawaban refresh tidak lengkap.",
        berakhir: false,
      };
    }
    return {
      ok: true,
      accessToken: hasil.access_token,
      // Kosong pada pemutaran ulang dalam jendela toleransi: cookie refresh yang ada tetap dipakai.
      refreshToken: hasil.refresh_token || undefined,
    };
  } catch (error) {
    if (error instanceof Error && "status" in error) {
      const { status, message } = error as { status: number; message: string };
      return { ok: false, status, message, berakhir: status === 401 };
    }
    return {
      ok: false,
      status: 500,
      message: "Kesalahan server internal.",
      berakhir: false,
    };
  }
}

/** Jalur pemulihan sesi; jangan pernah menjadi tujuan pemulihan itu sendiri (loop). */
const JALUR_PEMULIHAN = "/api/auth/refresh";

/**
 * Tujuan setelah sesi dipulihkan, dari parameter `lanjut` yang dikendalikan penyerang: hanya path internal,
 * dan bukan rute pemulihan itu sendiri.
 */
export function tujuanPemulihan(raw: string | null | undefined): string {
  const aman = safeInternalPath(raw, "/");
  return aman === JALUR_PEMULIHAN || aman.startsWith(`${JALUR_PEMULIHAN}?`)
    ? "/"
    : aman;
}
