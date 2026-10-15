"use client";

import { AnimatePresence, motion } from "motion/react";
import { AlertTriangle, Check, CloudOff, RefreshCw, UploadCloud } from "lucide-react";
import { useOnline } from "@/lib/offline/use-online";
import { fadeIn, spring } from "@/lib/motion/presets";
import { pendingCount, useFieldQueue } from "@/lib/stores/field-queue-store";
import { cn } from "@/lib/utils";

/** Pil status jaringan + antrean. Ketuk untuk menyinkronkan manual. */
export function OfflineIndicator() {
  const online = useOnline();
  const items = useFieldQueue((s) => s.items);
  const syncing = useFieldQueue((s) => s.syncing);
  const pending = pendingCount(items);
  const konflik = items.filter((i) => i.status === "KONFLIK").length;

  const st = !online
    ? { key: "off", icon: CloudOff, label: pending ? `Offline, ${pending} tersimpan` : "Offline", cls: "bg-mist-200 text-navy-800 dark:bg-white/10 dark:text-white", spin: false }
    : syncing
      ? { key: "sync", icon: RefreshCw, label: `Menyinkronkan ${pending}`, cls: "bg-data-50 text-data-700 dark:bg-data-500/15 dark:text-data-300", spin: true }
      : konflik
        ? { key: "konflik", icon: AlertTriangle, label: `${konflik} konflik`, cls: "bg-defisit-soft text-defisit-ink", spin: false }
        : pending
          ? { key: "pending", icon: UploadCloud, label: `${pending} menunggu`, cls: "bg-waspada-soft text-waspada-ink", spin: false }
          : { key: "ok", icon: Check, label: "Tersinkron", cls: "bg-normal-soft text-normal-ink dark:bg-normal/15 dark:text-emerald-300", spin: false };

  return (
    <motion.button
      layout
      transition={spring.snappy}
      onClick={() => window.dispatchEvent(new CustomEvent("garda:sinkron"))}
      className={cn("inline-flex h-8 items-center gap-1.5 overflow-hidden rounded-full px-3 text-xs font-medium", st.cls)}
      aria-live="polite"
      aria-label={`Status sinkronisasi: ${st.label}`}
    >
      <AnimatePresence mode="popLayout" initial={false}>
        <motion.span key={st.key} variants={fadeIn} initial="hidden" animate="show" exit="exit" className="inline-flex items-center gap-1.5">
          <st.icon className={cn("size-3.5", st.spin && "animate-spin-slow")} />
          <span className={st.key === "sync" || st.key === "pending" || st.key === "off" ? "num" : undefined}>{st.label}</span>
        </motion.span>
      </AnimatePresence>
    </motion.button>
  );
}

export function OfflineBanner() {
  const online = useOnline();
  return (
    <AnimatePresence>
      {!online && (
        <motion.div
          initial={{ height: 0, opacity: 0 }}
          animate={{ height: "auto", opacity: 1 }}
          exit={{ height: 0, opacity: 0 }}
          transition={spring.soft}
          className="overflow-hidden"
        >
          <div className="mx-4 mt-3 flex gap-2.5 rounded-xl bg-navy-800 px-4 py-3 text-sm text-white shadow-ambient">
            <CloudOff className="mt-0.5 size-4 shrink-0 text-data-300" />
            <p>Mode offline. Entri disimpan aman di perangkat dan dikirim otomatis saat jaringan kembali.</p>
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
