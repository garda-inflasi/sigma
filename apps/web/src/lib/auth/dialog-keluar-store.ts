"use client";

import { create } from "zustand";

/** "sesi": keluar dari perangkat ini; "semua": keluar dari semua perangkat. */
export type CakupanKeluar = "sesi" | "semua";

interface DialogKeluarState {
  terbuka: boolean;
  /** Entri belum terkirim yang akan hilang bila pengguna tetap keluar. */
  sisa: number;
  cakupan: CakupanKeluar;
  buka: (sisa: number, cakupan?: CakupanKeluar) => void;
  setSisa: (sisa: number) => void;
  tutup: () => void;
}

/** Status dialog konfirmasi keluar; dibuka oleh `keluarDariSesi()` dan `keluarDariSemuaPerangkat()` dan dirender sekali di Providers. */
export const useDialogKeluar = create<DialogKeluarState>((set) => ({
  terbuka: false,
  sisa: 0,
  cakupan: "sesi",
  buka: (sisa, cakupan = "sesi") => set({ terbuka: true, sisa, cakupan }),
  setSisa: (sisa) => set({ sisa }),
  tutup: () => set({ terbuka: false }),
}));
