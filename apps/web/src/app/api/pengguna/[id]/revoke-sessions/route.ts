import type { NextRequest } from "next/server";
import { userApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { jsonResponse } from "@/lib/bff/security-headers";

/** POST /api/pengguna/:id/revoke-sessions — paksa keluar (pengguna.kelola); izin ditegakkan API Go. */
export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  try {
    const { id } = await params;
    const hasil = await userApi.forceLogout(session.accessToken, id);
    return jsonResponse(hasil);
  } catch (error) {
    return handleRouteError(error);
  }
}
