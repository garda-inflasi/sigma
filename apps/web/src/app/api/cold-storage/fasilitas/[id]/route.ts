import type { NextRequest } from "next/server";
import { coldStorageApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { errorResponse, privateJsonResponse } from "@/lib/bff/security-headers";

/** UUID fasilitas atau kodenya (CS-01); sama dengan batas di backend (kolom kode varchar(64)). */
const MAX_ID = 64;

/**
 * GET /api/cold-storage/fasilitas/:id → Go GET /v1/cold-storage/fasilitas/:id
 * Detail satu fasilitas: riwayat suhu, lot tersimpan, dan transaksi terbaru.
 */
export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  const session = await requireSession(request);
  if (session.error) return session.error;

  const { id } = await params;
  if (!id || id.length > MAX_ID)
    return errorResponse("ID fasilitas tidak valid.", 400);

  try {
    return privateJsonResponse(
      await coldStorageApi.detail(session.accessToken, id),
    );
  } catch (error) {
    return handleRouteError(error);
  }
}

/** Kolom yang diteruskan ke backend; selebihnya dibuang. */
const KOLOM_UBAH = [
  "kapasitas_ton",
  "suhu_min_c",
  "suhu_maks_c",
  "status",
] as const;

/**
 * PATCH /api/cold-storage/fasilitas/:id → Go PATCH /v1/cold-storage/fasilitas/:id
 * Mengubah kapasitas, batas alarm suhu, atau status (tercatat di audit), jadi wajib lolos
 * CSRF + origin. Izin dan aturan bisnis (min < maks, kapasitas >= stok) ditegakkan backend.
 */
export async function PATCH(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  const { id } = await params;
  if (!id || id.length > MAX_ID)
    return errorResponse("ID fasilitas tidak valid.", 400);

  let body: Record<string, unknown>;
  try {
    const parsed: unknown = await request.json();
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed))
      throw new Error("bukan objek");
    body = parsed as Record<string, unknown>;
  } catch {
    return errorResponse("Isi permintaan bukan JSON yang valid.", 400);
  }

  const forwarded = Object.fromEntries(
    KOLOM_UBAH.filter((k) => body[k] !== undefined).map((k) => [k, body[k]]),
  );
  if (Object.keys(forwarded).length === 0)
    return errorResponse("Tidak ada perubahan yang dikirim.", 400);

  try {
    return privateJsonResponse(
      await coldStorageApi.ubahFasilitas(session.accessToken, id, forwarded),
    );
  } catch (error) {
    return handleRouteError(error);
  }
}
