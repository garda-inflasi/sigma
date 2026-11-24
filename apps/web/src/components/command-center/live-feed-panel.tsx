"use client";

import Link from "next/link";
import { memo, useMemo } from "react";
import { AnimatePresence, motion } from "motion/react";
import { BrainCircuit, Route, Siren, Snowflake, Truck } from "lucide-react";
import { StatusBadge } from "@/components/garda/status-badge";
import { PulseRing } from "@/components/motion/pulse-ring";
import type { LogistikOperasional, StokOperasional } from "@/lib/api/dashboard-operasional";
import type { StatusMl } from "@/lib/api/prediksi";
import type { EwsAlert } from "@/lib/data/types";
import { severityMeta } from "@/lib/design/tokens";
import { formatRelatif } from "@/lib/format";
import { cn } from "@/lib/utils";

const MAKS_FEED = 15;

export interface LiveFeedPanelProps {
  alerts?: EwsAlert[];
  kritisBaru?: EwsAlert[];
  newAlertIds: string[];
  logistik?: LogistikOperasional;
  stok?: StokOperasional;
  statusMl?: StatusMl;
  bolehEws: boolean;
}

export const LiveFeedPanel = memo(function LiveFeedPanel({
  alerts,
  kritisBaru,
  newAlertIds,
  logistik,
  stok,
  statusMl,
  bolehEws,
}: LiveFeedPanelProps) {
  // Peringatan KRITIS tetap diprioritaskan dan persisten di atas feed.
  const kritisPinned = useMemo(() => {
    const map = new Map<string, EwsAlert>();
    for (const a of kritisBaru ?? []) map.set(a.id, a);
    for (const a of alerts ?? []) {
      if (a.severity === "KRITIS" && a.status !== "SELESAI") {
        map.set(a.id, a);
      }
    }
    return [...map.values()].slice(0, 4);
  }, [alerts, kritisBaru]);

  const feedTerbatas = useMemo(() => {
    const kritisIds = new Set(kritisPinned.map((k) => k.id));
    return (alerts ?? [])
      .filter((a) => !kritisIds.has(a.id))
      .slice(0, MAKS_FEED - kritisPinned.length);
  }, [alerts, kritisPinned]);

  const gerakTerbaru = (logistik?.terbaru ?? []).slice(0, 3);

  return (
    <aside
      aria-label="Aliran peringatan dini dan denyut operasi"
      className="cc-bento glass flex min-h-0 flex-col overflow-hidden rounded-xl"
    >
      <div className="flex shrink-0 items-center justify-between gap-2 border-b border-border px-3.5 py-2.5">
        <div className="flex items-center gap-2 min-w-0">
          <Siren className="size-4 shrink-0 text-defisit" />
          <h2 className="truncate text-xs font-semibold uppercase tracking-wide text-foreground">
            Aliran EWS Live
          </h2>
        </div>
        {bolehEws && (
          <Link
            href="/ews"
            className="text-[11px] font-medium text-data-600 hover:underline"
          >
            Semua →
          </Link>
        )}
      </div>

      {/* Pinned KRITIS alerts (persisten, tidak pernah hilang diam-diam) */}
      {kritisPinned.length > 0 && (
        <div className="shrink-0 space-y-1.5 border-b border-rose-200 bg-rose-50/75 p-2.5 dark:border-rose-900/60 dark:bg-rose-950/30">
          <p className="flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-wider text-rose-700 dark:text-rose-300">
            <PulseRing tone="kritis" rings={1} />
            Prioritas Kritis ({kritisPinned.length})
          </p>
          {kritisPinned.map((k) => (
            <Link
              key={k.id}
              href={`/ews?id=${k.id}`}
              className="block rounded-lg border border-rose-300/80 bg-card/95 px-2.5 py-1.5 transition-colors hover:bg-secondary/70 dark:border-rose-900"
            >
              <div className="flex items-center justify-between gap-2">
                <span className="truncate text-xs font-semibold text-foreground">
                  {k.judul}
                </span>
                <span className="shrink-0 rounded bg-rose-500 px-1.5 py-0.5 text-[10px] font-semibold text-white">
                  KRITIS
                </span>
              </div>
              <p className="mt-0.5 truncate text-[11px] text-muted-foreground">
                {k.wilayah} · {formatRelatif(k.waktu)}
              </p>
            </Link>
          ))}
        </div>
      )}

      {/* Feed EWS real-time */}
      <div
        data-lenis-prevent
        className="scrollbar-thin min-h-0 flex-1 overflow-y-auto p-2.5 space-y-1.5"
      >
        {!bolehEws ? (
          <p className="p-3 text-center text-xs text-muted-foreground">
            Peran saat ini tidak memiliki izin melihat peringatan EWS.
          </p>
        ) : feedTerbatas.length === 0 && kritisPinned.length === 0 ? (
          <p className="p-4 text-center text-xs text-muted-foreground">
            Tidak ada peringatan aktif. Kondisi wilayah terpantau normal.
          </p>
        ) : (
          <AnimatePresence initial={false}>
            {feedTerbatas.map((a) => {
              const isBaru = newAlertIds.includes(a.id);
              const tone = severityMeta[a.severity]?.tone ?? "netral";
              return (
                <motion.div
                  key={a.id}
                  layout
                  initial={{ opacity: 0, y: -6 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, height: 0 }}
                  transition={{ duration: 0.24 }}
                >
                  <Link
                    href={`/ews?id=${a.id}`}
                    className={cn(
                      "block rounded-lg border border-border bg-card/70 px-2.5 py-2 transition-colors hover:bg-secondary/70",
                      isBaru && "ring-1 ring-sky-400/70 bg-sky-50/40 dark:bg-sky-950/20",
                    )}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <span className="truncate text-xs font-medium text-foreground">
                        {a.judul}
                      </span>
                      <StatusBadge tone={tone}>{a.severity}</StatusBadge>
                    </div>
                    <p className="mt-0.5 truncate text-[11px] text-muted-foreground">
                      {a.wilayah}
                      {a.komoditasNama ? ` · ${a.komoditasNama}` : ""} ·{" "}
                      {formatRelatif(a.waktu)}
                    </p>
                  </Link>
                </motion.div>
              );
            })}
          </AnimatePresence>
        )}
      </div>

      {/* Denyut Logistik, Cold Storage & ML datasafe */}
      <div className="shrink-0 space-y-2 border-t border-border bg-muted/25 px-3 py-2.5">
        <div className="flex items-center justify-between text-[11px]">
          <span className="inline-flex items-center gap-1.5 font-semibold text-foreground">
            <Truck className="size-3.5 text-data-600" />
            Logistik &amp; Stok
          </span>
          <span className="num text-muted-foreground">
            {logistik?.rute.jumlah ?? 0} rute ·{" "}
            {stok ? `${stok.ringkasan.totalTon.toFixed(0)} ton` : "—"}
          </span>
        </div>

        {gerakTerbaru.length > 0 ? (
          <div className="space-y-1">
            {gerakTerbaru.map((g) => (
              <div
                key={g.id}
                className="flex items-center justify-between gap-2 rounded border border-border bg-card/70 px-2 py-1 text-[11px]"
              >
                <span className="inline-flex items-center gap-1 truncate text-foreground">
                  <Route className="size-3 shrink-0 text-muted-foreground" />
                  {g.komoditasNama} ({g.jenis})
                </span>
                <span className="num shrink-0 font-medium text-muted-foreground">
                  {g.volumeTon.toFixed(1)} t
                </span>
              </div>
            ))}
          </div>
        ) : (
          <p className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
            <Snowflake className="size-3 shrink-0" />
            Belum ada pergerakan stok baru pekan ini.
          </p>
        )}

        <div className="flex items-center justify-between border-t border-border/60 pt-1.5 text-[11px]">
          <span className="inline-flex items-center gap-1 text-muted-foreground">
            <BrainCircuit className="size-3.5 text-purple-600" />
            Model ML
          </span>
          <StatusBadge tone={statusMl?.tersedia ? "normal" : "netral"}>
            {statusMl?.tersedia
              ? `Aktif${statusMl.versi ? ` · ${statusMl.versi}` : ""}`
              : "Siaga"}
          </StatusBadge>
        </div>
      </div>
    </aside>
  );
});
