"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useEffect, useRef } from "react";
import { API_BASE, MODE_MOCK } from "@/lib/api/client";
import { randomEws } from "@/lib/api/mock";
import { useSession } from "@/lib/auth/session-context";
import type { EwsAlert } from "@/lib/data/types";
import { severityMeta } from "@/lib/design/tokens";
import { useBulkAction } from "@/lib/query/hooks";
import { qk } from "@/lib/query/keys";
import { can } from "@/lib/rbac/permissions";
import { useEwsStore } from "@/lib/stores/ews-store";
import { notifyEws } from "./ews-toast";

/**
 * Langganan EWS real-time.
 * Produksi: SSE dari layanan Go (`/v1/ews/stream`, di belakangnya Redis pub/sub).
 * Mock: alert acak tiap 25–45 detik + event "garda:simulasi-ews" dari palet perintah.
 */
export function useEwsStream() {
  const qc = useQueryClient();
  const router = useRouter();
  const { role } = useSession();
  const catat = useEwsStore((s) => s.catat);
  const setStatus = useEwsStore((s) => s.setStatus);
  const ack = useBulkAction<EwsAlert>({ sumber: "ews", queryKey: qk.ews.list(), patch: (r) => ({ ...r, status: "DIAKUI" }) });
  const ackRef = useRef(ack.mutate);
  ackRef.current = ack.mutate;

  useEffect(() => {
    if (!can(role, "ews.lihat")) return;

    const handle = (a: EwsAlert) => {
      qc.setQueryData<EwsAlert[]>(qk.ews.list(), (old) => (old ? [a, ...old] : old));
      catat(a.id);
      if (severityMeta[a.severity].rank >= 2) {
        notifyEws(a, {
          onView: () => router.push(`/ews?id=${a.id}`),
          onAck: can(role, "ews.akui") ? () => ackRef.current({ aksi: "akui", ids: [a.id], label: "Alert diakui" }) : undefined,
        });
      }
    };

    const onSimulasi = () => handle({ ...randomEws(), severity: "KRITIS", nilai: 21.4, judul: "Lonjakan harga cabai merah keriting", komoditasKode: "CBM-KRT", komoditasNama: "Cabai merah keriting", wilayah: "Kota Banda Aceh", kabupatenKode: "1171", jenis: "LONJAKAN_HARGA" });
    window.addEventListener("garda:simulasi-ews", onSimulasi);

    if (MODE_MOCK) {
      setStatus("simulasi");
      let timer: ReturnType<typeof setTimeout>;
      const loop = () => {
        timer = setTimeout(() => {
          handle(randomEws());
          loop();
        }, 25_000 + Math.random() * 20_000);
      };
      loop();
      return () => {
        clearTimeout(timer);
        window.removeEventListener("garda:simulasi-ews", onSimulasi);
      };
    }

    setStatus("menghubungkan");
    const es = new EventSource(`${API_BASE}/v1/ews/stream`, { withCredentials: true });
    es.onopen = () => setStatus("terhubung");
    es.onerror = () => setStatus("terputus"); // EventSource menyambung ulang otomatis
    es.addEventListener("alert", (e) => handle(JSON.parse((e as MessageEvent<string>).data) as EwsAlert));
    return () => {
      es.close();
      window.removeEventListener("garda:simulasi-ews", onSimulasi);
    };
  }, [role, qc, router, catat, setStatus]);
}

export function EwsStreamMount() {
  useEwsStream();
  return null;
}
