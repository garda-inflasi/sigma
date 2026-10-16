import type { NextRequest } from "next/server";
import { userApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { jsonResponse, errorResponse } from "@/lib/bff/security-headers";

export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  try {
    const { id } = await params;
    const body = await request.json();
    const newPassword = body.new_password ?? body.password;

    if (!newPassword || typeof newPassword !== "string") {
      return errorResponse("Kata sandi baru wajib diisi.", 400);
    }

    const result = await userApi.resetPassword(
      session.accessToken,
      id,
      newPassword
    );
    return jsonResponse(result);
  } catch (error) {
    return handleRouteError(error);
  }
}