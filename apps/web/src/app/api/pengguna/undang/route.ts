import type { NextRequest } from "next/server";
import { userApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { errorResponse, jsonResponse } from "@/lib/bff/security-headers";
import type { Role } from "@/lib/rbac/roles";

const teks = (v: unknown): v is string =>
  typeof v === "string" && v.trim() !== "";

/**
 * Undang pengguna lewat email: API Go membuat akun MENUNGGU tanpa kata sandi dan mengirim tautan aktivasi.
 * Izin pengguna.kelola ditegakkan API Go.
 */
export async function POST(request: NextRequest) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  try {
    const body = await request.json();
    const { nama, email, peran, lembaga, kabupaten_kode } = body ?? {};

    if (!teks(nama) || !teks(email) || !teks(peran) || !teks(lembaga)) {
      return errorResponse("Nama, email, peran, dan lembaga wajib diisi.", 400);
    }
    if (kabupaten_kode !== undefined && typeof kabupaten_kode !== "string") {
      return errorResponse("Kode kabupaten tidak valid.", 400);
    }

    const result = await userApi.undang(session.accessToken, {
      nama,
      email,
      peran: peran as Role,
      lembaga,
      ...(kabupaten_kode ? { kabupaten_kode } : {}),
    });
    return jsonResponse(result, 201);
  } catch (error) {
    return handleRouteError(error);
  }
}
