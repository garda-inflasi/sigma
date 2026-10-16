import type { NextRequest } from "next/server";
import { userApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { jsonResponse } from "@/lib/bff/security-headers";

export async function GET(request: NextRequest) {
  const session = await requireSession(request);
  if (session.error) return session.error;

  try {
    const permissions = await userApi.listPermissions(session.accessToken);
    return jsonResponse(permissions);
  } catch (error) {
    return handleRouteError(error);
  }
}