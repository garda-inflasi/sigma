"use client";

import { create } from "zustand";

export type StreamStatus = "menghubungkan" | "terhubung" | "terputus" | "simulasi";

interface EwsState {
  unread: number;
  /** ID alert yang baru masuk — dipakai untuk kilau sorot di feed */
  baru: string[];
  status: StreamStatus;
  catat: (id: string) => void;
  bacaSemua: () => void;
  setStatus: (s: StreamStatus) => void;
}

export const useEwsStore = create<EwsState>()((set) => ({
  unread: 0,
  baru: [],
  status: "menghubungkan",
  catat: (id) => {
    set((s) => ({ unread: s.unread + 1, baru: [id, ...s.baru].slice(0, 10) }));
    setTimeout(() => set((s) => ({ baru: s.baru.filter((x) => x !== id) })), 6000);
  },
  bacaSemua: () => set({ unread: 0 }),
  setStatus: (status) => set({ status }),
}));
