/**
 * Jawaban BFF untuk berpindah akun (impersonasi start/stop): token dari API Go dipasang sebagai cookie sesi
 * (access, refresh, CSRF baru) dan TIDAK PERNAH dikirim ke badan respons, sehingga skrip halaman tidak melihatnya.
 * Browser hanya menerima profil akun efektif, metadata impersonasi, dan halaman tujuan yang aman.
 */

import type { NextResponse } from "next/server";
import { readEdgeClaims } from "@/lib/auth/edge-claims";
import type { HasilAlihAkun } from "@/lib/api/impersonasi";
import type { SesiImpersonasiResponse } from "./api-client";
import { tentukanTujuan } from "./impersonasi-masukan";
import { errorResponse, privateJsonResponse } from "./security-headers";
import { attachSession } from "./session-response";

export async function jawabAlihAkun(
  hasil: SesiImpersonasiResponse,
  dari: string | undefined,
): Promise<NextResponse> {
  if (!hasil?.access_token || !hasil.refresh_token || !hasil.user) {
    return errorResponse("Jawaban layanan sesi tidak lengkap.", 502);
  }
  // Tujuan memakai peran dan izin dari token BARU: halaman semula hanya dipertahankan bila peran barunya boleh.
  const klaim = readEdgeClaims(hasil.access_token);
  const redirectTo = klaim ? tentukanTujuan(dari, klaim.role, klaim.permissions) : "/";

  const badan: HasilAlihAkun = {
    user: hasil.user,
    ...(hasil.impersonasi ? { impersonasi: hasil.impersonasi } : {}),
    redirectTo,
  };
  const response = privateJsonResponse(badan);
  await attachSession(response, hasil);
  return response;
}
