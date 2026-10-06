"use client";

import { AnimatePresence, motion } from "motion/react";
import { BellRing, Check, ChevronsUp } from "lucide-react";
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

/** Feed EWS real-time: item baru meluncur masuk & berkilau; dapat dicentang untuk aksi massal. */
export function EwsLiveFeed({ limit = 7, className }: { limit?: number; className?: string }) {
  useTick();
  const { data, isLoading } = useEwsList();
  const baru = useEwsStore((s) => s.baru);
  const status = useEwsStore((s) => s.status);
  const [sel, setSel] = useState<string[]>([]);
  const patch = (aksi: string) => (r: EwsAlert): EwsAlert => ({ ...r, status: aksi === "akui" ? "DIAKUI" : "DIESKALASI" });
  const mutate = useBulkAction<EwsAlert>({ sumber: "ews", queryKey: qk.ews.list(), patch: (r, a) => patch(a)(r) });

  const items = (data ?? []).filter((a) => a.status === "BARU" || a.status === "DIESKALASI" || baru.includes(a.id)).slice(0, limit);
  const selectedRows = items.filter((i) => sel.includes(i.id));
  const actions: BulkAction<EwsAlert>[] = [
    { id: "akui", label: "Akui", icon: Check, perm: "ews.akui", tone: "data", run: (rows) => mutate.mutateAsync({ aksi: "akui", ids: rows.map((r) => r.id), label: "Alert diakui" }) },
    { id: "eskalasi", label: "Eskalasi", icon: ChevronsUp, perm: "ews.eskalasi", run: (rows) => mutate.mutateAsync({ aksi: "eskalasi", ids: rows.map((r) => r.id), label: "Alert dieskalasi" }) },
  ];

  return (
    <section className={cn("glass flex flex-col rounded-xl p-5 sm:p-6", className)}>
      <PanelTitle
        title="Peringatan dini"
        description="Alert yang perlu ditindaklanjuti"
        actions={
          <span className="inline-flex items-center gap-2 rounded-full bg-white/60 px-2.5 py-1 text-xs text-muted-foreground dark:bg-white/5">
            <span className={cn("size-2 rounded-full", status === "terputus" ? "bg-mist-400" : "animate-live bg-normal")} />
            {status === "simulasi" ? "Simulasi langsung" : status === "terhubung" ? "Langsung" : status === "terputus" ? "Terputus" : "Menghubungkan"}
          </span>
        }
      />
      <div className="mt-4 flex-1">
        {isLoading ? (
          <div className="space-y-2">{Array.from({ length: 5 }).map((_, i) => <Shimmer key={i} className="h-16 w-full rounded-xl" />)}</div>
        ) : items.length === 0 ? (
          <div className="grid h-full place-items-center py-10 text-center">
            <div>
              <BellRing className="mx-auto size-6 text-normal" />
              <p className="mt-2 font-display font-semibold">Semua alert sudah ditangani</p>
              <p className="text-sm text-muted-foreground">Alert baru akan muncul di sini secara otomatis.</p>
            </div>
          </div>
        ) : (
          <ul>
            <AnimatePresence initial={false}>
              {items.map((a) => {
                const meta = severityMeta[a.severity];
                const t = toneClass[meta.tone];
                const isNew = baru.includes(a.id);
                const checked = sel.includes(a.id);
                return (
                  <motion.li key={a.id} layout variants={feedItem} initial="hidden" animate="show" exit="exit" className="overflow-hidden">
                    <div
                      className={cn(
                        "group relative flex items-center gap-3 rounded-xl border border-transparent p-3 transition-colors",
                        checked ? "border-data-200 bg-data-50/70 dark:border-data-500/30 dark:bg-data-500/10" : "hover:bg-white/60 dark:hover:bg-white/5",
                      )}
                    >
                      {isNew && <motion.span aria-hidden className="absolute inset-0 rounded-xl bg-data-100 dark:bg-data-500/20" initial={{ opacity: 0.9 }} animate={{ opacity: 0 }} transition={{ duration: 2.4 }} />}
                      <span className={cn("relative transition-opacity", sel.length ? "opacity-100" : "opacity-0 group-hover:opacity-100 focus-within:opacity-100")}>
                        <Checkbox checked={checked} onCheckedChange={() => setSel((s) => (checked ? s.filter((x) => x !== a.id) : [...s, a.id]))} aria-label={`Pilih ${a.judul}`} className="rounded-md" />
                      </span>
                      <PulseRing tone={meta.tone} rings={meta.rank >= 3 ? 2 : 1} active={meta.rank >= 2 && a.status === "BARU"} className="size-9 shrink-0">
                        <span className={cn("relative grid size-9 place-items-center rounded-full", t.soft, t.text)}>
                          <JenisIcon jenis={a.jenis} className="size-4" />
                        </span>
                      </PulseRing>
                      <Link href={`/ews?id=${a.id}`} className="relative min-w-0 flex-1">
                        <p className="truncate text-sm font-medium">{a.judul}</p>
                        <p className="truncate text-xs text-muted-foreground">{a.wilayah}, {formatRelatif(a.waktu)}</p>
                      </Link>
                      <span className={cn("num relative text-sm font-semibold", t.text)}>{formatNilaiEws(a)}</span>
                    </div>
                  </motion.li>
                );
              })}
            </AnimatePresence>
          </ul>
        )}
      </div>
      <BulkActionBar rows={selectedRows} actions={actions} entityLabel="alert" onClear={() => setSel([])} placement="inline" />
      <Link href="/ews" className="mt-3 text-center text-sm font-medium text-data-700 hover:underline dark:text-data-300">Lihat semua peringatan</Link>
    </section>
  );
}
