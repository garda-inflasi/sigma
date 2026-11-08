"use client";

import { AnimatePresence, motion } from "motion/react";
import { ArrowRight, BellRing, Check, ChevronsUp, MapPin } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";
import { Checkbox } from "@/components/ui/checkbox";
import { BulkActionBar, type BulkAction } from "@/components/data-table/bulk-action-bar";
import { PulseRing } from "@/components/motion/pulse-ring";
import { PanelTitle } from "@/components/garda/page-header";
import { Shimmer } from "@/components/garda/shimmer";
import type { EwsAlert } from "@/lib/data/types";
import { severityMeta, toneClass } from "@/lib/design/tokens";
import { formatRelatif } from "@/lib/format";
import { feedItem } from "@/lib/motion/presets";
import { useBulkAction, useEwsList } from "@/lib/query/hooks";
import { qk } from "@/lib/query/keys";
import { useEwsStore } from "@/lib/stores/ews-store";
import { cn } from "@/lib/utils";
import { JenisIcon, formatNilaiEws } from "./ews-meta";

function useTick(ms = 30_000) {
  const [, set] = useState(0);
  useEffect(() => {
    const t = setInterval(() => set((n) => n + 1), ms);
    return () => clearInterval(t);
  }, [ms]);
}

/** Feed EWS real-time: kartu alert informatif dengan tingkat keparahan, pesan ringkas, wilayah, dan metrik. */
export function EwsLiveFeed({ limit = 35, className }: { limit?: number; className?: string }) {
  useTick();
  const { data, isLoading } = useEwsList();
  const baru = useEwsStore((s) => s.baru);
  const status = useEwsStore((s) => s.status);
  const [sel, setSel] = useState<string[]>([]);
  const patch = (aksi: string) => (r: EwsAlert): EwsAlert => ({ ...r, status: aksi === "akui" ? "DIAKUI" : "DIESKALASI" });
  const mutate = useBulkAction<EwsAlert>({ sumber: "ews", queryKey: qk.ews.list(), invalidate: qk.ews.all(), patch: (r, a) => patch(a)(r) });

  const items = (data ?? []).filter((a) => a.status === "BARU" || a.status === "DIESKALASI" || baru.includes(a.id)).slice(0, limit);
  const selectedRows = items.filter((i) => sel.includes(i.id));
  const actions: BulkAction<EwsAlert>[] = [
    { id: "akui", label: "Akui", icon: Check, perm: "ews.akui", tone: "data", run: (rows) => mutate.mutateAsync({ aksi: "akui", ids: rows.map((r) => r.id), label: "Alert diakui" }) },
    { id: "eskalasi", label: "Eskalasi", icon: ChevronsUp, perm: "ews.eskalasi", run: (rows) => mutate.mutateAsync({ aksi: "eskalasi", ids: rows.map((r) => r.id), label: "Alert dieskalasi" }) },
  ];

  return (
    <section className={cn("glass h-full flex flex-col rounded-xl p-5 sm:p-6", className)}>
      <PanelTitle
        title="Peringatan dini"
        description="Alert aktif yang memerlukan tindakan cepat"
        actions={
          <div className="flex items-center gap-2">
            {items.length > 0 && (
              <span className="inline-flex items-center rounded-full bg-defisit/15 px-2.5 py-0.5 text-xs font-semibold text-defisit">
                {items.length} alert
              </span>
            )}
            <span className="inline-flex items-center gap-2 rounded-full bg-white/70 px-2.5 py-1 text-xs text-muted-foreground shadow-xs dark:bg-white/5">
              <span className={cn("size-2 rounded-full", status === "terputus" ? "bg-mist-400" : "animate-live bg-normal")} />
              {status === "simulasi" ? "Simulasi" : status === "terhubung" ? "Langsung" : status === "terputus" ? "Terputus" : "Menghubungkan"}
            </span>
          </div>
        }
      />

      <div className="mt-4 flex-1 min-h-0 overflow-y-auto pr-1.5 space-y-3 scrollbar-thin">
        {isLoading ? (
          <div className="space-y-3">
            {Array.from({ length: 4 }).map((_, i) => (
              <Shimmer key={i} className="h-24 w-full rounded-xl" />
            ))}
          </div>
        ) : items.length === 0 ? (
          <div className="grid h-full min-h-[220px] place-items-center rounded-xl border border-dashed border-border/70 p-8 text-center bg-white/30 dark:bg-white/5">
            <div>
              <div className="mx-auto grid size-12 place-items-center rounded-2xl bg-normal/10 text-normal">
                <BellRing className="size-6" />
              </div>
              <p className="mt-3 font-display font-semibold text-foreground">Semua alert sudah ditangani</p>
              <p className="mt-1 text-xs text-muted-foreground max-w-xs">
                Tidak ada peringatan dini yang memerlukan tindakan saat ini. Peringatan baru akan otomatis masuk secara live.
              </p>
            </div>
          </div>
        ) : (
          <AnimatePresence initial={false}>
              {items.map((a) => {
                const meta = severityMeta[a.severity];
                const t = toneClass[meta.tone];
                const isNew = baru.includes(a.id);
                const checked = sel.includes(a.id);
                return (
                  <motion.div
                    key={a.id}
                    layout
                    variants={feedItem}
                    initial="hidden"
                    animate="show"
                    exit="exit"
                    className="overflow-hidden"
                  >
                    <div
                      className={cn(
                        "group relative flex flex-col gap-2.5 rounded-xl border p-3.5 transition-all shadow-xs",
                        checked
                          ? "border-data-300 bg-data-50/90 dark:border-data-500/40 dark:bg-data-500/15 ring-1 ring-data-400/40"
                          : "border-border/60 bg-white/60 hover:bg-white/90 hover:border-border hover:shadow-ambient dark:bg-white/5 dark:hover:bg-white/10 dark:border-white/10",
                      )}
                    >
                      {isNew && (
                        <motion.span
                          aria-hidden
                          className="absolute inset-0 rounded-xl bg-data-100 dark:bg-data-500/20"
                          initial={{ opacity: 0.9 }}
                          animate={{ opacity: 0 }}
                          transition={{ duration: 2.4 }}
                        />
                      )}

                      {/* Baris Atas: Checkbox, Icon, Severity, Judul, dan Nilai Deviasi */}
                      <div className="flex items-start justify-between gap-2.5">
                        <div className="flex items-start gap-2.5 min-w-0 flex-1">
                          <div className="pt-0.5 shrink-0">
                            <Checkbox
                              checked={checked}
                              onCheckedChange={() =>
                                setSel((s) => (checked ? s.filter((x) => x !== a.id) : [...s, a.id]))
                              }
                              aria-label={`Pilih ${a.judul}`}
                              className="rounded-md"
                            />
                          </div>

                          <PulseRing
                            tone={meta.tone}
                            rings={meta.rank >= 3 ? 2 : 1}
                            active={meta.rank >= 2 && a.status === "BARU"}
                            className="size-8 shrink-0 mt-0.5"
                          >
                            <span className={cn("relative grid size-8 place-items-center rounded-lg shadow-2xs", t.soft, t.text)}>
                              <JenisIcon jenis={a.jenis} className="size-4" />
                            </span>
                          </PulseRing>

                          <div className="min-w-0 flex-1">
                            <div className="flex flex-wrap items-center gap-1.5 mb-1">
                              <span
                                className={cn(
                                  "inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[10px] font-bold uppercase tracking-wider",
                                  t.soft,
                                  t.text,
                                )}
                              >
                                <span className={cn("size-1.5 rounded-full", t.bg)} />
                                {meta.label}
                              </span>

                              {a.status === "DIESKALASI" && (
                                <span className="rounded-md bg-waspada-soft/80 px-1.5 py-0.5 text-[10px] font-semibold text-waspada-ink">
                                  Dieskalasi
                                </span>
                              )}
                            </div>

                            <Link
                              href={`/ews?id=${a.id}`}
                              className="font-display text-sm font-semibold text-foreground hover:text-data-600 dark:hover:text-data-300 transition-colors line-clamp-1"
                            >
                              {a.judul}
                            </Link>
                          </div>
                        </div>

                        {/* Nilai Metrik di Kanan */}
                        <div className="shrink-0 text-right">
                          <span className={cn("num inline-flex items-center rounded-lg px-2.5 py-1 text-xs font-bold shadow-2xs", t.soft, t.text)}>
                            {formatNilaiEws(a)}
                          </span>
                        </div>
                      </div>

                      {/* Deskripsi / Pesan Detail */}
                      {a.pesan && (
                        <p className="text-xs leading-relaxed text-muted-foreground line-clamp-2 pl-9">
                          {a.pesan}
                        </p>
                      )}

                      {/* Baris Bawah: Wilayah, Komoditas, Waktu, & Aksi Cepat */}
                      <div className="flex items-center justify-between gap-2 pt-2 border-t border-border/40 pl-9 text-[11px] text-muted-foreground">
                        <div className="flex flex-wrap items-center gap-x-2 gap-y-1 min-w-0">
                          <span className="inline-flex items-center gap-1 font-medium text-foreground/80">
                            <MapPin className="size-3 text-muted-foreground shrink-0" />
                            <span className="truncate max-w-[140px] sm:max-w-[200px]">{a.wilayah}</span>
                          </span>
                          {a.komoditasNama && (
                            <>
                              <span>•</span>
                              <span className="truncate max-w-[120px]">{a.komoditasNama}</span>
                            </>
                          )}
                          <span>•</span>
                          <span className="shrink-0">{formatRelatif(a.waktu)}</span>
                        </div>

                        <Link
                          href={`/ews?id=${a.id}`}
                          className="inline-flex items-center gap-0.5 text-data-600 hover:text-data-700 hover:underline dark:text-data-400 font-semibold shrink-0"
                        >
                          Detail <ArrowRight className="size-3" />
                        </Link>
                      </div>
                    </div>
                  </motion.div>
                );
              })}
            </AnimatePresence>
          )}
      </div>

      <BulkActionBar rows={selectedRows} actions={actions} entityLabel="alert" onClear={() => setSel([])} placement="inline" />

      <div className="mt-3 pt-3 border-t border-border/50 text-center shrink-0">
        <Link
          href="/ews"
          className="inline-flex items-center justify-center gap-1.5 text-xs font-semibold text-data-700 hover:text-data-800 hover:underline dark:text-data-300 dark:hover:text-data-200"
        >
          Buka pusat kendali peringatan dini (EWS) <ArrowRight className="size-3.5" />
        </Link>
      </div>
    </section>
  );
}
