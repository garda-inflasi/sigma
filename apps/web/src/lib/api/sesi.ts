/**
 * Sesi aktif milik sendiri ("Keamanan akun"): pemetaan respons API (snake_case) ke model UI, pengurutan,
 * dan klien yang menembus BFF. Paksa keluar oleh admin ada di `penggunaApi.paksaKeluar` (lib/api/pengguna.ts).
 */

import type { SesiDicabutWire, SesiWire } from "@/lib/bff/api-client";
import { bffFetch } from "./client";

export interface Sesi {
  id: string;
  /** Ringkasan perangkat dari server, mis. "Chrome di macOS". */
  perangkat: string;
  userAgent: string;
  ip: string;
  /** Awal sesi (ISO 8601). */
  dibuatPada: string;
  /** Terakhir sesi menukar token (ISO 8601); selaras dengan umur access token. */
  terakhirDipakai: string;
  kedaluwarsaPada: string;
  /** Sesi tempat halaman ini dibuka. */
  saatIni: boolean;
}

const PERANGKAT_BAWAAN = "Perangkat tidak dikenal";

export function toSesi(w: SesiWire): Sesi {
  return {
    id: w.id,
    perangkat: w.perangkat?.trim() || PERANGKAT_BAWAAN,
    userAgent: w.user_agent ?? "",
    ip: w.ip ?? "",
    dibuatPada: w.dibuat_pada,
    terakhirDipakai: w.terakhir_dipakai_pada,
    kedaluwarsaPada: w.kedaluwarsa_pada,
    saatIni: w.saat_ini === true,
  };
}

const waktu = (iso: string) => {
  const t = new Date(iso).getTime();
  return Number.isNaN(t) ? 0 : t;
};

/** Sesi ini di paling atas, lalu yang terakhir dipakai paling awal. Tidak mengubah masukan. */
export function urutkanSesi(items: readonly Sesi[]): Sesi[] {
  return [...items].sort(
    (a, b) =>
      Number(b.saatIni) - Number(a.saatIni) ||
      waktu(b.terakhirDipakai) - waktu(a.terakhirDipakai),
  );
}

/** Jumlah sesi selain yang sedang dipakai. */
export function jumlahSesiLain(items: readonly Sesi[]): number {
  return items.filter((s) => !s.saatIni).length;
}

/** Baris rincian satu sesi: alamat IP bila diketahui. */
export function rincianSesi(s: Sesi): string {
  return s.ip ? `IP ${s.ip}` : "IP tidak diketahui";
}

/** Klien sesi aktif — semua panggilan melalui Route Handler BFF. */
export const keamananApi = {
  daftar: async (): Promise<Sesi[]> =>
    urutkanSesi((await bffFetch<SesiWire[]>("/api/auth/sessions")).map(toSesi)),

  /** Cabut satu sesi lain; sesi yang sedang dipakai ditolak server (400). */
  cabut: (id: string) =>
    bffFetch<{ id: string }>(`/api/auth/sessions/${encodeURIComponent(id)}`, {
      method: "DELETE",
    }),

  /** Keluar dari semua perangkat lain; sesi ini tetap masuk. */
  cabutLain: () =>
    bffFetch<SesiDicabutWire>("/api/auth/sessions/revoke-others", {
      method: "POST",
    }),

  /** Keluar dari semua perangkat termasuk yang ini; cookie dihapus server. */
  keluarSemua: () =>
    bffFetch<SesiDicabutWire>("/api/auth/logout-all", { method: "POST" }),
};
