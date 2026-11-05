/**
 * Pembantu tes untuk route handler SSO (dipakai route.test.ts rute SSO): fetch palsu
 * ke API Go dan pembacaan Set-Cookie. Bukan bagian aplikasi; tidak diimpor oleh kode produksi.
 */
export interface PanggilanApi {
  url: string;
  method: string;
  headers: Record<string, string>;
  body: unknown;
}

type Penanggap = (panggilan: PanggilanApi) => Response | Promise<Response>;

/** Mengganti fetch global; kembalikan daftar panggilan dan fungsi pemulih. */
export function fetchPalsu(penanggap: Penanggap): {
  panggilan: PanggilanApi[];
  pulihkan: () => void;
} {
  const asli = globalThis.fetch;
  const panggilan: PanggilanApi[] = [];
  globalThis.fetch = (async (
    input: string | URL | Request,
    init?: RequestInit,
  ) => {
    const headers: Record<string, string> = {};
    new Headers(init?.headers).forEach((v, k) => (headers[k] = v));
    const p: PanggilanApi = {
      url: String(input),
      method: init?.method ?? "GET",
      headers,
      body: typeof init?.body === "string" ? JSON.parse(init.body) : undefined,
    };
    panggilan.push(p);
    return penanggap(p);
  }) as typeof fetch;
  return { panggilan, pulihkan: () => (globalThis.fetch = asli) };
}

/** Amplop sukses API Go: { code: "200", message: "OK", data }. */
export function amplopSukses(data: unknown): Response {
  return Response.json({ code: "200", message: "OK", data });
}

/** Amplop galat API Go: { code, message } dengan status HTTP. */
export function amplopGalat(
  status: number,
  code: string,
  message = "galat",
): Response {
  return Response.json({ code, message }, { status });
}

/** Seluruh header Set-Cookie sebuah respons. */
export function setCookies(response: Response): string[] {
  return response.headers.getSetCookie();
}

/** Set-Cookie untuk nama tertentu, atau undefined. */
export function cookieDiset(
  response: Response,
  nama: string,
): string | undefined {
  return setCookies(response).find((c) => c.startsWith(`${nama}=`));
}

/** Nilai cookie dari baris Set-Cookie. */
export function nilaiCookie(baris: string | undefined): string | undefined {
  return baris?.split(";")[0].split("=").slice(1).join("=");
}

/**
 * Menjalankan fungsi yang mengimpor modul dengan setInterval di tingkat modul (rate-limit.ts) sambil
 * meng-unref timernya, supaya proses tes bisa berakhir.
 */
export async function impor<T>(muat: () => Promise<T>): Promise<T> {
  const asli = globalThis.setInterval;
  globalThis.setInterval = ((...args: Parameters<typeof setInterval>) => {
    const timer = asli(...args);
    timer.unref?.();
    return timer;
  }) as typeof setInterval;
  try {
    return await muat();
  } finally {
    globalThis.setInterval = asli;
  }
}
