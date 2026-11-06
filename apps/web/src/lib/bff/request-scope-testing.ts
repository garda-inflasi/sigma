/**
 * Pembantu tes: menjalankan kode yang memakai `cookies()` / `headers()` dari next/headers di luar server Next.
 * Fungsi-fungsi itu hanya hidup di dalam "request scope" (AsyncLocalStorage milik Next); di sini scope itu
 * dipasang manual dengan cookie yang ditentukan tes. Bergantung pada struktur internal Next, jadi bila Next
 * mengubahnya tes yang memakai ini gagal dengan jelas dan cukup pembantu ini yang disesuaikan.
 * Bukan bagian aplikasi; tidak diimpor oleh kode produksi.
 */
import { AsyncLocalStorage } from "node:async_hooks";

// Next membaca konstruktor ini dari global saat modul penyimpanannya dimuat (diatur server Next saat start);
// tanpanya penyimpanan itu berupa tiruan yang melempar. Karena itu berkas ini harus diimpor SEBELUM modul Next
// lain pada tes yang memakainya.
(globalThis as Record<string, unknown>).AsyncLocalStorage ??= AsyncLocalStorage;

/** Menjalankan `jalankan` seolah-olah sedang melayani permintaan yang membawa header Cookie `cookie`. */
export async function denganCookie<T>(
  cookie: string,
  jalankan: () => Promise<T>,
): Promise<T> {
  const { workAsyncStorage } =
    await import("next/dist/server/app-render/work-async-storage.external");
  const { workUnitAsyncStorage } =
    await import("next/dist/server/app-render/work-unit-async-storage.external");
  const { RequestCookies } =
    await import("next/dist/compiled/@edge-runtime/cookies");

  const cookies = new RequestCookies(new Headers(cookie ? { cookie } : {}));
  return workAsyncStorage.run({ route: "/uji" } as never, () =>
    workUnitAsyncStorage.run(
      // "private-cache" membuat cookies() mengembalikan penyimpanan ini apa adanya; phase "render" lolos
      // pemeriksaan fase API permintaan.
      { type: "private-cache", phase: "render", cookies } as never,
      jalankan,
    ),
  );
}
