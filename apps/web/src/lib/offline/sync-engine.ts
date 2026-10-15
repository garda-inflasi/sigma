"use client";

import { useEffect } from "react";
import { ApiError, apiFetch, MODE_MOCK } from "@/lib/api/client";
import { sleep } from "@/lib/utils";
import { useFieldQueue, type EntriLokal } from "@/lib/stores/field-queue-store";
import { useOnline } from "./use-online";

type Hasil = { ok: true } | { konflik: true; versiServer: NonNullable<EntriLokal["versiServer"]> };

async function kirim(e: EntriLokal): Promise<Hasil> {
  if (MODE_MOCK) {
    await sleep(450 + Math.random() * 600);
    // ±1 dari 9 entri disimulasikan bentrok dengan entri petugas lain di pasar yang sama
    if (parseInt(e.localId.slice(-2), 16) % 9 === 0) {
      return { konflik: true, versiServer: { harga: Math.round((e.harga * 1.06) / 100) * 100, dicatatPada: new Date(Date.now() - 35 * 60_000).toISOString(), petugas: "Muhammad Ikhsan" } };
    }
    return { ok: true };
  }
  try {
    const { foto, ...payload } = e;
    await apiFetch("/v1/lapangan/harga", { method: "POST", body: JSON.stringify({ ...payload, foto }), idempotencyKey: e.localId });
    return { ok: true };
  } catch (err) {
    if (err instanceof ApiError && err.status === 409) {
      return { konflik: true, versiServer: (err.payload as { versiServer: NonNullable<EntriLokal["versiServer"]> }).versiServer };
    }
    throw err;
  }
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
    const run = async () => {
      const st = useFieldQueue.getState();
      if (st.syncing || !navigator.onLine) return;
      const antre = st.items.filter((i) => i.status === "ANTRE" || (i.status === "GAGAL" && i.percobaan < 5));
      if (!antre.length) return;
      st.setSyncing(true);
      for (const e of antre) {
        st.perbarui(e.localId, { status: "MENYINKRON" });
        try {
          const h = await kirim(e);
          if ("konflik" in h) st.perbarui(e.localId, { status: "KONFLIK", versiServer: h.versiServer });
          else st.perbarui(e.localId, { status: "TERSINKRON", galat: undefined });
        } catch (err) {
          st.perbarui(e.localId, { status: "GAGAL", percobaan: e.percobaan + 1, galat: err instanceof Error ? err.message : "Gagal mengirim" });
        }
      }
      st.setSyncing(false);
      st.tandaiSync();
    };
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
