"use client";

import { motion } from "motion/react";
import { KomoditasIcon } from "@/components/garda/komoditas-icon";
import { PanelTitle } from "@/components/garda/page-header";
import type { RingkasanEksekutif, SumberAngka } from "@/lib/data/types";
import { formatAngka, formatPersen } from "@/lib/format";
import { ease } from "@/lib/motion/presets";
import { cn } from "@/lib/utils";

const BATAS = 100; // sumbu ±100%; surplus lebih besar ditandai panah

/** Neraca pangan: rasio kecukupan (produksi vs kebutuhan) sebagai batang divergen. */
export function FoodBalance({ data, sumber }: { data: NonNullable<RingkasanEksekutif["neraca"]>["items"]; sumber: SumberAngka }) {
  const rows = data
    .map((d) => ({ ...d, rasio: (d.produksi / d.kebutuhan - 1) * 100, selisih: d.produksi - d.kebutuhan }))
    .sort((a, b) => b.rasio - a.rasio);

  return (
    <section className="glass flex flex-col rounded-lg p-3.5 lg:h-full lg:min-h-0">
      <PanelTitle title="Neraca pangan" description={`Produksi terhadap kebutuhan bulanan provinsi, ${sumber === "DEMO" ? "data demo" : sumber}`} />
      <div className="mt-2 grid shrink-0 grid-cols-[128px_1fr_92px] gap-3 pr-1 text-[11px] text-muted-foreground max-sm:grid-cols-[100px_1fr_80px]">
        <span aria-hidden />
        <span className="flex justify-between"><span>Defisit</span><span>Surplus</span></span>
      </div>
      <ul className="scrollbar-thin mt-2 flex flex-col gap-2 pr-1 lg:min-h-0 lg:flex-1 lg:justify-around lg:overflow-y-auto">
        {rows.map((r, i) => {
          const surplus = r.rasio >= 0;
          const w = (Math.min(Math.abs(r.rasio), BATAS) / BATAS) * 50;
          return (
            <li key={r.komoditasKode} className="group grid grid-cols-[128px_1fr_92px] items-center gap-3 max-sm:grid-cols-[100px_1fr_80px]" title={`Produksi ${formatAngka(r.produksi)} ton, kebutuhan ${formatAngka(r.kebutuhan)} ton`}>
              <span className="flex min-w-0 items-center gap-2 text-sm">
                <KomoditasIcon kode={r.komoditasKode} className="shrink-0 text-muted-foreground" />
                <span className="truncate">{r.nama}</span>
              </span>
              <div className="relative h-6 rounded-lg bg-secondary">
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
      <p className="mt-3 shrink-0 text-xs leading-relaxed text-muted-foreground">
        {rows.length > 0 && rows[rows.length - 1].selisih < 0
          ? `Defisit terbesar: ${rows[rows.length - 1].nama.toLowerCase()} (${formatAngka(Math.abs(rows[rows.length - 1].selisih))} ton/bulan). Arahkan ke kanban siklus tekanan bila pasokan tertunda.`
          : "Tidak ada komoditas yang defisit pada periode ini."}
      </p>
    </section>
  );
}
