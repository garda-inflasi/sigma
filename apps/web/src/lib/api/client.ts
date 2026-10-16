/**
 * Klien HTTP ke backend Go (REST, JSON, ISO 8601). Sesi via cookie httpOnly
 * yang diterbitkan layanan auth (SSO + MFA). Tanpa NEXT_PUBLIC_API_BASE_URL → mode mock.
 */
export const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";
export const MODE_MOCK = !API_BASE;

export class ApiError extends Error {
  constructor(public status: number, message: string, public payload?: unknown) {
    super(message);
  }
}

export async function apiFetch<T>(path: string, init: RequestInit & { idempotencyKey?: string } = {}): Promise<T> {
  const { idempotencyKey, headers, ...rest } = init;
  const res = await fetch(`${API_BASE}${path}`, {
    credentials: "include",
    ...rest,
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
      ...(idempotencyKey ? { "Idempotency-Key": idempotencyKey } : {}),
      ...headers,
    },
  });
  if (!res.ok) {
    let payload: unknown;
    try {
      payload = await res.json();
    } catch {}
    const msg = (payload as { message?: string } | undefined)?.message ?? `Permintaan gagal (${res.status})`;
    throw new ApiError(res.status, msg, payload);
  }
  return res.status === 204 ? (undefined as T) : ((await res.json()) as T);
}

/**
 * Klien ke BFF (Route Handlers di origin yang sama). Selalu memakai path
 * relatif sehingga cookie sesi httpOnly ikut terkirim, dan menyertakan
 * header `x-csrf-token` untuk permintaan yang mengubah state.
 */
export async function bffFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const method = (init.method ?? "GET").toUpperCase();
  const mutating = method !== "GET" && method !== "HEAD";

  const res = await fetch(path, {
    credentials: "same-origin",
    ...init,
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
      ...(mutating ? { "x-csrf-token": readCsrfToken() } : {}),
      ...init.headers,
    },
  });

  if (!res.ok) {
    let payload: unknown;
    try {
      payload = await res.json();
    } catch {}
    const msg =
      (payload as { message?: string } | undefined)?.message ??
      `Permintaan gagal (${res.status})`;
    throw new ApiError(res.status, msg, payload);
  }

  return res.status === 204 ? (undefined as T) : ((await res.json()) as T);
}

/** Membaca token CSRF dari cookie non-httpOnly (pola double-submit). */
function readCsrfToken(): string {
  if (typeof document === "undefined") return "";
  const match = document.cookie.match(/(?:^|;\s*)garda_csrf=([^;]+)/);
  return match ? decodeURIComponent(match[1]) : "";
}

/** Latensi buatan di mode mock agar skeleton shimmer & transisi dapat ditinjau. */
export function mockDelay<T>(value: T | (() => T), min = 380, max = 900): Promise<T> {
  const ms = min + Math.random() * (max - min);
  return new Promise((r) => setTimeout(() => r(typeof value === "function" ? (value as () => T)() : value), ms));
}
