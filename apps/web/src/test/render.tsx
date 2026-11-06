/**
 * Perangkat tes komponen: render dengan Provider yang sama seperti aplikasi (React Query, sesi, router Next),
 * `fetch` palsu berbasis rute, dan URL halaman yang hidup (pushState/replaceState memperbarui `useSearchParams`).
 * Tidak ada modul yang dimock: konteks Next diisi langsung, jadi komponen berjalan dengan kode aslinya.
 */
import { GlobalRegistrator } from "@happy-dom/global-registrator";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MotionGlobalConfig } from "motion/react";
import { cleanup, render, type RenderResult } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AppRouterContext } from "next/dist/shared/lib/app-router-context.shared-runtime";
import { SearchParamsContext } from "next/dist/shared/lib/hooks-client-context.shared-runtime";
import { after, afterEach } from "node:test";
import { useSyncExternalStore, type ReactNode } from "react";
import { Toaster } from "sonner";
import { SessionProvider } from "@/lib/auth/session-context";
import type { SessionUser } from "@/lib/auth/resolve";
import { TooltipProvider } from "@/components/ui/tooltip";
import { ROLE_PERMISSIONS } from "@/lib/rbac/permissions";
import type { Permission } from "@/lib/rbac/permissions";

// Animasi langsung selesai: AnimatePresence melepas elemen yang keluar seketika, tanpa menunggu bingkai animasi.
MotionGlobalConfig.skipAnimations = true;

// RTL hanya membersihkan otomatis bila ada `afterEach` global (jest/vitest); di node:test dipasang manual.
afterEach(() => {
  cleanup();
  document.body.innerHTML = "";
});

// Timer happy-dom/Radix/Sonner yang tersisa menahan proses Node hidup sesudah tes selesai; menutup window
// membatalkan semuanya supaya `node --test` keluar sendiri.
after(async () => {
  await GlobalRegistrator.unregister();
});

// ── URL halaman yang hidup ───────────────────────────────────────────

/**
 * Meniru integrasi Next dengan History API: `pushState`/`replaceState` mengubah URL dan memberi tahu
 * `useSearchParams`. Dipasang sekali; `setUrl` mengatur ulang URL antartes.
 */
const pendengarUrl = new Set<() => void>();
let patched = false;
function pasangPatchHistory() {
  if (patched) return;
  patched = true;
  for (const nama of ["pushState", "replaceState"] as const) {
    const asli = window.history[nama].bind(window.history);
    window.history[nama] = (
      data: unknown,
      unused: string,
      url?: string | URL | null,
    ) => {
      asli(data, unused, url);
      pendengarUrl.forEach((p) => p());
    };
  }
}

export function setUrl(path: string) {
  // Atur ulang seluruh lokasi (bukan hanya riwayat): tes sebelumnya bisa saja meninggalkan origin lain.
  (window as unknown as { happyDOM: { setURL(url: string): void } }).happyDOM.setURL(`http://localhost:3000${path}`);
}

/**
 * Menangkap unduhan berkas (`<a download>.click()`): happy-dom akan menavigasi window ke href-nya, yang merusak
 * URL tes berikutnya. Mengembalikan daftar unduhan dan fungsi pemulih.
 */
export function tangkapUnduhan() {
  const asli = window.HTMLAnchorElement.prototype.click;
  const daftar: { href: string; download: string }[] = [];
  window.HTMLAnchorElement.prototype.click = function (this: HTMLAnchorElement) {
    daftar.push({ href: this.href, download: this.download });
  };
  const buat = URL.createObjectURL;
  const cabut = URL.revokeObjectURL;
  URL.createObjectURL = () => "blob:uji";
  URL.revokeObjectURL = () => {};
  return {
    daftar,
    pulihkan: () => {
      window.HTMLAnchorElement.prototype.click = asli;
      URL.createObjectURL = buat;
      URL.revokeObjectURL = cabut;
    },
  };
}

function useQueryUrl(): string {
  return useSyncExternalStore(
    (cb) => {
      pendengarUrl.add(cb);
      return () => pendengarUrl.delete(cb);
    },
    () => window.location.search,
    () => "",
  );
}

function UrlProvider({ children }: { children: ReactNode }) {
  const search = useQueryUrl();
  return (
    <SearchParamsContext.Provider value={new URLSearchParams(search)}>
      {children}
    </SearchParamsContext.Provider>
  );
}

// ── fetch palsu ──────────────────────────────────────────────────────

export interface PermintaanFetch {
  url: string;
  path: string;
  query: URLSearchParams;
  method: string;
  body: unknown;
  headers: Record<string, string>;
}

export type Penangan = (
  req: PermintaanFetch,
) => unknown | Response | Promise<unknown | Response>;

export interface FetchPalsu {
  /** Semua permintaan, urut. */
  panggilan: PermintaanFetch[];
  /** Permintaan yang cocok dengan metode dan path. */
  ke(method: string, path: string): PermintaanFetch[];
  /** Memasang/menimpa penangan untuk "METODE /path". */
  atur(rute: string, penangan: Penangan): void;
  atur(rute: string, data: unknown): void;
  pulihkan(): void;
}

const kepala = { "Content-Type": "application/json" };

/** Bungkus data seperti amplop API Go; apiFetch/bffFetch membuka amplop ini. */
export const amplop = (data: unknown, code = "200") => ({
  code,
  message: "OK",
  data,
});

/**
 * Menyadap `fetch` global. `rute` berbentuk "GET /api/pengguna"; penangan mengembalikan data (dijawab 200 JSON),
 * `Response`, atau `{ __status, body }`. Rute tak dikenal dijawab 500 dan dicatat agar tes gagal jelas.
 */
export function pasangFetchPalsu(
  rute: Record<string, Penangan | unknown> = {},
): FetchPalsu {
  const asli = globalThis.fetch;
  const penangan = new Map<string, Penangan | unknown>(Object.entries(rute));
  const panggilan: PermintaanFetch[] = [];

  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    const url =
      typeof input === "string"
        ? input
        : input instanceof URL
          ? input.toString()
          : input.url;
    const u = new URL(url, "http://localhost:3000");
    const method = (init?.method ?? "GET").toUpperCase();
    let body: unknown;
    if (typeof init?.body === "string") {
      try {
        body = JSON.parse(init.body);
      } catch {
        body = init.body;
      }
    }
    const req: PermintaanFetch = {
      url,
      path: u.pathname,
      query: u.searchParams,
      method,
      body,
      headers: Object.fromEntries(new Headers(init?.headers).entries()),
    };
    panggilan.push(req);
    const h = penangan.get(`${method} ${u.pathname}`);
    if (h === undefined) {
      return new Response(
        JSON.stringify({
          code: "500",
          message: `fetch palsu: ${method} ${u.pathname} tidak diatur`,
        }),
        { status: 500, headers: kepala },
      );
    }
    const hasil = typeof h === "function" ? await (h as Penangan)(req) : h;
    if (hasil instanceof Response) return hasil;
    if (hasil && typeof hasil === "object" && "__status" in hasil) {
      const { __status, body: badan } = hasil as {
        __status: number;
        body?: unknown;
      };
      return new Response(badan === undefined ? null : JSON.stringify(badan), {
        status: __status,
        headers: kepala,
      });
    }
    return new Response(JSON.stringify(hasil ?? {}), {
      status: 200,
      headers: kepala,
    });
  }) as typeof fetch;

  return {
    panggilan,
    ke: (method, path) =>
      panggilan.filter((p) => p.method === method && p.path === path),
    atur: (r, h) => void penangan.set(r, h),
    pulihkan: () => {
      globalThis.fetch = asli;
    },
  };
}

/** Jawaban galat API Go untuk dipakai penangan fetch. */
export const galatApi = (
  status: number,
  message: string,
  code = String(status),
) => ({ __status: status, body: { code, message } });

// ── Sesi ─────────────────────────────────────────────────────────────

/** Pengguna sesi untuk tes; `permissions` default = matriks bawaan perannya. */
export function penggunaSesi(sisip: Partial<SessionUser> = {}): SessionUser {
  const role = sisip.role ?? "ADMIN_SISTEM";
  return {
    id: "admin-1",
    nama: "Admin Satu",
    email: "admin@garda.test",
    role,
    lembaga: "BPPSDMP",
    wajibGantiPassword: false,
    permissions: [...ROLE_PERMISSIONS[role]] as Permission[],
    ...sisip,
  } as SessionUser;
}

// ── Render ───────────────────────────────────────────────────────────

export interface OpsiRender {
  /** Sesi; null = tanpa sesi. Bawaan: admin sistem. */
  sesi?: SessionUser | null;
  /** URL awal halaman (path + query). */
  url?: string;
  /** Router Next palsu; panggilan push/replace/refresh dicatat di sini. */
  router?: Partial<RouterPalsu>;
}

export interface RouterPalsu {
  push: (href: string) => void;
  replace: (href: string) => void;
  refresh: () => void;
  back: () => void;
  forward: () => void;
  prefetch: () => void;
}

export function renderHalaman(
  ui: ReactNode,
  opsi: OpsiRender = {},
): RenderResult & {
  user: ReturnType<typeof userEvent.setup>;
  qc: QueryClient;
  router: { push: string[]; replace: string[]; refresh: number };
} {
  pasangPatchHistory();
  setUrl(opsi.url ?? "/pengguna");
  const qc = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity, staleTime: 0 },
      mutations: { retry: false },
    },
  });
  const catatan = { push: [] as string[], replace: [] as string[], refresh: 0 };
  const router = {
    push: (h: string) => void catatan.push.push(h),
    replace: (h: string) => void catatan.replace.push(h),
    refresh: () => void catatan.refresh++,
    back: () => {},
    forward: () => {},
    prefetch: () => {},
    ...opsi.router,
  };
  const sesi = opsi.sesi === undefined ? penggunaSesi() : opsi.sesi;
  const bungkus = (isi: ReactNode) => (
    <QueryClientProvider client={qc}>
      <SessionProvider user={sesi}>
        <AppRouterContext.Provider value={router as never}>
          <UrlProvider>
            <TooltipProvider delayDuration={0}>
              {isi}
              <Toaster />
            </TooltipProvider>
          </UrlProvider>
        </AppRouterContext.Provider>
      </SessionProvider>
    </QueryClientProvider>
  );
  const hasil = render(bungkus(ui));
  return {
    ...hasil,
    // Render ulang tetap di dalam Provider yang sama (QueryClient, sesi, router).
    rerender: (isi: ReactNode) => hasil.rerender(bungkus(isi)),
    user: userEvent.setup({ pointerEventsCheck: 0 }),
    qc,
    router: catatan,
  };
}
