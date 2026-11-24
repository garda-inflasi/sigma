"use client";

import { create } from "zustand";
import { persist } from "zustand/middleware";

export type LingkunganData = "staging" | "prod";

export function defaultLingkunganData(): LingkunganData {
  return process.env.NEXT_PUBLIC_DEFAULT_DATA_ENV === "prod"
    ? "prod"
    : "staging";
}

export interface InfoSiklusMl {
  tersedia: boolean;
  cycleId?: string;
  cycleTimestamp?: string;
  dataUtama?: string;
  alasan?: string;
}

interface UiState {
  sidebarCollapsed: boolean;
  toggleSidebar: () => void;
  lingkunganData: LingkunganData;
  setLingkunganData: (env: LingkunganData) => void;
  mlInfo: InfoSiklusMl | null;
  setMlInfo: (info: InfoSiklusMl | null) => void;
  commandOpen: boolean;
  setCommandOpen: (open: boolean | ((prev: boolean) => boolean)) => void;
  mobileNavOpen: boolean;
  setMobileNavOpen: (open: boolean) => void;
}

let lingkunganAktifCache: LingkunganData = defaultLingkunganData();

/** Membaca sumber data aktif (`staging` | `prod`) di luar komponen React (klien HTTP & query key). */
export function getLingkunganData(): LingkunganData {
  return lingkunganAktifCache;
}

export const useUi = create<UiState>()(
  persist(
    (set) => ({
      sidebarCollapsed: false,
      toggleSidebar: () => set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),
      lingkunganData: defaultLingkunganData(),
      setLingkunganData: (lingkunganData) => {
        lingkunganAktifCache = lingkunganData;
        set({ lingkunganData });
      },
      mlInfo: null,
      setMlInfo: (mlInfo) => set({ mlInfo }),
      commandOpen: false,
      setCommandOpen: (open) => set((s) => ({ commandOpen: typeof open === "function" ? open(s.commandOpen) : open })),
      mobileNavOpen: false,
      setMobileNavOpen: (mobileNavOpen) => set({ mobileNavOpen }),
    }),
    {
      name: "garda-ui",
      partialize: (s) => ({
        sidebarCollapsed: s.sidebarCollapsed,
        lingkunganData: s.lingkunganData,
      }),
      onRehydrateStorage: () => (state) => {
        if (state?.lingkunganData) {
          lingkunganAktifCache = state.lingkunganData;
        }
      },
      skipHydration: true,
    },
  ),
);

useUi.subscribe((state) => {
  lingkunganAktifCache = state.lingkunganData;
});
