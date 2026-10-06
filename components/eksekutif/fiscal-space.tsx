"use client";

import { motion } from "motion/react";
import { PolarAngleAxis, RadialBar, RadialBarChart, ResponsiveContainer } from "recharts";
import { PanelTitle } from "@/components/garda/page-header";
import { StatusBadge } from "@/components/garda/status-badge";
import type { RingkasanEksekutif } from "@/lib/data/types";
import { garda } from "@/lib/design/tokens";
import { formatPersen, formatRupiah, formatTanggal } from "@/lib/format";
import { ease } from "@/lib/motion/presets";

/** Ruang fiskal (perspektif Kanwil DJPb): serapan instrumen belanja pengendalian inflasi. */
export function FiscalSpace({ data }: { data: RingkasanEksekutif["fiskal"] }) {
  const pct = (data.realisasi / data.pagu) * 100;
  const sisa = data.pagu - data.realisasi;
  const tone = data.status === "MEMADAI" ? "normal" : data.status === "TERBATAS" ? "waspada" : "defisit";
  return (
    <section className="glass h-full rounded-xl p-5 sm:p-6">
      <PanelTitle
        title="Ruang fiskal"
        description={`Kanwil DJPb, per ${formatTanggal(data.perTanggal)}`}
        actions={<StatusBadge tone={tone}>{data.status === "MEMADAI" ? "Memadai" : data.status === "TERBATAS" ? "Terbatas" : "Kritis"}</StatusBadge>}
      />
      <div className="mt-4 grid items-center gap-5 sm:grid-cols-[180px_1fr]">
        <div className="relative mx-auto size-[180px]">
          <ResponsiveContainer width="100%" height="100%">
            <RadialBarChart innerRadius="74%" outerRadius="100%" data={[{ v: pct }]} startAngle={90} endAngle={-270}>
              <PolarAngleAxis type="number" domain={[0, 100]} tick={false} />
              <RadialBar dataKey="v" cornerRadius={12} fill={garda.data[500]} background={{ fill: "var(--muted)" }} animationDuration={1300} />
            </RadialBarChart>
          </ResponsiveContainer>
          <div className="absolute inset-0 grid place-items-center text-center">
            <div>
              <p className="num text-2xl font-semibold">{formatPersen(pct, { digits: 0 })}</p>
              <p className="text-xs text-muted-foreground">terealisasi</p>
            </div>
          </div>
        </div>
        <dl className="grid grid-cols-2 gap-3 text-sm">
          <div className="rounded-xl bg-white/50 p-3 dark:bg-white/5"><dt className="text-xs text-muted-foreground">Pagu</dt><dd className="num mt-1 font-semibold">{formatRupiah(data.pagu, { compact: true })}</dd></div>
          <div className="rounded-xl bg-white/50 p-3 dark:bg-white/5"><dt className="text-xs text-muted-foreground">Sisa ruang</dt><dd className="num mt-1 font-semibold text-data-700 dark:text-data-300">{formatRupiah(sisa, { compact: true })}</dd></div>
          <div className="col-span-2 rounded-xl bg-white/50 p-3 dark:bg-white/5">
            <dt className="text-xs text-muted-foreground">Skor kelayakan intervensi fiskal</dt>
            <dd className="mt-1 flex items-baseline gap-1"><span className="num text-xl font-semibold">{data.skor}</span><span className="text-xs text-muted-foreground">/100</span></dd>
          </div>
        </dl>
      </div>
      <ul className="mt-5 space-y-3">
        {data.instrumen.map((ins, i) => {
          const p = (ins.realisasi / ins.pagu) * 100;
          return (
            <li key={ins.nama}>
              <div className="flex items-baseline justify-between gap-3 text-sm">
                <span className="truncate">{ins.nama}</span>
                <span className="num shrink-0 text-xs text-muted-foreground">{formatRupiah(ins.realisasi, { compact: true })} / {formatRupiah(ins.pagu, { compact: true })}</span>
              </div>
              <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-mist-200 dark:bg-white/10">
                <motion.div className="h-full rounded-full bg-navy-600 dark:bg-data-400" initial={{ width: 0 }} whileInView={{ width: `${p}%` }} viewport={{ once: true }} transition={{ duration: 0.9, ease: ease.garda, delay: i * 0.06 }} />
              </div>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
