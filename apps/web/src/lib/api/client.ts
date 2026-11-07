/**
 * Klien HTTP ke backend Go (REST, JSON, ISO 8601). Sesi via cookie httpOnly
 * yang diterbitkan layanan auth (SSO + MFA). Tanpa NEXT_PUBLIC_API_BASE_URL → mode mock.
 */
import { fetchSesi } from "@/lib/auth/sesi-client";

export const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";
export const MODE_MOCK = !API_BASE;

export class ApiError extends Error {
  constructor(public status: number, message: string, public payload?: unknown) {
    super(message);
  }
}

async function kirimApi<T>(path: string, init: RequestInit & { idempotencyKey?: string }): Promise<{ data: T; res: Response }> {
  const { idempotencyKey, headers, ...rest } = init;
  // 401 karena access token habis/dicabut → refresh senyap lalu ulangi sekali (lihat sesi-client).
  const res = await fetchSesi(`${API_BASE}${path}`, {
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
  if (res.status === 204) return { data: undefined as T, res };
  const body = (await res.json()) as unknown;
  // Backend Go membungkus semua respons sebagai { code, message, data }.
  // Envelope di-unwrap di satu tempat ini agar pemanggil menerima payload langsung.
  if (body && typeof body === "object" && "code" in body && "data" in body) {
    return { data: (body as { data: T }).data, res };
  }
  return { data: body as T, res };
}

export async function apiFetch<T>(path: string, init: RequestInit & { idempotencyKey?: string } = {}): Promise<T> {
  return (await kirimApi<T>(path, init)).data;
}

/** Jumlah seluruh baris dari header `X-Total-Count`; null bila header tidak ada atau bukan bilangan bulat tak negatif. */
export function bacaTotal(headers: Pick<Headers, "get">): number | null {
  const mentah = headers.get("X-Total-Count");
  if (mentah === null || !/^\d+$/.test(mentah.trim())) return null;
  const n = Number(mentah);
  return Number.isSafeInteger(n) ? n : null;
}

/** Daftar yang dipaginasi server: isi satu halaman beserta total dari `X-Total-Count` (null bila tidak dikirim). */
export async function apiFetchHalaman<T>(path: string, init: RequestInit = {}): Promise<{ data: T[]; total: number | null }> {
  const { data, res } = await kirimApi<T[]>(path, init);
  return { data: data ?? [], total: bacaTotal(res.headers) };
}

/**
 * Klien ke BFF (Route Handlers di origin yang sama). Selalu memakai path
 * relatif sehingga cookie sesi httpOnly ikut terkirim, dan menyertakan
 * header `x-csrf-token` untuk permintaan yang mengubah state.
 */
export async function bffFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const method = (init.method ?? "GET").toUpperCase();
  const mutating = method !== "GET" && method !== "HEAD";

  // Init berupa fungsi: token CSRF dibaca ulang saat permintaan diulang setelah refresh.
  const res = await fetchSesi(path, () => ({
    credentials: "same-origin",
    ...init,
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
      ...(mutating ? { "x-csrf-token": readCsrfToken() } : {}),
      ...init.headers,
    },
  }));

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
  // Galat yang dilempar fungsi mock (mis. ApiError 404) menolak promise-nya, bukan menjadi galat tak tertangkap di timer.
  return new Promise((resolve, reject) =>
    setTimeout(() => {
      try {
        resolve(typeof value === "function" ? (value as () => T)() : value);
      } catch (err) {
        reject(err);
      }
    }, ms),
  );
}
