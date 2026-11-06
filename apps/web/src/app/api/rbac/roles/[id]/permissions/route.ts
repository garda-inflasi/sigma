import type { NextRequest } from "next/server";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { rbacApiServer } from "@/lib/bff/rbac-api";
import { errorResponse, jsonResponse } from "@/lib/bff/security-headers";

/** PUT /api/rbac/roles/:id/permissions — mengganti seluruh izin satu peran (kunci optimistis `versi`). */
export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  try {
    const { id } = await params;
    const body = (await request.json()) as {
      permissions?: unknown;
      versi?: unknown;
    };
    const { permissions, versi } = body;
    if (
      !Array.isArray(permissions) ||
      !permissions.every((p) => typeof p === "string")
    ) {
      return errorResponse("Daftar izin wajib berupa larik teks.", 400);
    }
    if (typeof versi !== "number" || !Number.isInteger(versi) || versi < 1) {
      return errorResponse("Versi peran wajib diisi.", 400);
    }
    const peran = await rbacApiServer.updateRolePermissions(
      session.accessToken,
      id,
      { permissions, versi },
    );
    return jsonResponse(peran);
  } catch (error) {
    return handleRouteError(error);
  }
}
