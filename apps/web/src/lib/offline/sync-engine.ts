"use client";

import { useEffect } from "react";
import { useFieldQueue } from "@/lib/stores/field-queue-store";
import { kirimEntri } from "./kirim-entri";
import { useOnline } from "./use-online";

/**
 * Satu putaran sinkronisasi: mengirim entri ANTRE dan GAGAL (< 5 percobaan) berurutan. Dilewati bila putaran lain
 * sedang berjalan atau perangkat offline. Dipakai hook di bawah dan tombol "Kirim dulu" saat keluar.
 */
export async function sinkronkanAntrean() {
  const st = useFieldQueue.getState();
  if (st.syncing || !navigator.onLine) return;
  const antre = st.items.filter((i) => i.status === "ANTRE" || (i.status === "GAGAL" && i.percobaan < 5));
  if (!antre.length) return;
  st.setSyncing(true);
  for (const e of antre) {
    // Entri yang dihapus selama putaran berjalan (mis. pengguna memilih keluar) tidak boleh terkirim.
    if (!useFieldQueue.getState().items.some((i) => i.localId === e.localId)) continue;
    st.perbarui(e.localId, { status: "MENYINKRON" });
    try {
      const h = await kirimEntri(e);
      if ("konflik" in h) st.perbarui(e.localId, { status: "KONFLIK", versiServer: h.versiServer });
      else st.perbarui(e.localId, { status: "TERSINKRON", galat: undefined });
    } catch (err) {
      st.perbarui(e.localId, { status: "GAGAL", percobaan: e.percobaan + 1, galat: err instanceof Error ? err.message : "Gagal mengirim" });
    }
  }
  st.setSyncing(false);
  st.tandaiSync();
}

/**
 * Mesin sinkronisasi offline-first:
 * - berjalan saat kembali online, tiap 60 detik, dan saat event "garda:sinkron"
 * - berurutan, idempoten (Idempotency-Key = localId), backoff hingga 5 percobaan
 * - 409 dari server → status KONFLIK untuk diselesaikan petugas
 */
export function useSyncEngine() {
  const online = useOnline();
  const hydrated = useFieldQueue((s) => s.hydrated);

  useEffect(() => {
    if (!hydrated) return;
    const run = () => sinkronkanAntrean();
    if (online) run();
    const onEvt = () => run();
    window.addEventListener("garda:sinkron", onEvt);
    navigator.serviceWorker?.addEventListener("message", (m) => m.data === "garda:sinkron" && run());
    const t = setInterval(run, 60_000);
    return () => {
      window.removeEventListener("garda:sinkron", onEvt);
      clearInterval(t);
    };
  }, [online, hydrated]);
}
