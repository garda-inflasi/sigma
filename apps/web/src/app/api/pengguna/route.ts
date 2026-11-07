import type { NextRequest } from "next/server";
import { PARAM_DAFTAR_PENGGUNA, userApi, type ListUsersParams } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { jsonResponse, errorResponse } from "@/lib/bff/security-headers";

export async function GET(request: NextRequest) {
  const session = await requireSession(request);
  if (session.error) return session.error;

  try {
    const params = request.nextUrl.searchParams;
    const page = Number(params.get("page")) || 1;
    const limit = Number(params.get("limit")) || 100;

    // Filter dan urutan diteruskan apa adanya; API Go yang memvalidasi (400 untuk nilai tak dikenal).
    const saringan: ListUsersParams = {};
    for (const kunci of PARAM_DAFTAR_PENGGUNA) {
      const nilai = params.get(kunci);
      if (nilai) saringan[kunci] = nilai;
    }

    const result = await userApi.list(session.accessToken, {
      page,
      limit,
      ...saringan,
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

    if (!nama || !email || !password || !peran || !lapis || !lembaga) {
      return errorResponse(
        "Nama, email, kata sandi, peran, lapis, dan lembaga wajib diisi.",
        400
      );
    }

    // `id` opsional: kosong berarti API membuat UUID acak. Bila diisi, API yang memvalidasi (UUID sah, 400/409).
    await userApi.create(session.accessToken, {
      ...(id ? { id } : {}),
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