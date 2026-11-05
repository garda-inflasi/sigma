import type { NextRequest } from "next/server";
import { coldStorageApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { errorResponse, privateJsonResponse } from "@/lib/bff/security-headers";

const JENIS = ["MASUK", "KELUAR", "PINDAH"];
/** Kolom yang diteruskan ke backend; selebihnya dibuang. */
const KOLOM = [
  "jenis",
  "lot_id",
  "fasilitas_id",
  "fasilitas_tujuan_id",
  "komoditas_kode",
  "pemilik",
  "alasan",
  "volume_ton",
  "semua",
] as const;

/**
 * POST /api/cold-storage/transaksi → Go POST /v1/cold-storage/transaksi
 * Mengubah stok (tercatat di audit), jadi wajib lolos CSRF + origin. Izin dan
 * aturan bisnis (stok, kapasitas) ditegakkan backend; di sini hanya bentuk payload.
 */
export async function POST(request: NextRequest) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  let body: Record<string, unknown>;
  try {
    const parsed: unknown = await request.json();
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed))
      throw new Error("bukan objek");
    body = parsed as Record<string, unknown>;
  } catch {
    return errorResponse("Isi permintaan bukan JSON yang valid.", 400);
  }

  if (typeof body.jenis !== "string" || !JENIS.includes(body.jenis)) {
    return errorResponse("Jenis transaksi tidak dikenal.", 400);
  }
  const forwarded = Object.fromEntries(
    KOLOM.filter((k) => body[k] !== undefined).map((k) => [k, body[k]]),
  );

  try {
    return privateJsonResponse(
      await coldStorageApi.transaksi(session.accessToken, forwarded),
      201,
    );
  } catch (error) {
    return handleRouteError(error);
  }
}
