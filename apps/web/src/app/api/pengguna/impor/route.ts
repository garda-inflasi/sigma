import type { NextRequest } from "next/server";
import { userApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { errorResponse, jsonResponse } from "@/lib/bff/security-headers";
import { MAKS_BYTE_IMPOR } from "@/lib/pengguna/impor";

/** Badan JSON membungkus CSV (kutip dan baris baru di-escape), jadi batasnya sedikit di atas batas teks. */
const MAKS_BADAN = MAKS_BYTE_IMPOR * 2 + 4096;

/**
 * Impor CSV pengguna. `dry_run` wajib eksplisit: true hanya memeriksa, false membuat akun undangan untuk baris
 * yang sah. Pengurai dan pemeriksa ada di API Go; BFF hanya meneruskan teksnya.
 */
export async function POST(request: NextRequest) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  try {
    const panjang = Number(request.headers.get("content-length"));
    if (Number.isFinite(panjang) && panjang > MAKS_BADAN) {
      return errorResponse("Berkas terlalu besar.", 413);
    }

    const body = await request.json();
    const { csv, dry_run } = body ?? {};
    if (typeof csv !== "string" || csv.trim() === "") {
      return errorResponse("Isi berkas CSV wajib ada.", 400);
    }
    if (typeof dry_run !== "boolean") {
      return errorResponse("dry_run harus true atau false.", 400);
    }
    if (csv.length > MAKS_BADAN) {
      return errorResponse("Berkas terlalu besar.", 413);
    }

    const result = await userApi.impor(session.accessToken, csv, dry_run);
    return jsonResponse(result);
  } catch (error) {
    return handleRouteError(error);
  }
}
