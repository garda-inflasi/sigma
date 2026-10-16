import type { NextRequest } from "next/server";
import { parseColdStorageQuery } from "@/lib/api/cold-storage-query";
import { coldStorageApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { errorResponse, privateJsonResponse } from "@/lib/bff/security-headers";

/**
 * GET /api/cold-storage → Go GET /v1/cold-storage
 * Memvalidasi parameter lalu meneruskan hanya yang dikenal; token sesi
 * (cookie httpOnly) dipasang di sini sehingga tidak pernah terlihat browser.
 */
export async function GET(request: NextRequest) {
  const session = await requireSession(request);
  if (session.error) return session.error;

  const parsed = parseColdStorageQuery(request.nextUrl.searchParams);
  if (!parsed.ok) return errorResponse(parsed.message, 400);

  try {
    return privateJsonResponse(await coldStorageApi.list(session.accessToken, parsed.params));
  } catch (error) {
    return handleRouteError(error);
  }
}
