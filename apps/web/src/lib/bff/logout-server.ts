/**
 * Mencabut sesi di server saat pengguna keluar. Dipisah dari route handler agar urutannya teruji.
 */
import "server-only";

import type { RefreshTokenResponse } from "./api-client";

export type HasilCabut =
  /** Sesi diakhiri di server. */
  | "dicabut"
  /** Tidak ada yang perlu dicabut: tanpa cookie, atau sesinya sudah berakhir/dicabut. */
  | "sudah-berakhir"
  /** Server tidak terjangkau atau galat; cookie tetap dihapus, tetapi sesi mungkin masih hidup di server. */
  | "gagal";

export interface ApiKeluar {
  logout: (accessToken: string, refreshToken?: string) => Promise<unknown>;
  refresh: (data: { refresh_token: string }) => Promise<RefreshTokenResponse>;
}

const statusDari = (error: unknown): number | undefined =>
  error instanceof Error && "status" in error
    ? (error as { status: number }).status
    : undefined;

/**
 * Mengakhiri sesi pemilik cookie. Access token (15 menit) hampir selalu sudah kedaluwarsa bila pengguna
 * menekan Keluar setelah lama tidak aktif, padahal refresh token masih sah 7 hari; tanpa langkah kedua
 * sesi tetap hidup di server walau cookie browser dihapus. Karena itu: coba keluar dengan access token;
 * bila ditolak (401) atau tak ada, tukar refresh token lebih dulu untuk memperoleh access token yang sah
 * lalu keluar dengannya. Sesi yang memang sudah dicabut ditolak refresh (401) dan dianggap beres.
 */
export async function cabutSesi(
  {
    accessToken,
    refreshToken,
  }: { accessToken?: string; refreshToken?: string },
  api: ApiKeluar,
): Promise<HasilCabut> {
  if (accessToken) {
    try {
      await api.logout(accessToken, refreshToken);
      return "dicabut";
    } catch (error) {
      if (statusDari(error) !== 401) return "gagal";
      // 401: access token kedaluwarsa atau sesinya sudah dicabut; refresh akan menentukan yang mana.
    }
  }
  if (!refreshToken) return "sudah-berakhir";

  let baru: RefreshTokenResponse;
  try {
    baru = await api.refresh({ refresh_token: refreshToken });
  } catch (error) {
    return statusDari(error) === 401 ? "sudah-berakhir" : "gagal";
  }
  try {
    await api.logout(baru.access_token, baru.refresh_token ?? refreshToken);
    return "dicabut";
  } catch (error) {
    return statusDari(error) === 401 ? "sudah-berakhir" : "gagal";
  }
}
