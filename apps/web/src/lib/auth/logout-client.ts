"use client";

import { MODE_MOCK, bffFetch } from "@/lib/api/client";
import { bacaBelumTerkirim, bersihkanSaatKeluar } from "@/lib/offline/bersihkan-klien";
import { useDialogKeluar } from "./dialog-keluar-store";
import { tentukanAksiKeluar } from "./konfirmasi-keluar";
import { keluar } from "./logout";

/**
 * Keluar tanpa bertanya: cabut sesi lewat BFF (mode mock tidak punya sesi server), bersihkan data offline
 * perangkat (IndexedDB, cache halaman berpengguna), lalu muat ulang /masuk.
 */
export function keluarSekarang(): Promise<void> {
  return keluar({
    panggilLogout: () =>
      MODE_MOCK
        ? Promise.resolve()
        : bffFetch("/api/auth/logout", { method: "POST" }),
    bersihkanLokal: bersihkanSaatKeluar,
    pindah: (url) => window.location.assign(url),
  });
}

/**
 * Tindakan menu "Keluar". Bila masih ada entri lapangan yang belum terkirim, dialog konfirmasi dibuka
 * (components/auth/dialog-keluar.tsx) alih-alih langsung keluar, karena entri itu hilang bersama data perangkat.
 */
export async function keluarDariSesi(): Promise<void> {
  const aksi = await tentukanAksiKeluar(bacaBelumTerkirim);
  if (aksi.jenis === "konfirmasi") {
    useDialogKeluar.getState().buka(aksi.sisa);
    return;
  }
  await keluarSekarang();
}
