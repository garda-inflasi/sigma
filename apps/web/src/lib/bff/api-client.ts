/**
 * Typed API client for the BFF layer to communicate with the Go backend.
 * This runs only on the server side (Route Handlers / Server Actions).
 */

import "server-only";

import type { BulkColdStorageWire, ColdStorageWire, DetailFasilitasWire } from "@/lib/api/cold-storage";
import { toQueryString, type ColdStorageBulkAction, type ColdStorageParams } from "@/lib/api/cold-storage-query";

const API_BASE = process.env.GO_API_BASE_URL || "http://localhost:8080/v1";

interface ApiResponse<T> {
  code: string;
  message: string;
  data: T;
}

interface ApiErrorResponse {
  error?: boolean;
  code?: string;
  message?: string;
}

export class BffApiError extends Error {
  constructor(
    public status: number,
    message: string,
    public code?: string
  ) {
    super(message);
  }
}

/**
 * IP dan user agent peramban asli untuk diteruskan ke Go. Tanpa ini Go hanya melihat container web:
 * daftar sesi aktif dan audit akan menampilkan alamat internal dan "Node.js" untuk semua orang.
 * Go hanya memercayai X-Forwarded-For dari proxy yang dikenalnya (TRUSTED_PROXIES, bawaan jaringan privat).
 * Di luar lingkup request (tes) tidak ada yang diteruskan.
 */
export async function headerKlien(): Promise<Record<string, string>> {
  try {
    const { headers } = await import("next/headers");
    const masuk = await headers();
    const keluar: Record<string, string> = {};
    const userAgent = masuk.get("user-agent");
    if (userAgent) keluar["User-Agent"] = userAgent;
    const diteruskan = masuk.get("x-forwarded-for");
    if (diteruskan) keluar["X-Forwarded-For"] = diteruskan;
    return keluar;
  } catch {
    return {};
  }
}

/**
 * Internal fetch to the Go backend.
 * This is server-to-server, no cookies or CSRF needed.
 */
async function apiFetch<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const url = `${API_BASE}${path}`;

  const res = await fetch(url, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
      ...(await headerKlien()),
      ...options.headers,
    },
  });

  if (!res.ok) {
    let errorBody: ApiErrorResponse | undefined;
    try {
      errorBody = await res.json();
    } catch {
      // ignore parse errors
    }
    throw new BffApiError(
      res.status,
      errorBody?.message || `Backend error (${res.status})`,
      errorBody?.code
    );
  }

  // Handle 204 No Content
  if (res.status === 204) {
    return undefined as T;
  }

  // Unwrap Go API response: { code, message, data: T } → T
  const body: ApiResponse<T> = await res.json();
  if (body.code !== "200" && body.code !== "201") {
    throw new BffApiError(res.status, body.message || "Unknown error", body.code);
  }
  return body.data;
}

// ── Auth API ──────────────────────────────────────────────────────────

export interface LoginRequest {
  email: string;
  password: string;
}

/**
 * Hasil login. Akun ber-MFA menerima `mfa_required` dan `mfa_token` tanpa sesi; selain itu token sesi dan
 * profil terisi, dan `mfa_setup_diwajibkan` true berarti sesi hanya membuka endpoint setup MFA.
 */
export interface LoginResponse {
  access_token?: string;
  refresh_token?: string;
  user?: UserResponse;
  mfa_required?: boolean;
  mfa_token?: string;
  mfa_setup_diwajibkan?: boolean;
}

/** Sesi penuh hasil login atau validasi MFA. */
export type SessionResponse = Required<Pick<LoginResponse, "access_token" | "refresh_token" | "user">> &
  Pick<LoginResponse, "mfa_setup_diwajibkan">;

export interface MfaSetupResponse {
  secret: string;
  provisioning_uri: string;
  /** Kode QR provisioning_uri sebagai dokumen SVG. */
  qr_svg: string;
}

export interface MfaVerifyResponse {
  access_token: string;
  refresh_token: string;
  /** Hanya tampil sekali. */
  kode_cadangan: string[];
}

export interface RefreshTokenRequest {
  refresh_token: string;
}

export interface RefreshTokenResponse {
  access_token: string;
  /**
   * Refresh token pengganti. Tidak ada pada pemutaran ulang dalam jendela toleransi (dua tab menyegarkan
   * bersamaan): pertahankan cookie refresh yang sudah ada.
   */
  refresh_token?: string;
}

/** Satu sesi aktif (GET /v1/auth/sessions). */
export interface SesiWire {
  id: string;
  perangkat: string;
  user_agent?: string;
  ip?: string;
  dibuat_pada: string;
  terakhir_dipakai_pada: string;
  kedaluwarsa_pada: string;
  saat_ini: boolean;
}

/** Jumlah sesi yang dicabut oleh aksi massal. */
export interface SesiDicabutWire {
  dicabut: number;
}

export interface UserResponse {
  id: string;
  nama: string;
  email: string;
  peran: string;
  lapis: string;
  lembaga: string;
  kabupaten_kode?: string;
  mfa_aktif: boolean;
  status: string;
  terakhir_masuk?: string;
  dibuat_pada: string;
  /** Selama true, API menjawab 403 PASSWORD_CHANGE_REQUIRED selain untuk ganti kata sandi, keluar, dan profil. */
  wajib_ganti_password: boolean;
  /** Hanya pada GET /auth/me: akun wajib MFA tetapi belum aktif (403 MFA_SETUP_REQUIRED selain endpoint setup). */
  mfa_setup_diwajibkan?: boolean;
  /** Hanya pada GET /auth/me: MFA tidak boleh dinonaktifkan pemilik akun. */
  mfa_wajib?: boolean;
  /** Hanya pada GET /auth/me: sisa kode cadangan bila MFA aktif. */
  kode_cadangan_tersisa?: number;
}

export interface ChangePasswordRequest {
  old_password: string;
  new_password: string;
}

/** Sesi baru: ganti kata sandi mencabut sesi lama. */
export interface ChangePasswordResponse {
  access_token: string;
  refresh_token: string;
}

export interface ForgotPasswordRequest {
  email: string;
}

export interface ResetPasswordRequest {
  token: string;
  password: string;
}

// ── User Management Types ─────────────────────────────────────────────

export interface ListUsersParams {
  page?: number;
  limit?: number;
  search?: string;
}

export interface ListUsersResponse {
  items: UserResponse[];
  total: number;
  page: number;
  limit: number;
}

export interface CreateUserRequest {
  id: string;
  nama: string;
  email: string;
  password: string;
  peran: string;
  lapis: string;
  lembaga: string;
}

export interface UpdateUserRequest {
  nama?: string;
  email?: string;
  peran?: string;
  lapis?: string;
  lembaga?: string;
  kabupaten_kode?: string;
  status?: string;
}

export interface BulkUserRequest {
  action: "aktifkan" | "nonaktifkan" | "wajibkan_mfa" | "akhiri_sesi";
  ids: string[];
}

export interface BulkUserResponse {
  berhasil: number;
  gagal: number;
  errors?: string[];
}

export interface RoleResponse {
  id: string;
  permissions: string[];
}

export interface PermissionResponse {
  id: string;
}

export const authApi = {
  login: (data: LoginRequest) =>
    apiFetch<LoginResponse>("/auth/login", {
      method: "POST",
      body: JSON.stringify(data),
    }),

  refresh: (data: RefreshTokenRequest) =>
    apiFetch<RefreshTokenResponse>("/auth/refresh", {
      method: "POST",
      body: JSON.stringify(data),
    }),

  /**
   * Mengakhiri hanya sesi pemanggil. `refreshToken` (cookie refresh) dikirim sebagai cadangan agar server
   * dapat menentukan sesinya bila access token tidak membawa `sid`.
   */
  logout: (accessToken: string, refreshToken?: string) =>
    apiFetch<void>("/auth/logout", {
      method: "POST",
      headers: {
        Authorization: `Bearer ${accessToken}`,
      },
      body: JSON.stringify(refreshToken ? { refresh_token: refreshToken } : {}),
    }),

  /** Keluar dari semua perangkat, termasuk sesi pemanggil. */
  logoutAll: (accessToken: string) =>
    apiFetch<SesiDicabutWire>("/auth/logout-all", {
      method: "POST",
      headers: { Authorization: `Bearer ${accessToken}` },
    }),

  sessions: (accessToken: string) =>
    apiFetch<SesiWire[]>("/auth/sessions", {
      method: "GET",
      headers: { Authorization: `Bearer ${accessToken}` },
    }),

  revokeSession: (accessToken: string, id: string) =>
    apiFetch<{ id: string }>(`/auth/sessions/${encodeURIComponent(id)}`, {
      method: "DELETE",
      headers: { Authorization: `Bearer ${accessToken}` },
    }),

  revokeOtherSessions: (accessToken: string) =>
    apiFetch<SesiDicabutWire>("/auth/sessions/revoke-others", {
      method: "POST",
      headers: { Authorization: `Bearer ${accessToken}` },
    }),

  me: (accessToken: string) =>
    apiFetch<UserResponse>("/auth/me", {
      method: "GET",
      headers: {
        Authorization: `Bearer ${accessToken}`,
      },
    }),

  changePassword: (
    accessToken: string,
    data: ChangePasswordRequest
  ) =>
    apiFetch<ChangePasswordResponse>("/auth/change-password", {
      method: "POST",
      headers: {
        Authorization: `Bearer ${accessToken}`,
      },
      body: JSON.stringify(data),
    }),

  /** Menukar token tantangan dari login dan kode OTP/cadangan dengan sesi. */
  mfaValidate: (data: { mfa_token: string; code: string }) =>
    apiFetch<LoginResponse>("/auth/mfa/validate", {
      method: "POST",
      body: JSON.stringify(data),
    }),

  mfaSetup: (accessToken: string) =>
    apiFetch<MfaSetupResponse>("/auth/mfa/setup", {
      method: "POST",
      headers: { Authorization: `Bearer ${accessToken}` },
    }),

  mfaVerify: (accessToken: string, code: string) =>
    apiFetch<MfaVerifyResponse>("/auth/mfa/verify", {
      method: "POST",
      headers: { Authorization: `Bearer ${accessToken}` },
      body: JSON.stringify({ code }),
    }),

  mfaBackupCodes: (accessToken: string, password: string) =>
    apiFetch<{ kode_cadangan: string[] }>("/auth/mfa/backup-codes", {
      method: "POST",
      headers: { Authorization: `Bearer ${accessToken}` },
      body: JSON.stringify({ password }),
    }),

  mfaDisable: (accessToken: string, password: string) =>
    apiFetch<void>("/auth/mfa/disable", {
      method: "POST",
      headers: { Authorization: `Bearer ${accessToken}` },
      body: JSON.stringify({ password }),
    }),

  forgotPassword: (data: ForgotPasswordRequest) =>
    apiFetch<void>("/auth/forgot-password", {
      method: "POST",
      body: JSON.stringify(data),
    }),

  resetPassword: (data: ResetPasswordRequest) =>
    apiFetch<void>("/auth/reset-password", {
      method: "POST",
      body: JSON.stringify(data),
    }),
};

// ── User Management API ───────────────────────────────────────────────

const bearer = (accessToken: string) => ({
  Authorization: `Bearer ${accessToken}`,
});

export const userApi = {
  list: (accessToken: string, params: ListUsersParams = {}) => {
    const qs = new URLSearchParams();
    if (params.page) qs.set("page", String(params.page));
    if (params.limit) qs.set("limit", String(params.limit));
    if (params.search) qs.set("search", params.search);
    const query = qs.toString();
    return apiFetch<ListUsersResponse>(`/users${query ? `?${query}` : ""}`, {
      method: "GET",
      headers: bearer(accessToken),
    });
  },

  get: (accessToken: string, id: string) =>
    apiFetch<UserResponse>(`/users/${encodeURIComponent(id)}`, {
      method: "GET",
      headers: bearer(accessToken),
    }),

  create: (accessToken: string, data: CreateUserRequest) =>
    apiFetch<void>("/users", {
      method: "POST",
      headers: bearer(accessToken),
      body: JSON.stringify(data),
    }),

  update: (accessToken: string, id: string, data: UpdateUserRequest) =>
    apiFetch<UserResponse>(`/users/${encodeURIComponent(id)}`, {
      method: "PATCH",
      headers: bearer(accessToken),
      body: JSON.stringify(data),
    }),

  remove: (accessToken: string, id: string) =>
    apiFetch<{ id: string }>(`/users/${encodeURIComponent(id)}`, {
      method: "DELETE",
      headers: bearer(accessToken),
    }),

  /** Paksa keluar: mengakhiri semua sesi seorang pengguna (izin pengguna.kelola). */
  forceLogout: (accessToken: string, id: string) =>
    apiFetch<{ id: string; dicabut: number }>(
      `/users/${encodeURIComponent(id)}/revoke-sessions`,
      {
        method: "POST",
        headers: bearer(accessToken),
      }
    ),

  resetPassword: (accessToken: string, id: string, newPassword: string) =>
    apiFetch<{ id: string }>(
      `/users/${encodeURIComponent(id)}/reset-password`,
      {
        method: "POST",
        headers: bearer(accessToken),
        body: JSON.stringify({ new_password: newPassword }),
      }
    ),

  assignRole: (accessToken: string, id: string, peran: string) =>
    apiFetch<UserResponse>(`/users/${encodeURIComponent(id)}/roles`, {
      method: "PUT",
      headers: bearer(accessToken),
      body: JSON.stringify({ peran }),
    }),

  bulk: (accessToken: string, data: BulkUserRequest) =>
    apiFetch<BulkUserResponse>("/users/bulk", {
      method: "POST",
      headers: bearer(accessToken),
      body: JSON.stringify(data),
    }),

  listRoles: (accessToken: string) =>
    apiFetch<RoleResponse[]>("/roles", {
      method: "GET",
      headers: bearer(accessToken),
    }),

  listPermissions: (accessToken: string) =>
    apiFetch<PermissionResponse[]>("/permissions", {
      method: "GET",
      headers: bearer(accessToken),
    }),
};

// ── Cold Storage API ──────────────────────────────────────────────────

export interface BulkColdStorageRequest {
  action: ColdStorageBulkAction;
  ids: string[];
}

export const coldStorageApi = {
  /** GET /v1/cold-storage — kartu fasilitas, satu halaman stok, dan ringkasan. */
  list: (accessToken: string, params: ColdStorageParams = {}) => {
    const query = toQueryString(params);
    return apiFetch<ColdStorageWire>(`/cold-storage${query ? `?${query}` : ""}`, {
      method: "GET",
      headers: bearer(accessToken),
    });
  },

  /** GET /v1/cold-storage/fasilitas/:id — riwayat suhu, lot, dan transaksi satu fasilitas. */
  detail: (accessToken: string, id: string) =>
    apiFetch<DetailFasilitasWire>(`/cold-storage/fasilitas/${encodeURIComponent(id)}`, {
      method: "GET",
      headers: bearer(accessToken),
    }),

  /** PATCH /v1/cold-storage/fasilitas/:id — kapasitas, batas alarm suhu, atau status. */
  ubahFasilitas: (accessToken: string, id: string, body: Record<string, unknown>) =>
    apiFetch<unknown>(`/cold-storage/fasilitas/${encodeURIComponent(id)}`, {
      method: "PATCH",
      headers: bearer(accessToken),
      body: JSON.stringify(body),
    }),

  /** POST /v1/cold-storage/telemetri — catat pembacaan suhu (idempoten per fasilitas + waktu). */
  telemetri: (accessToken: string, body: Record<string, unknown>) =>
    apiFetch<unknown>("/cold-storage/telemetri", {
      method: "POST",
      headers: bearer(accessToken),
      body: JSON.stringify(body),
    }),

  /** POST /v1/cold-storage/transaksi — catat stok masuk, keluar, atau pindah. */
  transaksi: (accessToken: string, body: Record<string, unknown>) =>
    apiFetch<unknown>("/cold-storage/transaksi", {
      method: "POST",
      headers: bearer(accessToken),
      body: JSON.stringify(body),
    }),

  /** POST /v1/cold-storage/bulk — aksi massal pada lot stok. */
  bulk: (accessToken: string, data: BulkColdStorageRequest) =>
    apiFetch<BulkColdStorageWire>("/cold-storage/bulk", {
      method: "POST",
      headers: bearer(accessToken),
      body: JSON.stringify(data),
    }),
};
