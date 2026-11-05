import { NextRequest } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { jsonResponse } from "@/lib/bff/security-headers";

/** Memulai setup MFA: rahasia TOTP, URI, dan QR (SVG). Terbuka juga bagi akun yang wajib setup MFA. */
export async function POST(request: NextRequest) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  try {
    return jsonResponse(await authApi.mfaSetup(session.accessToken));
  } catch (error) {
    return handleRouteError(error);
  }
}
