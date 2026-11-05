"use client";

import { MODE_MOCK, bffFetch } from "@/lib/api/client";
import { keluar, LOGIN_URL } from "./logout";

/** Tindakan menu "Keluar": cabut sesi lewat BFF lalu muat ulang /masuk (mode mock tidak punya sesi server). */
export function keluarDariSesi(): Promise<void> {
  return keluar({
    panggilLogout: () =>
      MODE_MOCK
        ? Promise.resolve()
        : bffFetch("/api/auth/logout", { method: "POST" }),
    pindah: (url) => window.location.assign(url),
  });
}

/** "Keluar dari semua perangkat": server mengakhiri semua sesi dan menghapus cookie; galat dilempar ke pemanggil. */
export async function keluarDariSemuaPerangkat(): Promise<void> {
  await bffFetch("/api/auth/logout-all", { method: "POST" });
  window.location.assign(LOGIN_URL);
}
