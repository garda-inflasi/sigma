import type { NextRequest } from "next/server";
import { userApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { jsonResponse } from "@/lib/bff/security-headers";

/** Kirim ulang undangan akun yang masih menunggu aktivasi; tautan lama langsung hangus. */
export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  try {
    const { id } = await params;
    const result = await userApi.undangUlang(session.accessToken, id);
    return jsonResponse(result);
  } catch (error) {
    return handleRouteError(error);
  }
}
