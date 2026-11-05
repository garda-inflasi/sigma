import { NextRequest } from "next/server";
import { authApi } from "@/lib/bff/api-client";
import { tanggapiGalatApi } from "@/lib/bff/galat-auth";
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
    return tanggapiGalatApi(error, "reset");
  }
}
