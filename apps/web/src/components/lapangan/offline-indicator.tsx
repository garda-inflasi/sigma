"use client";

import { AnimatePresence, motion } from "motion/react";
import {
  AlertTriangle,
  Check,
  CloudOff,
  RefreshCw,
  UploadCloud,
} from "lucide-react";
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
    ? {
        key: "off",
        icon: CloudOff,
        label: pending ? `Offline, ${pending} tersimpan` : "Offline",
        cls: "bg-mist-200 text-navy-800 dark:bg-white/10 dark:text-white",
        spin: false,
      }
    : syncing
      ? {
          key: "sync",
          icon: RefreshCw,
          label: `Mengirim ${pending}`,
          cls: "bg-data-50 text-data-700 dark:bg-data-500/15 dark:text-data-300",
          spin: true,
        }
      : konflik
        ? {
            key: "konflik",
            icon: AlertTriangle,
            label: `${konflik} perlu dicek`,
            cls: "bg-defisit-soft text-defisit-ink dark:bg-defisit/15 dark:text-rose-300",
            spin: false,
          }
        : pending
          ? {
              key: "pending",
              icon: UploadCloud,
              label: `${pending} belum terkirim`,
              cls: "bg-waspada-soft text-waspada-ink dark:bg-waspada/15 dark:text-amber-300",
              spin: false,
            }
          : {
              key: "ok",
              icon: Check,
              label: "Semua terkirim",
              cls: "bg-normal-soft text-normal-ink dark:bg-normal/15 dark:text-emerald-300",
              spin: false,
            };

  return (
    <motion.button
      type="button"
      layout
      transition={spring.snappy}
      onClick={() => window.dispatchEvent(new CustomEvent("garda:sinkron"))}
      className={cn(
        "inline-flex h-10 min-w-0 items-center gap-1.5 overflow-hidden rounded-full px-3 text-xs font-medium",
        st.cls,
      )}
      aria-live="polite"
      aria-label={`Status sinkronisasi: ${st.label}. Ketuk untuk mengirim sekarang.`}
    >
      <AnimatePresence mode="popLayout" initial={false}>
        <motion.span
          key={st.key}
          variants={fadeIn}
          initial="hidden"
          animate="show"
          exit="exit"
          className="inline-flex min-w-0 items-center gap-1.5"
        >
          <st.icon
            className={cn("size-3.5 shrink-0", st.spin && "animate-spin-slow")}
          />
          <span
            className={cn(
              "truncate",
              st.key !== "ok" && st.key !== "konflik" && "num",
            )}
          >
            {st.label}
          </span>
        </motion.span>
      </AnimatePresence>
    </motion.button>
  );
}

export function OfflineBanner() {
  const online = useOnline();
  const pending = useFieldQueue((s) => pendingCount(s.items));
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
          <div
            role="status"
            className="mx-4 mt-3 flex gap-2.5 rounded-xl bg-navy-800 px-4 py-3 text-sm text-white shadow-ambient"
          >
            <CloudOff
              className="mt-0.5 size-4 shrink-0 text-data-300"
              aria-hidden
            />
            <div className="min-w-0">
              <p className="font-medium">Anda sedang offline</p>
              <p className="text-[13px] text-white/80">
                {pending ? (
                  <>
                    <span className="num">{pending}</span> entri aman di
                    perangkat
                  </>
                ) : (
                  "Entri baru disimpan aman di perangkat"
                )}{" "}
                dan dikirim otomatis saat jaringan kembali.
              </p>
            </div>
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
