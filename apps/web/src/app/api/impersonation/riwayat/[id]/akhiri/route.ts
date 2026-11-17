import type { NextRequest } from "next/server";
import { adalahUuid } from "@/lib/api/impersonasi";
import { impersonationApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { errorResponse, privateJsonResponse } from "@/lib/bff/security-headers";

/** POST /api/impersonation/riwayat/:id/akhiri — mengakhiri paksa satu sesi impersonasi aktif (ADMIN_SISTEM). */
export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  const { id } = await params;
  if (!adalahUuid(id)) return errorResponse("ID sesi impersonasi tidak valid.", 400);

  try {
    return privateJsonResponse(
      await impersonationApi.akhiri(session.accessToken, id),
    );
  } catch (error) {
    return handleRouteError(error);
  }
}
