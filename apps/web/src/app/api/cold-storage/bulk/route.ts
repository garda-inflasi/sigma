import type { NextRequest } from "next/server";
import { COLD_STORAGE_BULK_ACTIONS, COLD_STORAGE_MAX_BULK_IDS, type ColdStorageBulkAction } from "@/lib/api/cold-storage-query";
import { coldStorageApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { errorResponse, privateJsonResponse } from "@/lib/bff/security-headers";

/**
 * POST /api/cold-storage/bulk → Go POST /v1/cold-storage/bulk
 * Mengubah state (tercatat di audit), jadi wajib lolos CSRF + origin.
 * Izin per aksi ditegakkan oleh backend; di sini hanya bentuk payload.
 */
export async function POST(request: NextRequest) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  let body: { action?: unknown; ids?: unknown };
  try {
    body = await request.json();
  } catch {
    return errorResponse("Isi permintaan bukan JSON yang valid.", 400);
  }

  const { action, ids } = body;
  if (typeof action !== "string" || !(COLD_STORAGE_BULK_ACTIONS as readonly string[]).includes(action)) {
    return errorResponse("Aksi tidak dikenal.", 400);
  }
  if (!Array.isArray(ids) || ids.length === 0 || !ids.every((id) => typeof id === "string" && id.trim() !== "")) {
    return errorResponse("Pilih minimal satu lot stok.", 400);
  }
  if (ids.length > COLD_STORAGE_MAX_BULK_IDS) {
    return errorResponse(`Maksimal ${COLD_STORAGE_MAX_BULK_IDS} lot dalam satu aksi.`, 400);
  }

  try {
    return privateJsonResponse(await coldStorageApi.bulk(session.accessToken, { action: action as ColdStorageBulkAction, ids: ids as string[] }));
  } catch (error) {
    return handleRouteError(error);
  }
}
