"use client";

import { motion } from "motion/react";
import {
  Line,
  LineChart,
  ReferenceArea,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
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
 * Hero eksekutif: inflasi tahunan diletakkan di "koridor sasaran", dengan tren 12 bulan
 * di bawahnya supaya grafik memakai seluruh sisa tinggi panel.
 */
export function InflationHero({
  data,
}: {
  data: NonNullable<RingkasanEksekutif["inflasi"]>;
}) {
  const { yoy, mtm, ytd, sasaran, seri, periode, sumber, diimporPada } = data;
  const dalam = yoy >= sasaran.bawah && yoy <= sasaran.atas;
  // Sumbu mengikuti data dan koridor supaya garis memakai tinggi grafik dan tidak terpotong.
  const nilaiSeri = seri.map((s) => s.yoy);
  const sumbuBawah = Math.min(sasaran.bawah, ...nilaiSeri) - 0.15;
  const sumbuAtas = Math.max(sasaran.atas, ...nilaiSeri) + 0.15;

  return (
    <section className="relative overflow-hidden rounded-lg bg-gradient-to-br from-navy-800 via-navy-900 to-navy-950 p-4 text-white shadow-ambient-lg lg:p-5">
      <div
        aria-hidden
        className="animate-aurora absolute -top-24 -right-24 size-[320px] rounded-full bg-data-500/25 blur-3xl"
      />
      <div
        aria-hidden
        className="absolute inset-0 bg-[radial-gradient(60%_80%_at_0%_100%,rgba(51,194,184,0.12),transparent)]"
      />
      <div className="relative flex flex-col gap-8">
        <div className="grid shrink-0 grid-cols-1 gap-4 lg:grid-cols-[minmax(0,0.62fr)_minmax(0,1.28fr)] lg:items-center lg:gap-8">
          <div className="flex min-w-0 flex-col">
            <p className="text-sm text-navy-200">
              Inflasi tahunan Provinsi Aceh,{" "}
              {formatTanggal(periode, "MMMM yyyy")}
            </p>
            <div className="mt-2 flex items-end gap-2">
              <CountUp
                value={yoy}
                format={(n) => n.toFixed(2).replace(".", ",")}
                className="text-display-2xl font-semibold tracking-tight"
                duration={1.6}
              />
              <span className="num pb-2 text-3xl text-navy-300">%</span>
            </div>
            <span
              className={cn(
                "mt-3 inline-flex h-7 items-center gap-2 self-start rounded-full px-3 text-xs font-medium",
                dalam
                  ? "bg-emerald-400/15 text-emerald-200"
                  : "bg-rose-400/15 text-rose-200",
              )}
            >
              <span
                className={cn(
                  "size-1.5 rounded-full",
                  dalam ? "bg-emerald-300" : "bg-rose-300",
                )}
              />
              {dalam ? "Dalam koridor sasaran" : "Di luar koridor sasaran"}{" "}
              {formatPersen((sasaran.bawah + sasaran.atas) / 2)} ±{" "}
              {formatPersen((sasaran.atas - sasaran.bawah) / 2, { digits: 0 })}
            </span>
          </div>

          <div className="min-w-0">
            {/* Koridor sasaran */}
            <div
              role="img"
              aria-label={`Inflasi ${formatPersen(yoy, { digits: 2 })} pada koridor ${sasaran.bawah}–${sasaran.atas}%`}
            >
              <div className="relative h-14">
                <div className="absolute top-7 right-0 left-0 h-2 rounded-full bg-white/10" />
                <motion.div
                  className="absolute top-[22px] h-[18px] rounded-md border border-dashed border-emerald-300/60 bg-emerald-400/20"
                  style={{
                    left: pos(sasaran.bawah),
                    width: `calc(${pos(sasaran.atas)} - ${pos(sasaran.bawah)})`,
                  }}
                  initial={{ opacity: 0, scaleX: 0.6 }}
                  animate={{ opacity: 1, scaleX: 1 }}
                  transition={{ duration: 0.8, ease: ease.garda, delay: 0.2 }}
                />
                <motion.div
                  className="absolute top-0 -translate-x-1/2"
                  initial={{ left: "0%" }}
                  animate={{ left: pos(yoy) }}
                  transition={{ ...spring.soft, delay: 0.35 }}
                >
                  <div className="num -translate-y-1 rounded-md bg-white px-1.5 py-0.5 text-[11px] font-semibold text-navy-900 shadow-ambient">
                    {yoy.toFixed(2).replace(".", ",")}
                  </div>
                  <div className="mx-auto mt-1 h-7 w-0.5 bg-white" />
                  <div className="mx-auto -mt-1 size-3 rounded-full bg-white shadow-[0_0_0_4px_rgba(255,255,255,0.18)]" />
                </motion.div>
              </div>
              <div className="num relative mt-1 h-4 text-[11px] text-navy-300">
                {[0, 1, 2, 3, 4, 5].map((t) => (
                  <span
                    key={t}
                    className="absolute -translate-x-1/2"
                    style={{ left: pos(t) }}
                  >
                    {t}%
                  </span>
                ))}
              </div>
            </div>

            <dl className="mt-3 grid grid-cols-3 gap-3 border-t border-white/10 pt-3">
              <div>
                <dt className="text-xs text-navy-300">Bulanan</dt>
                <dd className="num mt-1 text-lg font-semibold">
                  {formatPersen(mtm, { digits: 2, signed: true })}
                </dd>
              </div>
              <div>
                <dt className="text-xs text-navy-300">Tahun kalender</dt>
                <dd className="num mt-1 text-lg font-semibold">
                  {formatPersen(ytd, { digits: 2 })}
                </dd>
              </div>
              <div>
                <dt className="text-xs text-navy-300">Sumber</dt>
                <dd className="mt-1 text-sm font-medium">
                  {sumber === "DEMO"
                    ? "Data demo"
                    : `${sumber}, diimpor ${formatTanggal(diimporPada, "d MMM yyyy, HH.mm")}`}
                </dd>
              </div>
            </dl>
          </div>
        </div>

        <div className="flex shrink-0 flex-col rounded-lg bg-white/[0.06] px-2.5 py-2.5 ring-1 ring-white/10 sm:px-3.5 sm:py-3.5">
          <p className="px-0.5 text-xs font-medium text-navy-200">
            12 bulan terakhir
          </p>
          <div className="mt-1 h-80 sm:h-84">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart
                data={seri}
                margin={{ top: 10, right: 10, bottom: 0, left: -20 }}
              >
                <ReferenceArea
                  y1={sasaran.bawah}
                  y2={sasaran.atas}
                  fill="#34d399"
                  fillOpacity={0.1}
                  stroke="none"
                />
                <XAxis
                  dataKey="bulan"
                  tick={{ fontSize: 10, fill: garda.navy[300] }}
                  axisLine={false}
                  tickLine={false}
                  interval="preserveStartEnd"
                  minTickGap={8}
                  padding={{ left: 6, right: 6 }}
                />
                <YAxis
                  domain={[sumbuBawah, sumbuAtas]}
                  ticks={[
                    sasaran.bawah,
                    (sasaran.bawah + sasaran.atas) / 2,
                    sasaran.atas,
                  ]}
                  tickFormatter={(v: number) => v.toFixed(1).replace(".", ",")}
                  tick={{
                    fontSize: 10,
                    fill: garda.navy[300],
                    fontFamily: "var(--font-mono)",
                  }}
                  axisLine={false}
                  tickLine={false}
                />
                <Tooltip
                  content={
                    <GlassTooltip
                      labelFormat={(l) => String(l)}
                      valueFormat={(v) => formatPersen(v, { digits: 2 })}
                    />
                  }
                  cursor={{ stroke: "rgba(255,255,255,0.2)" }}
                />
                <Line
                  type="monotone"
                  dataKey="yoy"
                  name="Inflasi (yoy)"
                  stroke={garda.data[300]}
                  strokeWidth={2.25}
                  dot={false}
                  activeDot={{ r: 4, fill: "#fff" }}
                  animationDuration={1400}
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>
    </section>
  );
}
