"use client";

import { motion } from "motion/react";
import { X } from "lucide-react";
import { toast } from "sonner";
import { PulseRing } from "@/components/motion/pulse-ring";
import { SpringButton } from "@/components/garda/spring-button";
import { StatusBadge } from "@/components/garda/status-badge";
import type { EwsAlert } from "@/lib/data/types";
import { severityMeta, toneClass } from "@/lib/design/tokens";
import { spring } from "@/lib/motion/presets";
import { cn } from "@/lib/utils";
import { JenisIcon, formatNilaiEws } from "./ews-meta";

interface Handlers {
  onView?: () => void;
  onAck?: () => void;
}

/**
 * Toast EWS dengan pulse ring. Kritis = menetap sampai ditindaklanjuti;
 * lainnya hilang otomatis dengan bilah waktu tersisa.
 */
export function notifyEws(alert: EwsAlert, handlers: Handlers = {}) {
  const duration = alert.severity === "KRITIS" ? Infinity : 10_000;
  toast.custom((id) => <EwsToastCard id={id} alert={alert} duration={duration} {...handlers} />, { id: alert.id, duration });
}

function EwsToastCard({ id, alert, duration, onView, onAck }: { id: string | number; alert: EwsAlert; duration: number } & Handlers) {
  const meta = severityMeta[alert.severity];
  const t = toneClass[meta.tone];
  return (
    <motion.div
      initial={{ opacity: 0, x: 48, scale: 0.96 }}
      animate={{ opacity: 1, x: 0, scale: 1 }}
      transition={spring.snappy}
      role="alert"
      aria-live="assertive"
      className="glass-strong relative w-[384px] max-w-[calc(100vw-2rem)] overflow-hidden rounded-2xl p-4 pl-5"
    >
      <span className={cn("absolute inset-y-0 left-0 w-1", t.bg)} />
      <button onClick={() => toast.dismiss(id)} className="absolute top-3 right-3 grid size-7 place-items-center rounded-lg text-muted-foreground hover:bg-secondary" aria-label="Tutup">
        <X className="size-3.5" />
      </button>
      <div className="flex gap-3.5 pr-6">
        <PulseRing tone={meta.tone} rings={alert.severity === "KRITIS" ? 3 : 2} className="size-10 shrink-0">
          <span className={cn("relative grid size-10 place-items-center rounded-full text-white shadow-ambient-sm", t.bg)}>
            <JenisIcon jenis={alert.jenis} className="size-[18px]" />
          </span>
        </PulseRing>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <StatusBadge tone={meta.tone}>{meta.label}</StatusBadge>
            <span className="text-xs text-muted-foreground">Peringatan dini</span>
          </div>
          <p className="mt-1.5 font-display leading-snug font-semibold text-navy-900 dark:text-white">{alert.judul}</p>
          <p className="mt-0.5 text-sm text-muted-foreground">
            {alert.wilayah}, perubahan <span className={cn("num font-semibold", t.text)}>{formatNilaiEws(alert)}</span>
          </p>
        </div>
      </div>
      <div className="mt-3.5 flex justify-end gap-2">
        {onAck && (
          <SpringButton size="sm" variant="ghost" onClick={() => { onAck(); toast.dismiss(id); }}>
            Akui
          </SpringButton>
        )}
        <SpringButton size="sm" variant={alert.severity === "KRITIS" ? "danger" : "primary"} onClick={() => { onView?.(); toast.dismiss(id); }}>
          Lihat detail
        </SpringButton>
      </div>
      {Number.isFinite(duration) && (
        <motion.span className={cn("absolute bottom-0 left-0 h-0.5", t.bg)} initial={{ width: "100%" }} animate={{ width: "0%" }} transition={{ duration: duration / 1000, ease: "linear" }} />
      )}
    </motion.div>
  );
}
