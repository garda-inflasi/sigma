"use client";

import {
  kirimLaluPeriksa,
  type HasilKirimDulu,
} from "@/lib/auth/konfirmasi-keluar";
import { useFieldQueue } from "@/lib/stores/field-queue-store";
import { hitungBelumTerkirimDariItems } from "./bersihkan";
import { pemilikSaatIni } from "./bersihkan-klien";
import { sinkronkanAntrean } from "./sync-engine";

/** Batas tunggu sinkronisasi yang sedang berjalan; sesudahnya dilanjutkan agar dialog tidak menggantung. */
const BATAS_TUNGGU_MS = 30_000;

/** Menunggu sampai tidak ada putaran sinkronisasi yang berjalan (atau batas waktu habis). */
function tungguTidakSinkron(batasMs = BATAS_TUNGGU_MS): Promise<void> {
  return new Promise((selesai) => {
    if (!useFieldQueue.getState().syncing) return selesai();
    let berhenti: () => void = () => undefined;
    const timer = setTimeout(() => {
      berhenti();
      selesai();
    }, batasMs);
    berhenti = useFieldQueue.subscribe((s) => {
      if (s.syncing) return;
      clearTimeout(timer);
      berhenti();
      selesai();
    });
  });
}

/**
 * "Kirim dulu" pada dialog keluar: pulihkan antrean bila belum (halaman konsol), sinkronkan sekarang, lalu
 * hitung entri yang masih tertinggal. Antrean di memori adalah sumber kebenaran sesudah dipulihkan.
 */
export function kirimSebelumKeluar(): Promise<HasilKirimDulu> {
  return kirimLaluPeriksa({
    online: () => navigator.onLine,
    siapkan: async () => {
      if (useFieldQueue.getState().hydrated) return;
      // Antrean milik pengguna lain tidak boleh ikut terkirim atas nama sesi ini.
      if (!((await pemilikSaatIni())?.aman ?? true)) return;
      await useFieldQueue.persist.rehydrate();
    },
    kirim: async () => {
      await tungguTidakSinkron();
      await sinkronkanAntrean();
      await tungguTidakSinkron();
    },
    hitungSisa: () =>
      hitungBelumTerkirimDariItems(useFieldQueue.getState().items),
  });
}
