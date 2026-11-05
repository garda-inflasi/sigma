import type { NextRequest } from "next/server";
import { coldStorageApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { errorResponse, privateJsonResponse } from "@/lib/bff/security-headers";

/** Kolom per pembacaan yang diteruskan ke backend; selebihnya dibuang. */
const KOLOM = [
  "fasilitas_id",
  "waktu",
  "suhu_c",
  "kelembapan_pct",
  "sumber",
] as const;
/** Sama dengan batas kiriman di backend (MaxTelemetriBatch). */
const MAX_PEMBACAAN = 500;

/**
 * POST /api/cold-storage/telemetri → Go POST /v1/cold-storage/telemetri
 * Mencatat pembacaan suhu (tercatat di audit), jadi wajib lolos CSRF + origin. Izin,
 * rentang nilai, dan deteksi duplikat/alarm ditegakkan backend; di sini hanya bentuk payload.
 */
export async function POST(request: NextRequest) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  let pembacaan: unknown;
  try {
    const parsed: unknown = await request.json();
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed))
      throw new Error("bukan objek");
    pembacaan = (parsed as Record<string, unknown>).pembacaan;
  } catch {
    return errorResponse("Isi permintaan bukan JSON yang valid.", 400);
  }

  if (
    !Array.isArray(pembacaan) ||
    pembacaan.length === 0 ||
    pembacaan.length > MAX_PEMBACAAN ||
    pembacaan.some((p) => !p || typeof p !== "object" || Array.isArray(p))
  ) {
    return errorResponse(
      `Kirim 1 sampai ${MAX_PEMBACAAN} pembacaan dalam larik "pembacaan".`,
      400,
    );
  }

  const forwarded = (pembacaan as Record<string, unknown>[]).map((p) =>
    Object.fromEntries(
      KOLOM.filter((k) => p[k] !== undefined).map((k) => [k, p[k]]),
    ),
  );

  try {
    return privateJsonResponse(
      await coldStorageApi.telemetri(session.accessToken, {
        pembacaan: forwarded,
      }),
    );
  } catch (error) {
    return handleRouteError(error);
  }
}
