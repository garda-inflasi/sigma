"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useEffect, useRef } from "react";
import { API_BASE, MODE_MOCK } from "@/lib/api/client";
import { jalankanAliranEws } from "@/lib/api/ews-sambung";
import { layakDiumumkan, mergeAlert, parseAlertEvent } from "@/lib/api/ews-stream";
import { randomEws } from "@/lib/api/mock";
import { pulihkanSesi } from "@/lib/auth/sesi-client";
import { useSession } from "@/lib/auth/session-context";
import type { EwsAlert } from "@/lib/data/types";
import { severityMeta } from "@/lib/design/tokens";
import { dapatDitindaklanjutiEws, terapkanPeristiwaKritis } from "@/lib/ews/kritis";
import { useAksiEws } from "@/lib/query/hooks";
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
  const sesi = useSession();
  const bolehLihat = can(sesi, "ews.lihat");
  const bolehAkui = can(sesi, "ews.akui");
  const catat = useEwsStore((s) => s.catat);
  const setStatus = useEwsStore((s) => s.setStatus);
  const ack = useAksiEws();
  const ackRef = useRef(ack.mutate);
  ackRef.current = ack.mutate;

  useEffect(() => {
    if (!bolehLihat) return;

    const handle = (a: EwsAlert) => {
      const diumumkan = layakDiumumkan(qc.getQueryData<EwsAlert[]>(qk.ews.list()), a);
      qc.setQueryData<EwsAlert[]>(qk.ews.list(), (old) => mergeAlert(old, a));
      // Sumber toast dan banner KRITIS: masuk bila BARU, keluar begitu server menyatakan sudah ditindaklanjuti.
      qc.setQueryData<EwsAlert[]>(qk.ews.kritisBaru(), (old) => terapkanPeristiwaKritis(old, a));
      if (!diumumkan) return;
      catat(a.id);
      // Toast KRITIS dirender PengelolaToastKritis dari data di atas supaya bertahan sampai ditindaklanjuti.
      if (a.severity !== "KRITIS" && severityMeta[a.severity].rank >= 2) {
        const siapTindak = dapatDitindaklanjutiEws(a, MODE_MOCK);
        notifyEws(a, {
          onView: () => router.push(siapTindak ? `/ews?id=${a.id}` : `/ews?severity=${encodeURIComponent(a.severity)}`),
          onAck: bolehAkui && siapTindak ? () => ackRef.current({ aksi: "akui", ids: [a.id], label: "Alert diakui" }) : undefined,
        });
      }
    };

    const onSimulasi = () => handle({ ...randomEws(), severity: "KRITIS", nilai: 21.4, judul: "Lonjakan harga cabai merah keriting", komoditasKode: "CBM-KRT", komoditasNama: "Cabai merah keriting", wilayah: "Kota Banda Aceh", kabupatenKode: "1171", jenis: "LONJAKAN_HARGA" });
    if (MODE_MOCK) window.addEventListener("garda:simulasi-ews", onSimulasi);

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

    // Cookie sesi ikut terkirim (same-origin). EventSource menyambung ulang sendiri dengan Last-Event-ID
    // untuk putus jaringan; bila server menolak (401 karena access token habis) koneksi mati permanen, jadi
    // penyambung memulihkan sesi (refresh) lalu membuat EventSource baru dan menyegarkan daftar alert.
    return jalankanAliranEws({
      buat: () => new EventSource(`${API_BASE}/v1/ews/stream`, { withCredentials: true }),
      pulih: (sejak) => pulihkanSesi(sejak),
      saatStatus: setStatus,
      saatAlert: (data) => {
        const alert = parseAlertEvent(data);
        if (alert) handle(alert);
      },
      // Event yang terlewat selama terputus: segarkan daftar dan KRITIS yang menahan toast serta banner.
      saatTersambungUlang: () => {
        qc.invalidateQueries({ queryKey: qk.ews.list() });
        qc.invalidateQueries({ queryKey: qk.ews.kritisBaru() });
      },
    });
  }, [bolehLihat, bolehAkui, qc, router, catat, setStatus]);
}

export function EwsStreamMount() {
  useEwsStream();
  return null;
}
