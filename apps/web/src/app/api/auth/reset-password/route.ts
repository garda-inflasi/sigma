import { NextRequest } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import { jsonResponse, errorResponse } from "@/lib/bff/security-headers";

export async function POST(request: NextRequest) {
  try {
    const body = await request.json();
    const { token, password } = body;

    if (!token || !password) {
      return errorResponse("Token dan password baru wajib diisi.", 400);
    }

    await authApi.resetPassword({ token, password });

    return jsonResponse({
      message: "Password berhasil direset. Silakan login dengan password baru.",
    });
  } catch (error) {
    if (error instanceof Error && "status" in error) {
      const apiError = error as { status: number; message: string };
      return errorResponse(apiError.message, apiError.status);
    }
    return errorResponse("Kesalahan server internal.", 500);
  }
}
