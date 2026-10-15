"use client";

import { motion } from "motion/react";
import { KomoditasIcon } from "@/components/garda/komoditas-icon";
import { PanelTitle } from "@/components/garda/page-header";
import type { RingkasanEksekutif } from "@/lib/data/types";
import { formatAngka, formatPersen } from "@/lib/format";
import { ease } from "@/lib/motion/presets";
import { cn } from "@/lib/utils";

const BATAS = 100; // sumbu ±100%; surplus lebih besar ditandai panah

/** Neraca pangan: rasio kecukupan (produksi vs kebutuhan) sebagai batang divergen. */
export function FoodBalance({ data }: { data: RingkasanEksekutif["neraca"] }) {
  const rows = data
    .map((d) => ({ ...d, rasio: (d.produksi / d.kebutuhan - 1) * 100, selisih: d.produksi - d.kebutuhan }))
    .sort((a, b) => b.rasio - a.rasio);

  return (
    <section className="glass h-full rounded-xl p-5 sm:p-6">
      <PanelTitle title="Neraca pangan" description="Produksi terhadap kebutuhan bulanan provinsi" />
      <div className="mt-2 flex justify-between px-[140px] text-[11px] text-muted-foreground max-sm:px-[112px]">
        <span>Defisit</span><span>Surplus</span>
      </div>
      <ul className="mt-2 space-y-2.5">
        {rows.map((r, i) => {
          const surplus = r.rasio >= 0;
          const w = (Math.min(Math.abs(r.rasio), BATAS) / BATAS) * 50;
          return (
            <li key={r.komoditasKode} className="group grid grid-cols-[128px_1fr_92px] items-center gap-3 max-sm:grid-cols-[100px_1fr_80px]" title={`Produksi ${formatAngka(r.produksi)} ton, kebutuhan ${formatAngka(r.kebutuhan)} ton`}>
              <span className="flex min-w-0 items-center gap-2 text-sm">
                <KomoditasIcon kode={r.komoditasKode} className="shrink-0 text-muted-foreground" />
                <span className="truncate">{r.nama}</span>
              </span>
              <div className="relative h-6 rounded-lg bg-mist-100/70 dark:bg-white/5">
                <div className="absolute inset-y-0 left-1/2 w-px bg-navy-900/15 dark:bg-white/20" />
                <motion.div
                  className={cn("absolute inset-y-1 rounded-md", surplus ? "left-1/2 bg-gradient-to-r from-surplus/70 to-surplus" : "right-1/2 bg-gradient-to-l from-defisit/70 to-defisit")}
                  initial={{ width: 0 }}
                  whileInView={{ width: `${w}%` }}
                  viewport={{ once: true }}
                  transition={{ duration: 0.9, ease: ease.garda, delay: i * 0.05 }}
                />
                {Math.abs(r.rasio) > BATAS && <span className={cn("absolute top-1/2 -translate-y-1/2 text-[10px] text-white", surplus ? "right-1.5" : "left-1.5")}>▸▸</span>}
              </div>
              <span className={cn("num text-right text-sm font-semibold", surplus ? "text-surplus-ink dark:text-blue-300" : "text-defisit-ink dark:text-rose-300")}>
                {formatPersen(r.rasio, { digits: 0, signed: true })}
              </span>
            </li>
          );
        })}
      </ul>
      <p className="mt-4 text-xs leading-relaxed text-muted-foreground">
        Defisit terbesar: bawang merah ({formatAngka(Math.abs(rows[rows.length - 1].selisih))} ton/bulan) dipenuhi pasokan antarprovinsi. Arahkan ke kanban siklus tekanan bila pasokan tertunda.
      </p>
    </section>
  );
}
