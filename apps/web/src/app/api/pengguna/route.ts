import type { NextRequest } from "next/server";
import { userApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { jsonResponse, errorResponse } from "@/lib/bff/security-headers";

export async function GET(request: NextRequest) {
  const session = await requireSession(request);
  if (session.error) return session.error;

  try {
    const params = request.nextUrl.searchParams;
    const page = Number(params.get("page")) || 1;
    const limit = Number(params.get("limit")) || 100;
    const search = params.get("search") ?? undefined;

    const result = await userApi.list(session.accessToken, {
      page,
      limit,
      search,
    });
    return jsonResponse(result);
  } catch (error) {
    return handleRouteError(error);
  }
}

export async function POST(request: NextRequest) {
  const session = await requireSession(request, { csrf: true });
  if (session.error) return session.error;

  try {
    const body = await request.json();
    const { id, nama, email, password, peran, lapis, lembaga } = body;

    if (!id || !nama || !email || !password || !peran || !lapis || !lembaga) {
      return errorResponse(
        "ID, nama, email, kata sandi, peran, lapis, dan lembaga wajib diisi.",
        400
      );
    }

    await userApi.create(session.accessToken, {
      id,
      nama,
      email,
      password,
      peran,
      lapis,
      lembaga,
    });
    return jsonResponse({ success: true }, 201);
  } catch (error) {
    return handleRouteError(error);
  }
}