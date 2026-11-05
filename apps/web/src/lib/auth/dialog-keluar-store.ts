"use client";

import { create } from "zustand";

interface DialogKeluarState {
  terbuka: boolean;
  /** Entri belum terkirim yang akan hilang bila pengguna tetap keluar. */
  sisa: number;
  buka: (sisa: number) => void;
  setSisa: (sisa: number) => void;
  tutup: () => void;
}

/** Status dialog konfirmasi keluar; dibuka oleh `keluarDariSesi()` dan dirender sekali di Providers. */
export const useDialogKeluar = create<DialogKeluarState>((set) => ({
  terbuka: false,
  sisa: 0,
  buka: (sisa) => set({ terbuka: true, sisa }),
  setSisa: (sisa) => set({ sisa }),
  tutup: () => set({ terbuka: false }),
}));
