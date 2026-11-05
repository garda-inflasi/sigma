/**
 * Penyambung browser untuk pemulihan sesi senyap (logika di sesi-pulih.ts): fetch ke BFF, kunci antar-tab
 * (Web Locks), catatan refresh terakhir di localStorage, dan pengalihan ke /masuk saat sesi benar-benar berakhir.
 * Aman diimpor di server (tidak menyentuh global browser saat dimuat); `fetchSesi` dipakai klien API.
 */
import {
  buatPemulih,
  fetchDenganPemulihan,
  tujuanSetelahBerakhir,
  type CatatanSegar,
  type HasilPulih,
} from "./sesi-pulih";

const KUNCI_LOCK = "garda-sesi-pulih";
const KUNCI_CATATAN = "garda:sesi:segar";

function bacaCatatan(): CatatanSegar | null {
  try {
    const mentah = window.localStorage.getItem(KUNCI_CATATAN);
    if (!mentah) return null;
    const nilai = JSON.parse(mentah) as { t?: unknown };
    return typeof nilai.t === "number" ? { t: nilai.t } : null;
  } catch {
    return null;
  }
}

function tulisCatatan(catatan: CatatanSegar): void {
  try {
    window.localStorage.setItem(KUNCI_CATATAN, JSON.stringify(catatan));
  } catch {
    // Mode privat/penyimpanan penuh: tanpa catatan tab tetap benar, hanya bisa memutar token dua kali.
  }
}

async function denganKunci<T>(jalankan: () => Promise<T>): Promise<T> {
  const locks = typeof navigator === "undefined" ? undefined : navigator.locks;
  return locks ? locks.request(KUNCI_LOCK, jalankan) : jalankan();
}

function keHalamanMasuk(): void {
  const tujuan = tujuanSetelahBerakhir(
    window.location.pathname,
    window.location.search,
  );
  if (tujuan) window.location.assign(tujuan);
}

let pemulih: ReturnType<typeof buatPemulih> | null = null;

/** Pemulih tunggal per tab: `POST /api/auth/refresh` bersama untuk semua permintaan yang 401 bersamaan. */
export function pulihkanSesi(sejak?: number): Promise<HasilPulih> {
  pemulih ??= buatPemulih({
    refresh: () =>
      fetch("/api/auth/refresh", {
        method: "POST",
        credentials: "same-origin",
        headers: { Accept: "application/json" },
      }),
    kunci: denganKunci,
    baca: bacaCatatan,
    tulis: tulisCatatan,
    saatBerakhir: keHalamanMasuk,
  });
  return pemulih(sejak);
}

/** fetch dengan pemulihan sesi senyap: 401 karena token kedaluwarsa → refresh → ulangi sekali. */
export function fetchSesi(
  input: string,
  init: RequestInit | (() => RequestInit) = {},
): Promise<Response> {
  return fetchDenganPemulihan(
    (url, opsi) => fetch(url, opsi),
    pulihkanSesi,
    input,
    init,
  );
}
