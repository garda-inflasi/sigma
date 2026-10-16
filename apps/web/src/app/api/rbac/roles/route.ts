import type { NextRequest } from "next/server";
import { userApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { jsonResponse } from "@/lib/bff/security-headers";

export async function GET(request: NextRequest) {
  const session = await requireSession(request);
  if (session.error) return session.error;

  try {
    const roles = await userApi.listRoles(session.accessToken);
    return jsonResponse(roles);
  } catch (error) {
    return handleRouteError(error);
  }
}