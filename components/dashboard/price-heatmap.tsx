"use client";

import { motion } from "motion/react";
import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";
import { KomoditasIcon } from "@/components/garda/komoditas-icon";
import { PanelTitle } from "@/components/garda/page-header";
import { Segmented } from "@/components/garda/segmented";
import { Shimmer } from "@/components/garda/shimmer";
import type { SelHeatmap } from "@/lib/data/types";
import { heatTone } from "@/lib/design/tokens";
import { formatPersen, formatRupiah } from "@/lib/format";
import { heatCell } from "@/lib/motion/presets";
import { useHeatmap } from "@/lib/query/hooks";
import { komoditasByKode, kabupatenByKode } from "@/lib/data/reference";

const LEGENDA = [-10, -5, 0, 5, 9, 14].map((v) => heatTone(v));

/** Heatmap Δ% harga konsumen (komoditas × wilayah) vs rata-rata 7 hari. */
export function PriceHeatmap() {
  const { data, isLoading } = useHeatmap();
  const router = useRouter();
  const [mode, setMode] = useState<"semua" | "pilot">("semua");
  const [hover, setHover] = useState<SelHeatmap | null>(null);

  const index = useMemo(() => new Map(data?.sel.map((s) => [`${s.komoditasKode}:${s.kabupatenKode}`, s])), [data]);
  const komoditas = data?.komoditas.filter((k) => mode === "semua" || k.pilot) ?? [];
  const kab = data?.kabupaten ?? [];

  return (
    <section className="glass rounded-xl p-5 sm:p-6">
      <PanelTitle
        title="Peta panas harga"
        description="Perubahan harga konsumen terhadap rata-rata 7 hari"
        actions={<Segmented layoutId="heat-mode" size="sm" value={mode} onChange={setMode} options={[{ value: "semua", label: "Semua" }, { value: "pilot", label: "Pilot" }]} />}
      />
      <p className="mt-4 min-h-5 text-sm" aria-live="polite">
        {hover ? (
          <>
            <span className="font-medium">{komoditasByKode[hover.komoditasKode].nama}</span> di {kabupatenByKode[hover.kabupatenKode].nama}:{" "}
            <span className="num">{formatRupiah(hover.harga)}</span>/{komoditasByKode[hover.komoditasKode].satuan},{" "}
            <span className="num font-semibold" style={{ color: heatTone(hover.delta).ink }}>{formatPersen(hover.delta, { signed: true })}</span>
          </>
        ) : (
          <span className="text-muted-foreground">Arahkan kursor atau fokus ke sel untuk melihat detail. Klik untuk membuka prediksi.</span>
        )}
      </p>

      <div className="scrollbar-thin mt-3 overflow-x-auto pb-1">
        {isLoading ? (
          <Shimmer className="h-[420px] w-full rounded-xl" />
        ) : (
          <motion.div
            initial="hidden"
            animate="show"
            className="grid min-w-[760px] gap-1"
            style={{ gridTemplateColumns: `minmax(170px,1.6fr) repeat(${kab.length}, minmax(52px,1fr))` }}
            onMouseLeave={() => setHover(null)}
          >
            <div />
            {kab.map((k) => (
              <div key={k.kode} className="truncate px-1 pb-1 text-center text-[11px] text-muted-foreground" title={k.nama}>{k.nama}</div>
            ))}
            {komoditas.map((kom, r) => (
              <div key={kom.kode} className="contents">
                <div className="flex items-center gap-2 pr-2 text-sm">
                  <KomoditasIcon kode={kom.kode} className="text-muted-foreground" />
                  <span className="truncate">{kom.nama}</span>
                  {kom.pilot && <span className="rounded-md bg-navy-50 px-1.5 text-[10px] font-medium text-navy-600 dark:bg-white/10 dark:text-navy-200">Pilot</span>}
                </div>
                {kab.map((k, c) => {
                  const s = index.get(`${kom.kode}:${k.kode}`);
                  if (!s) return <div key={k.kode} className="h-9 rounded-lg bg-mist-100 dark:bg-white/5" />;
                  const t = heatTone(s.delta);
                  return (
                    <motion.button
                      key={k.kode}
                      custom={r + c}
                      variants={heatCell}
                      onMouseEnter={() => setHover(s)}
                      onFocus={() => setHover(s)}
                      onClick={() => router.push(`/prediksi?komoditas=${kom.kode}&wilayah=${k.kode}`)}
                      whileHover={{ scale: 1.08, zIndex: 2 }}
                      aria-label={`${kom.nama}, ${k.nama}: ${formatPersen(s.delta, { signed: true })}`}
                      className="num relative h-9 rounded-lg text-[11px] font-medium ring-white/70 transition-shadow hover:shadow-ambient hover:ring-2 dark:ring-white/20"
                      style={{ background: t.fill, color: t.ink }}
                    >
                      {s.delta > 0 ? "+" : ""}{s.delta.toFixed(1).replace(".", ",")}
                    </motion.button>
                  );
                })}
              </div>
            ))}
          </motion.div>
        )}
      </div>

      <div className="mt-4 flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-muted-foreground">
        {LEGENDA.map((l) => (
          <span key={l.label} className="inline-flex items-center gap-1.5">
            <span className="size-3 rounded" style={{ background: l.fill }} /> {l.label}
          </span>
        ))}
      </div>
    </section>
  );
}
