import type { NextRequest } from "next/server";
import { userApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { jsonResponse, errorResponse } from "@/lib/bff/security-headers";

export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  try {
    const { id } = await params;
    const body = await request.json();
    const peran = body.peran ?? body.role;

    if (!peran || typeof peran !== "string") {
      return errorResponse("Peran wajib diisi.", 400);
    }

    const user = await userApi.assignRole(session.accessToken, id, peran);
    return jsonResponse(user);
  } catch (error) {
    return handleRouteError(error);
  }
}