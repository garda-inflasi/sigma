"use client";

import { motion } from "motion/react";
import { Minimize2, X } from "lucide-react";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { PulseRing } from "@/components/motion/pulse-ring";
import { SpringButton } from "@/components/garda/spring-button";
import { StatusBadge } from "@/components/garda/status-badge";
import { useSession } from "@/lib/auth/session-context";
import type { EwsAlert } from "@/lib/data/types";
import { severityMeta, toneClass } from "@/lib/design/tokens";
import { pilihToastKritis } from "@/lib/ews/kritis";
import { spring } from "@/lib/motion/presets";
import { useAksiEws } from "@/lib/query/hooks";
import { useEwsStore } from "@/lib/stores/ews-store";
import { cn } from "@/lib/utils";
import { JenisIcon, formatNilaiEws } from "./ews-meta";

interface Handlers {
  onView?: () => void;
  onAck?: () => void;
}

/** Toast EWS di bawah KRITIS: hilang otomatis dengan bilah waktu tersisa dan bisa ditutup. */
export function notifyEws(alert: EwsAlert, handlers: Handlers = {}) {
  const duration = 10_000;
  toast.custom((id) => <EwsToastCard id={id} alert={alert} duration={duration} {...handlers} />, { id: alert.id, duration });
}

interface HandlersKritis {
  onView: () => void;
  /** Hanya untuk pemegang `ews.akui`. Toast tetap ada sampai server menyatakan statusnya berubah. */
  onAck?: () => Promise<unknown>;
  /** Hanya untuk yang tidak bisa mengakui: memindahkan toast ke banner global. */
  onKecilkan?: () => void;
}

const idToastKritis = (alertId: string) => `ews-kritis-${alertId}`;

/**
 * Toast KRITIS: menetap, tanpa tombol tutup dan tidak bisa digeser. Satu-satunya yang menghapusnya adalah
 * PengelolaToastKritis, yaitu saat data server menyatakan alert sudah ditindaklanjuti atau saat dikecilkan.
 */
function tampilkanToastKritis(alert: EwsAlert, handlers: HandlersKritis) {
  toast.custom(() => <KartuToastKritis alert={alert} {...handlers} />, {
    id: idToastKritis(alert.id),
    duration: Infinity,
    dismissible: false,
  });
}

/**
 * Menyelaraskan toast KRITIS dengan daftar KRITIS berstatus BARU dari server. Karena bersumber dari data, toast muncul
 * lagi setelah muat ulang dan hilang bila alert ditindaklanjuti dari tab atau pengguna lain. Jumlahnya dibatasi;
 * sisanya terwakili oleh banner global.
 */
export function PengelolaToastKritis({ alerts, bolehAkui }: { alerts: EwsAlert[]; bolehAkui: boolean }) {
  const router = useRouter();
  const penggunaId = useSession().id;
  const aksi = useAksiEws();
  const dikecilkan = useEwsStore((s) => s.dikecilkan);
  const [siap, setSiap] = useState(false);
  const tampil = useRef(new Set<string>());

  // Handler terbaru tanpa memicu ulang efek penyelarasan.
  const handlerRef = useRef<(a: EwsAlert) => HandlersKritis>(null!);
  handlerRef.current = (a) => ({
    onView: () => router.push(`/ews?id=${a.id}`),
    onAck: bolehAkui
      ? () => aksi.mutateAsync({ aksi: "akui", ids: [a.id], label: "Alert diakui" }).catch(() => undefined)
      : undefined,
    onKecilkan: bolehAkui ? undefined : () => useEwsStore.getState().kecilkan(penggunaId, a.id),
  });

  useEffect(() => {
    useEwsStore.getState().muatDikecilkan(penggunaId);
    setSiap(true);
  }, [penggunaId]);

  const kunciAlert = alerts.map((a) => a.id).join("|");
  useEffect(() => {
    if (!siap) return;
    useEwsStore.getState().pangkasDikecilkan(penggunaId, alerts.map((a) => a.id));
    const target = new Set(pilihToastKritis(alerts, dikecilkan));
    for (const id of tampil.current) {
      if (!target.has(id)) {
        toast.dismiss(idToastKritis(id));
        tampil.current.delete(id);
      }
    }
    for (const a of alerts) {
      if (target.has(a.id)) {
        tampilkanToastKritis(a, handlerRef.current(a));
        tampil.current.add(a.id);
      }
    }
    // `alerts` diwakili kunciAlert supaya refetch dengan isi sama tidak menggambar ulang toast.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [siap, kunciAlert, dikecilkan, bolehAkui, penggunaId]);

  // Keluar dari konsol: toast tidak dibawa ke halaman yang tidak punya banner.
  useEffect(() => {
    const aktif = tampil.current;
    return () => {
      for (const id of aktif) toast.dismiss(idToastKritis(id));
      aktif.clear();
    };
  }, []);

  return null;
}

function KartuToastKritis({ alert, onView, onAck, onKecilkan }: { alert: EwsAlert } & HandlersKritis) {
  const meta = severityMeta[alert.severity];
  const t = toneClass[meta.tone];
  const [mengakui, setMengakui] = useState(false);
  return (
    <motion.div
      initial={{ opacity: 0, x: 48, scale: 0.96 }}
      animate={{ opacity: 1, x: 0, scale: 1 }}
      transition={spring.snappy}
      role="alert"
      aria-live="assertive"
      data-testid="toast-kritis"
      className="glass-strong relative w-full max-w-[calc(100vw-2rem)] overflow-hidden rounded-lg p-4 pl-5"
    >
      <span className={cn("absolute inset-y-0 left-0 w-1", t.bg)} />
      <IsiToast alert={alert} rings={3} />
      <div className="mt-3.5 flex flex-wrap justify-end gap-2">
        {onKecilkan && (
          <SpringButton size="sm" variant="ghost" onClick={onKecilkan}>
            <Minimize2 /> Kecilkan
          </SpringButton>
        )}
        {onAck && (
          <SpringButton
            size="sm"
            variant="ghost"
            disabled={mengakui}
            onClick={async () => {
              setMengakui(true);
              try {
                await onAck();
              } finally {
                setMengakui(false);
              }
            }}
          >
            {mengakui ? "Mengakui…" : "Akui"}
          </SpringButton>
        )}
        <SpringButton size="sm" variant="danger" onClick={onView}>
          Lihat detail
        </SpringButton>
      </div>
    </motion.div>
  );
}

function IsiToast({ alert, rings, className }: { alert: EwsAlert; rings: number; className?: string }) {
  const meta = severityMeta[alert.severity];
  const t = toneClass[meta.tone];
  return (
    <div className={cn("flex gap-3.5", className)}>
      <PulseRing tone={meta.tone} rings={rings} className="size-10 shrink-0">
        <span className={cn("relative grid size-10 place-items-center rounded-full text-white shadow-ambient-sm", t.bg)}>
          <JenisIcon jenis={alert.jenis} className="size-[18px]" />
        </span>
      </PulseRing>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <StatusBadge tone={meta.tone}>{meta.label}</StatusBadge>
          <span className="text-xs text-muted-foreground">Peringatan dini</span>
        </div>
        <p className="mt-1.5 font-display leading-snug font-semibold text-foreground">{alert.judul}</p>
        <p className="mt-0.5 text-sm text-muted-foreground">
          {alert.wilayah}, perubahan <span className={cn("num font-semibold", t.text)}>{formatNilaiEws(alert)}</span>
        </p>
      </div>
    </div>
  );
}

function EwsToastCard({ id, alert, duration, onView, onAck }: { id: string | number; alert: EwsAlert; duration: number } & Handlers) {
  const t = toneClass[severityMeta[alert.severity].tone];
  return (
    <motion.div
      initial={{ opacity: 0, x: 48, scale: 0.96 }}
      animate={{ opacity: 1, x: 0, scale: 1 }}
      transition={spring.snappy}
      role="alert"
      aria-live="assertive"
      className="glass-strong relative w-full max-w-[calc(100vw-2rem)] overflow-hidden rounded-lg p-4 pl-5"
    >
      <span className={cn("absolute inset-y-0 left-0 w-1", t.bg)} />
      <button type="button" onClick={() => toast.dismiss(id)} className="absolute top-3 right-3 grid size-7 place-items-center rounded-md text-muted-foreground hover:bg-secondary" aria-label="Tutup">
        <X className="size-3.5" />
      </button>
      <IsiToast alert={alert} rings={2} className="pr-6" />
      <div className="mt-3.5 flex justify-end gap-2">
        {onAck && (
          <SpringButton size="sm" variant="ghost" onClick={() => { onAck(); toast.dismiss(id); }}>
            Akui
          </SpringButton>
        )}
        <SpringButton size="sm" variant="primary" onClick={() => { onView?.(); toast.dismiss(id); }}>
          Lihat detail
        </SpringButton>
      </div>
      <motion.span className={cn("absolute bottom-0 left-0 h-0.5", t.bg)} initial={{ width: "100%" }} animate={{ width: "0%" }} transition={{ duration: duration / 1000, ease: "linear" }} />
    </motion.div>
  );
}
