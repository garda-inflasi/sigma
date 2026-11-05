import type { NextRequest } from "next/server";
import { coldStorageApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { errorResponse, privateJsonResponse } from "@/lib/bff/security-headers";

const MAX_ID = 255;

/**
 * GET /api/cold-storage/fasilitas/:id → Go GET /v1/cold-storage/fasilitas/:id
 * Detail satu fasilitas: riwayat suhu, lot tersimpan, dan transaksi terbaru.
 */
export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  const session = await requireSession(request);
  if (session.error) return session.error;

  const { id } = await params;
  if (!id || id.length > MAX_ID)
    return errorResponse("ID fasilitas tidak valid.", 400);

  try {
    return privateJsonResponse(
      await coldStorageApi.detail(session.accessToken, id),
    );
  } catch (error) {
    return handleRouteError(error);
  }
}
