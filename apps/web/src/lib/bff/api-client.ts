/**
 * Typed API client for the BFF layer to communicate with the Go backend.
 * This runs only on the server side (Route Handlers / Server Actions).
 */

import "server-only";

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

export interface LoginResponse {
  access_token: string;
  refresh_token: string;
  user: UserResponse;
}

export interface RefreshTokenRequest {
  refresh_token: string;
}

export interface RefreshTokenResponse {
  access_token: string;
  refresh_token: string;
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
}

export interface ChangePasswordRequest {
  old_password: string;
  new_password: string;
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

  logout: (accessToken: string) =>
    apiFetch<void>("/auth/logout", {
      method: "POST",
      headers: {
        Authorization: `Bearer ${accessToken}`,
      },
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
    apiFetch<void>("/auth/change-password", {
      method: "POST",
      headers: {
        Authorization: `Bearer ${accessToken}`,
      },
      body: JSON.stringify(data),
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
