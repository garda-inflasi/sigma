import type { NextRequest } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { jsonResponse } from "@/lib/bff/security-headers";

/** DELETE /api/auth/sessions/:id — cabut satu sesi milik sendiri (bukan yang sedang dipakai). */
export async function DELETE(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  try {
    const { id } = await params;
    const hasil = await authApi.revokeSession(session.accessToken, id);
    return jsonResponse(hasil);
  } catch (error) {
    return handleRouteError(error);
  }
}
