import type { NextRequest } from "next/server";
import { impersonationApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { privateJsonResponse } from "@/lib/bff/security-headers";

/** GET /api/impersonation/targets — pemilih sasaran bagi pemanggil (`boleh: false` bila tidak berhak). */
export async function GET(request: NextRequest) {
  const session = await requireSession(request);
  if (session.error) return session.error;

  try {
    return privateJsonResponse(
      await impersonationApi.targets(session.accessToken),
    );
  } catch (error) {
    return handleRouteError(error);
  }
}
