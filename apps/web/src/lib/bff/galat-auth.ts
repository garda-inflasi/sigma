/**
 * Jawaban galat route BFF autentikasi (login, validasi OTP, lupa/atur ulang/ganti kata sandi). Semuanya
 * lewat tabel pesan tunggal di lib/auth/pesan-login: kode mesin dari API Go diteruskan tanpa diubah
 * (`kode`), sisa waktu tunggu dibawa di `sisa_detik` dan header `Retry-After`, dan teksnya Indonesia.
 */

import type { NextResponse } from "next/server";
import {
  galatAuth,
  kodeStabil,
  KODE_GALAT,
  pesanTampil,
  sisaDetik,
  type GalatAuth,
  type KonteksAuth,
  type MasukanGalat,
} from "@/lib/auth/pesan-login";
import { errorResponse } from "./security-headers";

export interface JawabanGalat {
  status: number;
  /** Isi JSON jawaban. */
  body: { error: true; message: string; kode?: string; sisa_detik?: number };
  /** Nilai header Retry-After (detik), hanya untuk galat yang menunggu. */
  retryAfter?: number;
  galat: GalatAuth;
}

/** Menyusun jawaban galat dari apa yang diketahui; murni (tanpa NextResponse) supaya mudah diuji. */
export function susunJawabanGalat(
  masukan: MasukanGalat,
  konteks: KonteksAuth,
  sekarangMs: number = Date.now(),
): JawabanGalat {
  const galat = galatAuth(masukan, konteks, sekarangMs);
  const status = masukan.status ?? 500;
  const tunggu =
    galat.sampaiMs !== undefined ? sisaDetik(galat.sampaiMs, sekarangMs) : 0;
  return {
    status,
    galat,
    retryAfter: tunggu > 0 ? tunggu : undefined,
    body: {
      error: true,
      message: pesanTampil(galat, sekarangMs),
      ...(kodeStabil(galat.kode) ? { kode: kodeStabil(galat.kode) } : {}),
      ...(tunggu > 0 ? { sisa_detik: tunggu } : {}),
    },
  };
}

/** Jawaban JSON untuk `jawaban`, lengkap dengan Retry-After dan header keamanan. */
export function responsGalat(jawaban: JawabanGalat): NextResponse {
  return errorResponse(jawaban.body.message, jawaban.status, {
    kode: jawaban.body.kode,
    sisaDetik: jawaban.retryAfter,
  });
}

/** Masukan galat dari exception route: BffApiError (status, message, code, detik) atau galat tak terduga. */
export function masukanDariGalatApi(error: unknown): MasukanGalat {
  if (error instanceof Error && "status" in error) {
    const e = error as {
      status: number;
      message: string;
      code?: string;
      detik?: number;
    };
    return { status: e.status, pesan: e.message, kode: e.code, detik: e.detik };
  }
  return { status: 500, pesan: "Kesalahan server internal." };
}

/** Jawaban untuk exception yang ditangkap route: meneruskan kode dan sisa waktu dari API Go. */
export function tanggapiGalatApi(
  error: unknown,
  konteks: KonteksAuth,
): NextResponse {
  return responsGalat(susunJawabanGalat(masukanDariGalatApi(error), konteks));
}

/** Masukan untuk batas laju BFF yang terlampaui: 429 RATE_LIMITED dengan lama tunggu. */
export function masukanBatasLaju(tungguDetik: number): MasukanGalat {
  return { status: 429, kode: KODE_GALAT.TERLALU_BANYAK, detik: tungguDetik };
}

/** Jawaban 429 untuk batas laju BFF. */
export function responsBatasLaju(
  tungguDetik: number,
  konteks: KonteksAuth,
): NextResponse {
  return responsGalat(
    susunJawabanGalat(masukanBatasLaju(tungguDetik), konteks),
  );
}
