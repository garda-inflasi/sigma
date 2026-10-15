"use client";

import { motion } from "motion/react";
import { PanelTitle } from "@/components/garda/page-header";
import { Shimmer } from "@/components/garda/shimmer";
import { namaWilayah } from "@/lib/data/generators";
import { USERS } from "@/lib/data/reference";
import { ease } from "@/lib/motion/presets";
import { useEntriHarga } from "@/lib/query/hooks";
import { cn } from "@/lib/utils";

const TARGET_PER_AGEN = 14;

/** Aktivitas tim lapangan per wilayah: capaian entri hari ini vs target. */
export function FieldActivity() {
  const { data, isLoading } = useEntriHarga();
  const agen = USERS.filter((u) => u.role === "OPS_MAHASISWA");
  const perKab = [...new Set(agen.map((a) => a.kabupatenKode!))].map((kode) => {
    const nAgen = agen.filter((a) => a.kabupatenKode === kode).length;
    const entri = data?.filter((e) => e.kabupatenKode === kode).length ?? 0;
    const target = nAgen * TARGET_PER_AGEN;
    return { kode, nama: namaWilayah(kode), nAgen, entri, target, pct: Math.min(100, (entri / target) * 100) };
  }).sort((a, b) => b.pct - a.pct);

  return (
    <section className="glass h-full rounded-xl p-5 sm:p-6">
      <PanelTitle title="Aktivitas tim lapangan" description="Capaian entri hari ini terhadap target" />
      <ul className="mt-5 space-y-3.5">
        {isLoading
          ? Array.from({ length: 6 }).map((_, i) => <Shimmer key={i} className="h-9 w-full" />)
          : perKab.map((k, i) => (
            <li key={k.kode}>
              <div className="flex items-baseline justify-between gap-2 text-sm">
                <span className="truncate">{k.nama}</span>
                <span className="num shrink-0 text-xs text-muted-foreground">
                  <span className="font-semibold text-foreground">{k.entri}</span>/{k.target}
                </span>
              </div>
              <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-mist-200 dark:bg-white/10">
                <motion.div
                  className={cn("h-full rounded-full", k.pct >= 80 ? "bg-normal" : k.pct >= 50 ? "bg-data-500" : "bg-waspada")}
                  initial={{ width: 0 }}
                  animate={{ width: `${k.pct}%` }}
                  transition={{ duration: 0.9, ease: ease.garda, delay: 0.1 + i * 0.05 }}
                />
              </div>
            </li>
          ))}
      </ul>
    </section>
  );
}
