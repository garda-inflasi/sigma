"use client";

import { MODE_MOCK, bffFetch } from "@/lib/api/client";
import { keluar } from "./logout";

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
