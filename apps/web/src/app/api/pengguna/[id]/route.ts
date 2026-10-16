import type { NextRequest } from "next/server";
import { userApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { jsonResponse } from "@/lib/bff/security-headers";

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const session = await requireSession(request);
  if (session.error) return session.error;

  try {
    const { id } = await params;
    const user = await userApi.get(session.accessToken, id);
    return jsonResponse(user);
  } catch (error) {
    return handleRouteError(error);
  }
}

export async function PATCH(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  try {
    const { id } = await params;
    const body = await request.json();
    const user = await userApi.update(session.accessToken, id, body);
    return jsonResponse(user);
  } catch (error) {
    return handleRouteError(error);
  }
}

export async function DELETE(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  try {
    const { id } = await params;
    const result = await userApi.remove(session.accessToken, id);
    return jsonResponse(result);
  } catch (error) {
    return handleRouteError(error);
  }
}