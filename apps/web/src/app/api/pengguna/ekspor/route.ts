import { NextResponse, type NextRequest } from "next/server";
import { userApi } from "@/lib/bff/api-client";
import { handleRouteError, requireSession } from "@/lib/bff/handler";
import { applySecurityHeaders } from "@/lib/bff/security-headers";
import { KUNCI_FILTER } from "@/lib/pengguna/tabel";

const PARAM_EKSPOR = ["search", ...KUNCI_FILTER, "ids"] as const;

/**
 * Ekspor CSV pengguna. API Go mencatatnya sebagai EKSPOR_DATA sebelum menyerahkan berkas; BFF meneruskan berkas
 * dan header unduhannya apa adanya. Hanya parameter filter daftar (search, peran, status, lapis, mfa, terkunci)
 * dan `ids` yang diteruskan; validasinya di API Go (400 untuk nilai asing, sama dengan GET /v1/users).
 */
export async function GET(request: NextRequest) {
  const session = await requireSession(request);
  if (session.error) return session.error;

  try {
    const params = request.nextUrl.searchParams;
    const keluar = new URLSearchParams();
    for (const nama of PARAM_EKSPOR) {
      const nilai = params.get(nama);
      if (nilai) keluar.set(nama, nilai);
    }
    const query = keluar.toString();

    const res = await userApi.ekspor(
      session.accessToken,
      query ? `?${query}` : "",
    );
    const response = new NextResponse(res.body, {
      status: 200,
      headers: {
        "Content-Type":
          res.headers.get("Content-Type") ?? "text/csv; charset=utf-8",
        "Content-Disposition":
          res.headers.get("Content-Disposition") ??
          'attachment; filename="pengguna.csv"',
        // Data pribadi: tidak boleh disimpan cache bersama maupun peramban.
        "Cache-Control": "private, no-store",
      },
    });
    for (const nama of ["X-Export-Rows", "X-Export-Truncated"]) {
      const nilai = res.headers.get(nama);
      if (nilai) response.headers.set(nama, nilai);
    }
    return applySecurityHeaders(response);
  } catch (error) {
    return handleRouteError(error);
  }
}
