import type { NextRequest } from "next/server";
import { impersonationApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { bacaParamRiwayat } from "@/lib/bff/impersonasi-masukan";
import { errorResponse, privateJsonResponse } from "@/lib/bff/security-headers";

/** GET /api/impersonation/riwayat?status=&limit=&cursor= — riwayat impersonasi (khusus ADMIN_SISTEM; API Go menegakkan). */
export async function GET(request: NextRequest) {
  const session = await requireSession(request);
  if (session.error) return session.error;

  const query = bacaParamRiwayat(request.nextUrl.searchParams);
  if (!query.ok) return errorResponse(query.pesan, 400);

  try {
    return privateJsonResponse(
      await impersonationApi.riwayat(session.accessToken, query.nilai),
    );
  } catch (error) {
    return handleRouteError(error);
  }
}
