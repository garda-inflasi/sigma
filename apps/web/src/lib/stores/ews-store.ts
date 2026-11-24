"use client";

import { create } from "zustand";

export type StreamStatus = "menghubungkan" | "terhubung" | "terputus" | "simulasi";

interface EwsState {
  unread: number;
  /** ID alert yang baru masuk — dipakai untuk kilau sorot di feed */
  baru: string[];
  status: StreamStatus;
  /** ID alert KRITIS yang toast-nya dikecilkan ke banner (per pengguna, bertahan selama tab hidup). */
  dikecilkan: string[];
  catat: (id: string) => void;
  bacaSemua: () => void;
  setStatus: (s: StreamStatus) => void;
  muatDikecilkan: (penggunaId: string) => void;
  kecilkan: (penggunaId: string, id: string) => void;
  /** Membuang ID yang sudah tidak berstatus BARU: bila alert itu kembali, toast-nya muncul lagi. */
  pangkasDikecilkan: (penggunaId: string, idAktif: readonly string[]) => void;
}

const kunciDikecilkan = (penggunaId: string) => `garda:ews-kritis-dikecilkan:${penggunaId}`;

/** Penyimpanan bisa tidak tersedia (mode privat): toast tetap berfungsi, hanya muncul lagi setelah muat ulang. */
function bacaDikecilkan(penggunaId: string): string[] {
  try {
    const nilai: unknown = JSON.parse(sessionStorage.getItem(kunciDikecilkan(penggunaId)) ?? "[]");
    return Array.isArray(nilai) ? nilai.filter((v): v is string => typeof v === "string") : [];
  } catch {
    return [];
  }
}

function tulisDikecilkan(penggunaId: string, ids: string[]) {
  try {
    sessionStorage.setItem(kunciDikecilkan(penggunaId), JSON.stringify(ids));
  } catch {
    // Lihat bacaDikecilkan.
  }
}

export const useEwsStore = create<EwsState>()((set) => ({
  unread: 0,
  baru: [],
  status: "menghubungkan",
  dikecilkan: [],
  catat: (id) => {
    set((s) => ({ unread: s.unread + 1, baru: [id, ...s.baru].slice(0, 10) }));
    setTimeout(() => set((s) => ({ baru: s.baru.filter((x) => x !== id) })), 6000);
  },
  bacaSemua: () => set({ unread: 0 }),
  setStatus: (status) => set({ status }),
  muatDikecilkan: (penggunaId) => set({ dikecilkan: bacaDikecilkan(penggunaId) }),
  kecilkan: (penggunaId, id) =>
    set((s) => {
      if (s.dikecilkan.includes(id)) return s;
      const dikecilkan = [...s.dikecilkan, id];
      tulisDikecilkan(penggunaId, dikecilkan);
      return { dikecilkan };
    }),
  pangkasDikecilkan: (penggunaId, idAktif) =>
    set((s) => {
      const dikecilkan = s.dikecilkan.filter((id) => idAktif.includes(id));
      if (dikecilkan.length === s.dikecilkan.length) return s;
      tulisDikecilkan(penggunaId, dikecilkan);
      return { dikecilkan };
    }),
}));
