"use client";

import { MODE_MOCK, bffFetch } from "@/lib/api/client";
import { bacaBelumTerkirim, bersihkanSaatKeluar } from "@/lib/offline/bersihkan-klien";
import { useDialogKeluar } from "./dialog-keluar-store";
import { tentukanAksiKeluar } from "./konfirmasi-keluar";
import { keluar, LOGIN_URL } from "./logout";

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
    useDialogKeluar.getState().buka(aksi.sisa, "sesi");
    return;
  }
  await keluarSekarang();
}

/**
 * Keluar dari semua perangkat tanpa bertanya: server mengakhiri semua sesi dan menghapus cookie, lalu data
 * offline perangkat ini dibersihkan. Galat server dilempar ke pemanggil sebelum apa pun dihapus, supaya
 * pengguna tahu perangkat lain belum dikeluarkan.
 */
export async function keluarSemuaSekarang(): Promise<void> {
  await bffFetch("/api/auth/logout-all", { method: "POST" });
  await bersihkanSaatKeluar();
  window.location.assign(LOGIN_URL);
}

/**
 * "Keluar dari semua perangkat". Mengembalikan false bila keputusan diserahkan ke dialog entri belum
 * terkirim (pemanggil mengembalikan tampilannya); galat server dilempar ke pemanggil.
 */
export async function keluarDariSemuaPerangkat(): Promise<boolean> {
  const aksi = await tentukanAksiKeluar(bacaBelumTerkirim);
  if (aksi.jenis === "konfirmasi") {
    useDialogKeluar.getState().buka(aksi.sisa, "semua");
    return false;
  }
  await keluarSemuaSekarang();
  return true;
}
