"use client";

import { motion } from "motion/react";
import { Line, LineChart, ReferenceArea, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { GlassTooltip } from "@/components/charts/glass-tooltip";
import { CountUp } from "@/components/motion/count-up";
import type { RingkasanEksekutif } from "@/lib/data/types";
import { garda } from "@/lib/design/tokens";
import { formatPersen, formatTanggal } from "@/lib/format";
import { ease, spring } from "@/lib/motion/presets";
import { cn } from "@/lib/utils";

const SKALA = 5; // sumbu koridor 0–5%
const pos = (v: number) => `${Math.max(0, Math.min(100, (v / SKALA) * 100))}%`;

/**
 * Hero eksekutif: inflasi tahunan diletakkan di "koridor sasaran".
 * Visual utamanya adalah posisi angka terhadap koridor, bukan angka saja.
 */
export function InflationHero({ data }: { data: RingkasanEksekutif["inflasi"] }) {
  const { yoy, mtm, ytd, sasaran, seri, rilis, penyumbang } = data;
  const dalam = yoy >= sasaran.bawah && yoy <= sasaran.atas;
  const maxAndil = Math.max(...penyumbang.map((p) => Math.abs(p.andil)));

  return (
    <section className="relative overflow-hidden rounded-xl bg-gradient-to-br from-navy-800 via-navy-900 to-navy-950 p-6 text-white shadow-ambient-lg sm:p-8">
      <div aria-hidden className="animate-aurora absolute -top-24 -right-24 size-[420px] rounded-full bg-data-500/25 blur-3xl" />
      <div aria-hidden className="absolute inset-0 bg-[radial-gradient(60%_80%_at_0%_100%,rgba(51,194,184,0.12),transparent)]" />
      <div className="relative grid gap-8 lg:grid-cols-[1.15fr_1fr]">
        <div>
          <p className="text-sm text-navy-200">Inflasi tahunan Provinsi Aceh, September 2026</p>
          <div className="mt-3 flex items-end gap-2">
            <CountUp value={yoy} format={(n) => n.toFixed(2).replace(".", ",")} className="text-display-2xl font-semibold tracking-tight" duration={1.6} />
            <span className="num pb-2 text-3xl text-navy-300">%</span>
          </div>
          <span className={cn("mt-3 inline-flex h-7 items-center gap-2 rounded-full px-3 text-xs font-medium", dalam ? "bg-emerald-400/15 text-emerald-200" : "bg-rose-400/15 text-rose-200")}>
            <span className={cn("size-1.5 rounded-full", dalam ? "bg-emerald-300" : "bg-rose-300")} />
            {dalam ? "Dalam koridor sasaran" : "Di luar koridor sasaran"} {formatPersen((sasaran.bawah + sasaran.atas) / 2)} ± {formatPersen((sasaran.atas - sasaran.bawah) / 2, { digits: 0 })}
          </span>

          {/* Koridor sasaran */}
          <div className="mt-8" role="img" aria-label={`Inflasi ${formatPersen(yoy, { digits: 2 })} pada koridor ${sasaran.bawah}–${sasaran.atas}%`}>
            <div className="relative h-14">
              <div className="absolute top-7 right-0 left-0 h-2 rounded-full bg-white/10" />
              <motion.div
                className="absolute top-[22px] h-[18px] rounded-md border border-dashed border-emerald-300/60 bg-emerald-400/20"
                style={{ left: pos(sasaran.bawah), width: `calc(${pos(sasaran.atas)} - ${pos(sasaran.bawah)})` }}
                initial={{ opacity: 0, scaleX: 0.6 }}
                animate={{ opacity: 1, scaleX: 1 }}
                transition={{ duration: 0.8, ease: ease.garda, delay: 0.2 }}
              />
              <motion.div className="absolute top-0 -translate-x-1/2" initial={{ left: "0%" }} animate={{ left: pos(yoy) }} transition={{ ...spring.soft, delay: 0.35 }}>
                <div className="num -translate-y-1 rounded-md bg-white px-1.5 py-0.5 text-[11px] font-semibold text-navy-900 shadow-ambient">{yoy.toFixed(2).replace(".", ",")}</div>
                <div className="mx-auto mt-1 h-7 w-0.5 bg-white" />
                <div className="mx-auto -mt-1 size-3 rounded-full bg-white shadow-[0_0_0_4px_rgba(255,255,255,0.18)]" />
              </motion.div>
            </div>
            <div className="num relative mt-1 h-4 text-[11px] text-navy-300">
              {[0, 1, 2, 3, 4, 5].map((t) => (
                <span key={t} className="absolute -translate-x-1/2" style={{ left: pos(t) }}>{t}%</span>
              ))}
            </div>
          </div>

          <dl className="mt-8 grid grid-cols-3 gap-4 border-t border-white/10 pt-5">
            <div><dt className="text-xs text-navy-300">Bulanan</dt><dd className="num mt-1 text-lg font-semibold">{formatPersen(mtm, { digits: 2, signed: true })}</dd></div>
            <div><dt className="text-xs text-navy-300">Tahun kalender</dt><dd className="num mt-1 text-lg font-semibold">{formatPersen(ytd, { digits: 2 })}</dd></div>
            <div><dt className="text-xs text-navy-300">Rilis BPS</dt><dd className="mt-1 text-sm font-medium">{formatTanggal(rilis)}</dd></div>
          </dl>
        </div>

        <div className="flex flex-col gap-6">
          <div className="rounded-xl bg-white/[0.06] p-4 ring-1 ring-white/10">
            <p className="text-sm text-navy-200">12 bulan terakhir</p>
            <div className="mt-2 h-[150px]">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={seri} margin={{ top: 8, right: 8, bottom: 0, left: -18 }}>
                  <ReferenceArea y1={sasaran.bawah} y2={sasaran.atas} fill="#34d399" fillOpacity={0.1} stroke="none" />
                  <XAxis dataKey="bulan" tick={{ fontSize: 10, fill: garda.navy[300] }} axisLine={false} tickLine={false} interval={1} />
                  <YAxis domain={[1, 4]} ticks={[1.5, 2.5, 3.5]} tick={{ fontSize: 10, fill: garda.navy[300], fontFamily: "var(--font-mono)" }} axisLine={false} tickLine={false} />
                  <Tooltip content={<GlassTooltip labelFormat={(l) => String(l)} valueFormat={(v) => formatPersen(v, { digits: 2 })} />} cursor={{ stroke: "rgba(255,255,255,0.2)" }} />
                  <Line type="monotone" dataKey="yoy" name="Inflasi (yoy)" stroke={garda.data[300]} strokeWidth={2.25} dot={false} activeDot={{ r: 4, fill: "#fff" }} animationDuration={1400} />
                </LineChart>
              </ResponsiveContainer>
            </div>
          </div>
          <div>
            <p className="text-sm text-navy-200">Penyumbang utama (poin persen)</p>
            <ul className="mt-3 space-y-2">
              {penyumbang.map((p, i) => (
                <li key={p.nama} className="grid grid-cols-[110px_1fr_52px] items-center gap-3 text-sm">
                  <span className="truncate text-navy-100">{p.nama}</span>
                  <div className="relative h-2">
                    <div className="absolute inset-y-0 left-1/2 w-px bg-white/20" />
                    <motion.div
                      className={cn("absolute inset-y-0 rounded-full", p.andil >= 0 ? "left-1/2 bg-rose-300" : "right-1/2 bg-sky-300")}
                      initial={{ width: 0 }}
                      animate={{ width: `${(Math.abs(p.andil) / maxAndil) * 50}%` }}
                      transition={{ duration: 0.8, ease: ease.garda, delay: 0.4 + i * 0.06 }}
                    />
                  </div>
                  <span className="num text-right text-xs">{p.andil > 0 ? "+" : ""}{p.andil.toFixed(2).replace(".", ",")}</span>
                </li>
              ))}
            </ul>
          </div>
        </div>
      </div>
    </section>
  );
}
