import type { NextRequest } from "next/server";
import { impersonationApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { bacaKebijakan } from "@/lib/bff/impersonasi-masukan";
import { errorResponse, privateJsonResponse } from "@/lib/bff/security-headers";

/** GET /api/impersonation/policy — kebijakan impersonasi. Khusus ADMIN_SISTEM; selain itu API Go menjawab 403. */
export async function GET(request: NextRequest) {
  const session = await requireSession(request);
  if (session.error) return session.error;

  try {
    return privateJsonResponse(
      await impersonationApi.policy(session.accessToken),
    );
  } catch (error) {
    return handleRouteError(error);
  }
}

/** PUT /api/impersonation/policy — mengganti seluruh kebijakan (kunci optimistis `versi`; 409 bila basi). */
export async function PUT(request: NextRequest) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  let badan: unknown;
  try {
    badan = await request.json();
  } catch {
    return errorResponse("Badan permintaan tidak valid.", 400);
  }
  const masukan = bacaKebijakan(badan);
  if (!masukan.ok) return errorResponse(masukan.pesan, 400);

  try {
    return privateJsonResponse(
      await impersonationApi.savePolicy(session.accessToken, masukan.nilai),
    );
  } catch (error) {
    return handleRouteError(error);
  }
}
