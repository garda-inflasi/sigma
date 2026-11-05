import { NextRequest } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { errorResponse, jsonResponse } from "@/lib/bff/security-headers";

/** Menerbitkan kode cadangan baru (yang lama hangus) setelah konfirmasi kata sandi. */
export async function POST(request: NextRequest) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  try {
    const { password } = (await request.json()) as { password?: string };
    if (!password) {
      return errorResponse("Kata sandi wajib diisi.", 400);
    }
    return jsonResponse(
      await authApi.mfaBackupCodes(session.accessToken, password),
    );
  } catch (error) {
    return handleRouteError(error);
  }
}
