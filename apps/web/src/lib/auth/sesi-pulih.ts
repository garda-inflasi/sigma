/**
 * Pemulihan sesi senyap di klien (logika murni, tanpa global browser agar teruji di Node).
 *
 * Access token berumur 15 menit dan cookie-nya ikut lenyap saat itu; refresh token (7 hari) tetap ada
 * di cookie httpOnly yang hanya terkirim ke /api/auth. Saat permintaan data menerima 401, klien menukar
 * refresh token lewat `POST /api/auth/refresh` (single-flight) lalu mengulang permintaannya satu kali.
 * Hanya bila refresh sendiri ditolak (sesi benar-benar berakhir) pengguna diarahkan ke /masuk.
 *
 * Beberapa tab: cookie dipakai bersama, jadi dua tab yang 401 bersamaan akan sama-sama menukar token yang
 * sama. Server menoleransi itu (jendela toleransi refresh), tetapi demi menghindari rotasi ganda, tab dikunci
 * (Web Locks) dan tab yang menunggu giliran melewati refresh bila tab lain baru saja berhasil memperbaruinya
 * setelah permintaannya dimulai.
 */

export type HasilPulih = "ok" | "berakhir" | "gagal";

/** Jalur BFF yang 401-nya bukan tanda sesi kedaluwarsa (alur masuk itu sendiri). */
const TANPA_PEMULIHAN = [
  "/api/auth/login",
  "/api/auth/refresh",
  "/api/auth/logout",
  "/api/auth/mfa/validate",
  "/api/auth/forgot-password",
  "/api/auth/reset-password",
];

/** Pesan 401 karena kredensial yang diketik salah (bukan token): mengulangnya hanya melipatgandakan percobaan. */
const GALAT_KREDENSIAL = /password|kata sandi/i;

function pathDari(url: string): string {
  try {
    return new URL(url, "http://lokal").pathname;
  } catch {
    return url;
  }
}

/** Apakah 401 ini layak dibalas pemulihan sesi lalu pengulangan permintaan. */
export function perluPulih(
  url: string,
  status: number,
  pesan: string | undefined,
): boolean {
  if (status !== 401) return false;
  const path = pathDari(url);
  if (TANPA_PEMULIHAN.some((p) => path === p || path.startsWith(`${p}/`)))
    return false;
  return !GALAT_KREDENSIAL.test(pesan ?? "");
}

export interface CatatanSegar {
  /** Waktu (ms epoch) refresh berhasil. */
  t: number;
}

export interface PemulihDeps {
  /** POST /api/auth/refresh; melempar bila jaringan putus. */
  refresh: () => Promise<{ status: number }>;
  /** Mengunci antar-tab (navigator.locks); tanpa ini hanya single-flight dalam tab. */
  kunci?: <T>(jalankan: () => Promise<T>) => Promise<T>;
  /** Catatan refresh berhasil terakhir yang dibagi antar-tab (localStorage). */
  baca?: () => CatatanSegar | null;
  tulis?: (catatan: CatatanSegar) => void;
  sekarang?: () => number;
  /** Dipanggil sekali bila sesi dinyatakan berakhir (arahkan ke /masuk). */
  saatBerakhir?: () => void;
}

export interface Pemulih {
  /**
   * Memulihkan sesi. `sejak` adalah waktu (ms) permintaan yang gagal dimulai: refresh tab lain yang selesai
   * sesudahnya sudah cukup, tidak perlu memutar token lagi.
   */
  (sejak?: number): Promise<HasilPulih>;
  /** True setelah sesi dinyatakan berakhir; pemanggilan berikutnya langsung "berakhir". */
  berakhir(): boolean;
}

export function buatPemulih(deps: PemulihDeps): Pemulih {
  const sekarang = deps.sekarang ?? Date.now;
  let berjalan: Promise<HasilPulih> | null = null;
  let sudahBerakhir = false;

  const tukar = async (sejak: number | undefined): Promise<HasilPulih> => {
    const catatan = deps.baca?.();
    if (catatan && sejak !== undefined && catatan.t >= sejak) return "ok";
    let res: { status: number };
    try {
      res = await deps.refresh();
    } catch {
      return "gagal";
    }
    if (res.status >= 200 && res.status < 300) {
      deps.tulis?.({ t: sekarang() });
      return "ok";
    }
    // 401 = refresh token ditolak (tidak sah, dicabut, kedaluwarsa). 5xx/429 = sementara.
    return res.status === 401 ? "berakhir" : "gagal";
  };

  const pulih = ((sejak?: number) => {
    if (sudahBerakhir) return Promise.resolve<HasilPulih>("berakhir");
    if (berjalan) return berjalan;
    const proses = (async () => {
      try {
        const hasil = await (deps.kunci
          ? deps.kunci(() => tukar(sejak))
          : tukar(sejak));
        if (hasil === "berakhir" && !sudahBerakhir) {
          sudahBerakhir = true;
          deps.saatBerakhir?.();
        }
        return hasil;
      } finally {
        berjalan = null;
      }
    })();
    berjalan = proses;
    return proses;
  }) as Pemulih;
  pulih.berakhir = () => sudahBerakhir;
  return pulih;
}

/** Membaca `message` dari badan galat tanpa mengonsumsi respons asli. */
export async function pesanGalat(res: Response): Promise<string | undefined> {
  try {
    const payload = (await res.clone().json()) as { message?: unknown };
    return typeof payload.message === "string" ? payload.message : undefined;
  } catch {
    return undefined;
  }
}

type FetchFn = (input: string, init?: RequestInit) => Promise<Response>;

/**
 * fetch dengan pemulihan sesi: 401 yang layak → `pulih` → ulangi sekali. `init` boleh fungsi supaya header
 * dinamis (token CSRF) dihitung ulang pada pengulangan, karena refresh dapat menerbitkan cookie CSRF baru.
 */
export async function fetchDenganPemulihan(
  fetchFn: FetchFn,
  pulih: (sejak?: number) => Promise<HasilPulih>,
  input: string,
  init: RequestInit | (() => RequestInit) = {},
): Promise<Response> {
  const buat = () => (typeof init === "function" ? init() : init);
  const mulai = Date.now();
  const res = await fetchFn(input, buat());
  if (res.status !== 401) return res;
  if (!perluPulih(input, res.status, await pesanGalat(res))) return res;

  const hasil = await pulih(mulai);
  if (hasil !== "ok") return res;
  return fetchFn(input, buat());
}

const HALAMAN_AUTH = ["/masuk", "/lupa-password", "/reset-password"];

/**
 * Tujuan setelah sesi berakhir: halaman masuk membawa halaman semula di `lanjut`. Null bila sudah di halaman
 * auth publik (tidak perlu dialihkan).
 */
export function tujuanSetelahBerakhir(
  pathname: string,
  search = "",
): string | null {
  if (HALAMAN_AUTH.some((p) => pathname === p || pathname.startsWith(`${p}/`)))
    return null;
  return `/masuk?lanjut=${encodeURIComponent(pathname + search)}`;
}
