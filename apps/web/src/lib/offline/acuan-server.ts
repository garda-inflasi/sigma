"use client";

import { del, get, set } from "idb-keyval";
import { create } from "zustand";
import { API_BASE, MODE_MOCK } from "@/lib/api/client";
import type { DataAcuanResponse } from "@/lib/api/lapangan";
import { KUNCI_ACUAN } from "./kunci";
import { penulisanIdbTertutup } from "./penutup";

const KUNCI_CACHE = KUNCI_ACUAN;

interface CacheAcuan {
  etag: string;
  data: DataAcuanResponse;
}

/** Data acuan terunduh yang dipakai form input untuk validasi di perangkat. */
export const useAcuanServer = create<{ data?: DataAcuanResponse }>(() => ({}));

const adaIdb = () => typeof indexedDB !== "undefined";

/** Menghapus data acuan lokal, mis. saat ganti akun atau keluar. */
export async function hapusAcuanLokal() {
  useAcuanServer.setState({ data: undefined });
  if (adaIdb()) await del(KUNCI_CACHE);
}

/**
 * Mengunduh data acuan dari /v1/lapangan/acuan. Cache lokal dimuat lebih dulu (berguna offline);
 * permintaan memakai If-None-Match sehingga server menjawab 304 tanpa isi bila tidak ada yang baru.
 * Mengembalikan true bila data baru tersimpan. Galat jaringan tidak dilempar: cache lama tetap dipakai.
 */
export async function unduhAcuan(
  ambil: typeof fetch = fetch,
): Promise<boolean> {
  const cache = adaIdb() ? await get<CacheAcuan>(KUNCI_CACHE) : undefined;
  if (cache && !useAcuanServer.getState().data)
    useAcuanServer.setState({ data: cache.data });
  if (MODE_MOCK) return false;

  try {
    const res = await ambil(`${API_BASE}/v1/lapangan/acuan`, {
      credentials: "include",
      headers: {
        Accept: "application/json",
        ...(cache ? { "If-None-Match": cache.etag } : {}),
      },
    });
    if (res.status === 304 || !res.ok) return false;

    const etag = res.headers.get("ETag");
    const body = (await res.json()) as { data?: DataAcuanResponse };
    if (!body.data || !etag) return false;
    useAcuanServer.setState({ data: body.data });
    if (adaIdb() && !penulisanIdbTertutup())
      await set(KUNCI_CACHE, { etag, data: body.data } satisfies CacheAcuan);
    return true;
  } catch {
    return false;
  }
}
