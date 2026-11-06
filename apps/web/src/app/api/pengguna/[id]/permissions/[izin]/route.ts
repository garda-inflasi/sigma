import type { NextRequest } from "next/server";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { POLA_NAMA_IZIN, rbacApiServer } from "@/lib/bff/rbac-api";
import { errorResponse, jsonResponse } from "@/lib/bff/security-headers";

/** DELETE /api/pengguna/:id/permissions/:izin — mencabut izin langsung. */
export async function DELETE(
  request: NextRequest,
  { params }: { params: Promise<{ id: string; izin: string }> },
) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  try {
    const { id, izin } = await params;
    if (!POLA_NAMA_IZIN.test(izin))
      return errorResponse("Izin tidak valid.", 400);
    return jsonResponse(
      await rbacApiServer.revokeUserPermission(session.accessToken, id, izin),
    );
  } catch (error) {
    return handleRouteError(error);
  }
}
