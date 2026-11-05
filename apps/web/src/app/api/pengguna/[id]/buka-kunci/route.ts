import type { NextRequest } from "next/server";
import { userApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { jsonResponse } from "@/lib/bff/security-headers";

/** Membuka kunci login (lockout) satu pengguna; izin pengguna.kelola ditegakkan API Go dan edge guard. */
export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  try {
    const { id } = await params;
    const result = await userApi.bukaKunci(session.accessToken, id);
    return jsonResponse(result);
  } catch (error) {
    return handleRouteError(error);
  }
}
