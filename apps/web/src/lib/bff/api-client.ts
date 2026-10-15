/**
 * Typed API client for the BFF layer to communicate with the Go backend.
 * This runs only on the server side (Route Handlers / Server Actions).
 */

import "server-only";

const API_BASE = process.env.GO_API_BASE_URL || "http://localhost:8080/v1";

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

  return res.json() as Promise<T>;
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
