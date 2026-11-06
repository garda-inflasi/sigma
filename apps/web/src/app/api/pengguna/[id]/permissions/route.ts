import type { NextRequest } from "next/server";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { POLA_NAMA_IZIN, rbacApiServer } from "@/lib/bff/rbac-api";
import { errorResponse, jsonResponse } from "@/lib/bff/security-headers";

/** GET /api/pengguna/:id/permissions — izin dari peran, izin langsung, dan yang berlaku. */
export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  const session = await requireSession(request);
  if (session.error) return session.error;

  try {
    const { id } = await params;
    return jsonResponse(
      await rbacApiServer.listUserPermissions(session.accessToken, id),
    );
  } catch (error) {
    return handleRouteError(error);
  }
}

/** POST /api/pengguna/:id/permissions — memberi izin langsung (alasan wajib, masa berlaku opsional). */
export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  try {
    const { id } = await params;
    const body = (await request.json()) as {
      izin?: unknown;
      alasan?: unknown;
      berlaku_sampai?: unknown;
    };
    if (typeof body.izin !== "string" || !POLA_NAMA_IZIN.test(body.izin)) {
      return errorResponse("Izin tidak valid.", 400);
    }
    if (typeof body.alasan !== "string" || !body.alasan.trim()) {
      return errorResponse("Alasan wajib diisi.", 400);
    }
    if (
      body.berlaku_sampai !== undefined &&
      typeof body.berlaku_sampai !== "string"
    ) {
      return errorResponse("Masa berlaku tidak valid.", 400);
    }
    const izin = await rbacApiServer.grantUserPermission(
      session.accessToken,
      id,
      {
        izin: body.izin,
        alasan: body.alasan,
        ...(body.berlaku_sampai ? { berlaku_sampai: body.berlaku_sampai } : {}),
      },
    );
    return jsonResponse(izin, 201);
  } catch (error) {
    return handleRouteError(error);
  }
}
