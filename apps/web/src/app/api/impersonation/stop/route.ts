import type { NextRequest } from "next/server";
import { impersonationApi } from "@/lib/bff/api-client";
import { jawabAlihAkun } from "@/lib/bff/alih-akun";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { errorResponse } from "@/lib/bff/security-headers";

/**
 * POST /api/impersonation/stop — kembali ke akun asli. Sah bagi sesi impersonasi apa pun (tanpa izin khusus);
 * akun asli ditentukan server dari catatan sesi, bukan dari badan atau header klien. Badan hanya boleh memuat
 * `dari` (halaman semula, untuk tujuan sesudahnya).
 */
export async function POST(request: NextRequest) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  let dari: string | undefined;
  try {
    const teks = await request.text();
    if (teks.trim()) {
      const badan: unknown = JSON.parse(teks);
      if (badan === null || typeof badan !== "object" || Array.isArray(badan)) {
        return errorResponse("Badan permintaan tidak valid.", 400);
      }
      const nilai = (badan as { dari?: unknown }).dari;
      if (nilai !== undefined && nilai !== null && typeof nilai !== "string") {
        return errorResponse("Halaman semula tidak valid.", 400);
      }
      dari = typeof nilai === "string" ? nilai : undefined;
    }
  } catch {
    return errorResponse("Badan permintaan tidak valid.", 400);
  }

  try {
    const hasil = await impersonationApi.stop(session.accessToken);
    return await jawabAlihAkun(hasil, dari);
  } catch (error) {
    return handleRouteError(error);
  }
}
