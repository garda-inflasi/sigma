import { NextRequest } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { errorResponse, jsonResponse } from "@/lib/bff/security-headers";

/** Menonaktifkan MFA setelah konfirmasi kata sandi; ditolak server bila MFA diwajibkan untuk akun. */
export async function POST(request: NextRequest) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  try {
    const { password } = (await request.json()) as { password?: string };
    if (!password) {
      return errorResponse("Kata sandi wajib diisi.", 400);
    }
    await authApi.mfaDisable(session.accessToken, password);
    return jsonResponse({ success: true });
  } catch (error) {
    return handleRouteError(error);
  }
}
