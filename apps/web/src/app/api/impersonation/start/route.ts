import type { NextRequest } from "next/server";
import { impersonationApi } from "@/lib/bff/api-client";
import { jawabAlihAkun } from "@/lib/bff/alih-akun";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { bacaMulai } from "@/lib/bff/impersonasi-masukan";
import { errorResponse } from "@/lib/bff/security-headers";

/**
 * POST /api/impersonation/start — mulai (atau ganti) impersonasi ke akun atau peran sasaran. Berhak-tidaknya,
 * kebijakan, dan alasan wajib ditegakkan API Go. Sesi baru dipasang sebagai cookie; tidak ada token di badan.
 */
export async function POST(request: NextRequest) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  let badan: unknown;
  try {
    badan = await request.json();
  } catch {
    return errorResponse("Badan permintaan tidak valid.", 400);
  }
  const masukan = bacaMulai(badan);
  if (!masukan.ok) return errorResponse(masukan.pesan, 400);

  try {
    const hasil = await impersonationApi.start(
      session.accessToken,
      masukan.nilai.masukan,
    );
    return await jawabAlihAkun(hasil, masukan.nilai.dari);
  } catch (error) {
    return handleRouteError(error);
  }
}
