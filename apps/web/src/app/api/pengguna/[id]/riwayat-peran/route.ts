import type { NextRequest } from "next/server";
import { userApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { jsonResponse } from "@/lib/bff/security-headers";

/** GET /api/pengguna/:id/riwayat-peran — riwayat perubahan peran dari audit trail (pengguna.kelola); izin ditegakkan API Go. */
export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  const session = await requireSession(request);
  if (session.error) return session.error;

  try {
    const { id } = await params;
    const hasil = await userApi.riwayatPeran(session.accessToken, id);
    return jsonResponse(hasil);
  } catch (error) {
    return handleRouteError(error);
  }
}
