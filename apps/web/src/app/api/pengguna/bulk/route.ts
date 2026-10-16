import type { NextRequest } from "next/server";
import { userApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { jsonResponse, errorResponse } from "@/lib/bff/security-headers";

const ALLOWED_ACTIONS = [
  "aktifkan",
  "nonaktifkan",
  "wajibkan_mfa",
  "akhiri_sesi",
] as const;

type AllowedAction = (typeof ALLOWED_ACTIONS)[number];

export async function POST(request: NextRequest) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  try {
    const body = await request.json();
    const action = body.action ?? body.aksi;
    const ids = body.ids;

    if (!ALLOWED_ACTIONS.includes(action)) {
      return errorResponse("Aksi tidak dikenal.", 400);
    }
    if (!Array.isArray(ids) || ids.length === 0) {
      return errorResponse("Tidak ada pengguna yang dipilih.", 400);
    }

    const result = await userApi.bulk(session.accessToken, {
      action: action as AllowedAction,
      ids,
    });
    return jsonResponse(result);
  } catch (error) {
    return handleRouteError(error);
  }
}