/**
 * Keputusan akses di edge (middleware) sebagai fungsi murni: masukan rute + cookie,
 * keluaran instruksi (lanjut / alihkan / JSON galat). Middleware hanya menerjemahkan
 * keputusan ini menjadi NextResponse, sehingga logikanya dapat diuji tanpa Next.
 *
 * Izin dibaca dari klaim JWT tanpa verifikasi tanda tangan (edge tidak memegang
 * rahasia): ini hanya UX. API Go tetap otoritatif dan menolak akses tanpa izin.
 */

import {
  MFA_SETUP_PATH,
  MFA_SETUP_REQUIRED_CODE,
  isMfaSetupExempt,
} from "@/lib/auth/mfa";
import {
  PASSWORD_CHANGE_PATH,
  PASSWORD_CHANGE_REQUIRED_CODE,
  isPasswordChangeExempt,
} from "@/lib/auth/password-change";
import { isExpired, readEdgeClaims } from "@/lib/auth/edge-claims";
import { ROLE_PERMISSIONS } from "./permissions";
import { ROLES } from "./roles";
import { requiredPermission } from "./routes";

/**
 * Rute yang tidak memerlukan sesi (halaman publik, alur masuk, aset). Langkah OTP login
 * (`/api/auth/mfa/validate`) publik karena sesi baru terbit setelah kode diterima; ia
 * memakai cookie tantangan berumur pendek dari `/api/auth/login`.
 * Masuk dengan Google (`/api/auth/sso/google` dan `/callback`) publik karena dipakai sebelum ada sesi;
 * keamanannya ada pada state/PKCE di API dan cookie alur berumur 10 menit (lib/bff/sso.ts).
 */
export const PUBLIC_ROUTES = [
  "/masuk",
  "/lupa-password",
  "/reset-password",
  "/api/auth/login",
  "/api/auth/mfa/validate",
  "/api/auth/sso",
  "/api/auth/refresh",
  // Keluar harus bisa dipanggil walau access token sudah kedaluwarsa (route-nya memulihkan sesi dulu
  // supaya sesi di server benar-benar dicabut); tanpa ini edge menjawab 401 dan refresh token tertinggal.
  "/api/auth/logout",
  "/api/auth/forgot-password",
  "/api/auth/reset-password",
  "/healthz",
  "/akses-ditolak",
  "/offline",
  "/_next/static",
  "/_next/image",
  "/favicon.ico",
];

export const ACCESS_DENIED_PATH = "/akses-ditolak";
export const LOGIN_PATH = "/masuk";
/** Pemulihan sesi saat navigasi halaman: menukar refresh token lalu kembali ke `lanjut` (GET di route BFF). */
export const RESUME_PATH = "/api/auth/refresh";

export function isPublicRoute(pathname: string): boolean {
  return PUBLIC_ROUTES.some(
    (route) => pathname === route || pathname.startsWith(`${route}/`),
  );
}

export type EdgeDecision =
  | { kind: "next" }
  | {
      kind: "redirect";
      path: string;
      query?: Record<string, string>;
      clearAccessCookie?: boolean;
      /** Hapus penanda pemulihan (`garda_pulih`) bersama pengalihan ini. */
      clearResumeCookie?: boolean;
    }
  | {
      kind: "json";
      status: 401 | 403;
      body: { error: true; message: string; code?: string };
      clearAccessCookie?: boolean;
    };

export interface EdgeInput {
  pathname: string;
  /** Query string asli (dengan "?" di depan) agar tujuan semula tidak kehilangan parameternya. */
  search?: string;
  accessToken?: string;
  refreshToken?: string;
  /** Penanda `garda_pulih` ada: sesi baru saja dipulihkan, jangan memulihkan lagi (pengaman loop). */
  pulihBaru?: boolean;
  /** Detik epoch saat ini (dapat disuntik untuk tes). */
  nowSeconds: number;
}

const isApi = (pathname: string) => pathname.startsWith("/api/");

/**
 * Tujuan semula untuk parameter `lanjut`: path beserta query-nya, tanpa parameter internal Next (`_rsc`)
 * yang tidak bermakna bagi halaman.
 */
export function tujuanSemula(pathname: string, search?: string): string {
  if (!search || search === "?") return pathname;
  const params = new URLSearchParams(search);
  params.delete("_rsc");
  const query = params.toString();
  return query ? `${pathname}?${query}` : pathname;
}

function toLogin(
  tujuan: string,
  clearAccessCookie = false,
  clearResumeCookie = false,
): EdgeDecision {
  return {
    kind: "redirect",
    path: LOGIN_PATH,
    query: { lanjut: tujuan },
    clearAccessCookie,
    ...(clearResumeCookie ? { clearResumeCookie } : {}),
  };
}

export function decideEdge({
  pathname,
  search,
  accessToken,
  refreshToken,
  pulihBaru,
  nowSeconds,
}: EdgeInput): EdgeDecision {
  if (isPublicRoute(pathname)) return { kind: "next" };

  const claims = accessToken ? readEdgeClaims(accessToken) : null;

  // Tanpa token akses yang sah: tak ada token sama sekali, token rusak, atau kedaluwarsa.
  if (!claims || isExpired(claims, nowSeconds)) {
    const clearAccessCookie = Boolean(accessToken);
    if (isApi(pathname)) {
      const expired = Boolean(refreshToken) || Boolean(claims);
      return {
        kind: "json",
        status: 401,
        body: {
          error: true,
          message: expired ? "Token expired" : "Unauthorized",
        },
        clearAccessCookie,
      };
    }
    const tujuan = tujuanSemula(pathname, search);
    // Halaman: access token (15 menit) habis tetapi refresh token (7 hari) mungkin masih sah. Refresh
    // token hanya dikirim browser ke /api/auth, jadi edge tidak bisa melihatnya; kirim navigasi lewat
    // GET /api/auth/refresh yang menukarnya lalu kembali ke tujuan semula (atau ke /masuk bila tidak ada
    // refresh token atau sudah dicabut). Penanda pulihBaru memutus loop bila cookie baru tetap tak terpakai.
    if (pulihBaru) return toLogin(tujuan, clearAccessCookie, true);
    return {
      kind: "redirect",
      path: RESUME_PATH,
      query: { lanjut: tujuan },
      clearAccessCookie,
    };
  }

  // Akun yang wajib mengganti kata sandi hanya boleh membuka halaman ganti kata sandi.
  if (claims.wajibGantiPassword && !isPasswordChangeExempt(pathname)) {
    if (isApi(pathname)) {
      return {
        kind: "json",
        status: 403,
        body: {
          error: true,
          code: PASSWORD_CHANGE_REQUIRED_CODE,
          message: "Anda harus mengganti kata sandi sebelum melanjutkan.",
        },
      };
    }
    return {
      kind: "redirect",
      path: PASSWORD_CHANGE_PATH,
      query: { lanjut: pathname },
    };
  }

  // Peran wajib MFA yang belum menyiapkannya hanya boleh membuka halaman setup MFA
  // (setelah ganti kata sandi bila itu juga diwajibkan).
  if (claims.wajibSetupMfa && !isMfaSetupExempt(pathname)) {
    if (isApi(pathname)) {
      return {
        kind: "json",
        status: 403,
        body: {
          error: true,
          code: MFA_SETUP_REQUIRED_CODE,
          message:
            "Anda harus mengaktifkan verifikasi dua langkah sebelum melanjutkan.",
        },
      };
    }
    return {
      kind: "redirect",
      path: MFA_SETUP_PATH,
      query: { lanjut: pathname },
    };
  }

  // Akar situs: ke beranda sesuai peran pengguna.
  if (pathname === "/")
    return { kind: "redirect", path: ROLES[claims.role].beranda };

  // Izin rute. Daftar izin dari token; bila klaim tidak ada, pakai matriks peran.
  const needed = requiredPermission(pathname);
  if (needed) {
    const granted = claims.permissions ?? ROLE_PERMISSIONS[claims.role];
    if (!granted.includes(needed)) {
      if (isApi(pathname)) {
        return {
          kind: "json",
          status: 403,
          body: { error: true, message: "Forbidden" },
        };
      }
      // Salinan izin di token bisa saja basi: admin baru memberi izin langsung atau mengubah matriks peran
      // (02-C), sedangkan access token membawa izin saat terbit. Sekali saja, tukar refresh token (token baru
      // membawa izin terkini) lalu coba rute yang sama; penanda pulihBaru (20 detik) memutus loop, jadi pengguna
      // yang memang tidak berhak hanya menempuh satu pengalihan tambahan sebelum /akses-ditolak.
      if (!pulihBaru) {
        return {
          kind: "redirect",
          path: RESUME_PATH,
          query: { lanjut: tujuanSemula(pathname, search) },
        };
      }
      return {
        kind: "redirect",
        path: ACCESS_DENIED_PATH,
        query: { dari: pathname },
      };
    }
  }

  return { kind: "next" };
}
