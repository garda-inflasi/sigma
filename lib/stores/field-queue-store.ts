"use client";

import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";
import type { Kualitas, TipeHarga } from "@/lib/data/types";
import { idbStorage } from "@/lib/offline/idb-storage";

export type StatusLokal = "DRAF" | "ANTRE" | "MENYINKRON" | "TERSINKRON" | "KONFLIK" | "GAGAL";

export interface EntriLokal {
  localId: string; // juga dipakai sebagai Idempotency-Key
  komoditasKode: string;
  pasarId: string;
  tipe: TipeHarga;
  harga: number;
  satuan: string;
  kualitas: Kualitas;
  kodePedagang?: string;
  foto?: string; // data URL terkompresi
  lat?: number;
  lng?: number;
  akurasiM?: number;
  dalamGeofence?: boolean;
  catatan?: string;
  dicatatPada: string;
  status: StatusLokal;
  percobaan: number;
  galat?: string;
  versiServer?: { harga: number; dicatatPada: string; petugas: string };
}

interface QueueState {
  items: EntriLokal[];
  syncing: boolean;
  lastSyncAt?: string;
  hydrated: boolean;
  tambah: (e: EntriLokal) => void;
  perbarui: (id: string, patch: Partial<EntriLokal>) => void;
  hapus: (ids: string[]) => void;
  antrekan: (ids: string[]) => void;
  setSyncing: (v: boolean) => void;
  tandaiSync: () => void;
}

export const useFieldQueue = create<QueueState>()(
  persist(
    (set) => ({
      items: [],
      syncing: false,
      hydrated: false,
      tambah: (e) => set((s) => ({ items: [e, ...s.items] })),
      perbarui: (id, patch) => set((s) => ({ items: s.items.map((i) => (i.localId === id ? { ...i, ...patch } : i)) })),
      hapus: (ids) => set((s) => ({ items: s.items.filter((i) => !ids.includes(i.localId)) })),
      antrekan: (ids) =>
        set((s) => ({
          items: s.items.map((i) => (ids.includes(i.localId) && i.status !== "TERSINKRON" ? { ...i, status: "ANTRE", galat: undefined } : i)),
        })),
      setSyncing: (syncing) => set({ syncing }),
      tandaiSync: () => set({ lastSyncAt: new Date().toISOString() }),
    }),
    {
      name: "garda-antrean-lapangan",
      storage: createJSONStorage(() => idbStorage),
      partialize: (s) => ({ items: s.items, lastSyncAt: s.lastSyncAt }),
      skipHydration: true,
      onRehydrateStorage: () => () => useFieldQueue.setState({ hydrated: true }),
    },
  ),
);

export const pendingCount = (items: EntriLokal[]) =>
  items.filter((i) => i.status === "ANTRE" || i.status === "GAGAL" || i.status === "MENYINKRON").length;
