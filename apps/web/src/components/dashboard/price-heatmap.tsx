"use client";

import { motion } from "motion/react";
import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";
import { KomoditasIcon } from "@/components/garda/komoditas-icon";
import { PanelTitle } from "@/components/garda/page-header";
import { EmptyState } from "@/components/garda/empty-state";
import { PanelGalat } from "@/components/garda/panel-galat";
import { Segmented } from "@/components/garda/segmented";
import { Shimmer } from "@/components/garda/shimmer";
import type { SelHeatmap } from "@/lib/data/types";
import { heatTone } from "@/lib/design/tokens";
import { formatPersen, formatRupiah } from "@/lib/format";
import { heatCell } from "@/lib/motion/presets";
import { useHeatmap } from "@/lib/query/hooks";
import { komoditasByKode, kabupatenByKode } from "@/lib/data/reference";
import { cn } from "@/lib/utils";
import { useFilterOperasional } from "./use-filter-operasional";

const LEGENDA = [-10, -5, 0, 5, 9, 14].map((v) => heatTone(v));

/**
 * Heatmap Δ% harga konsumen (komoditas × wilayah) vs rata-rata 7 hari. Server hanya mengirim kabupaten dan komoditas
 * yang punya sel, jadi cakupan tanpa data (mis. Koordinator di kabupaten yang belum punya harga tervalidasi) berupa
 * daftar kosong: itu keadaan kosong, bukan galat, dan tidak boleh menyisakan grid tanpa kolom.
 */
export function PriceHeatmap({ className }: { className?: string } = {}) {
  const { filter } = useFilterOperasional();
  const { data, isLoading, error, refetch } = useHeatmap(filter);
  const router = useRouter();
  const [mode, setMode] = useState<"semua" | "pilot">("semua");
  const [hover, setHover] = useState<SelHeatmap | null>(null);

  const index = useMemo(() => new Map(data?.sel.map((s) => [`${s.komoditasKode}:${s.kabupatenKode}`, s])), [data]);
  const komoditas = data?.komoditas.filter((k) => mode === "semua" || k.pilot) ?? [];
  const kab = data?.kabupaten ?? [];

  return (
    <section className={cn("glass flex w-full min-w-0 flex-col rounded-lg p-3.5 lg:max-h-full lg:min-h-0", className)}>
      <PanelTitle
        title="Peta panas harga"
        description="Perubahan harga konsumen terhadap rata-rata 7 hari"
        actions={<Segmented layoutId="heat-mode" size="sm" value={mode} onChange={setMode} options={[{ value: "semua", label: "Semua" }, { value: "pilot", label: "Pilot" }]} />}
      />
      <p className="mt-2 min-h-5 shrink-0 truncate text-[13px]" aria-live="polite">
        {hover ? (
          <>
            <span className="font-medium">{komoditasByKode[hover.komoditasKode]?.nama ?? hover.komoditasKode}</span> di {kabupatenByKode[hover.kabupatenKode]?.nama ?? hover.kabupatenKode}:{" "}
            <span className="num">{formatRupiah(hover.harga)}</span>/{komoditasByKode[hover.komoditasKode]?.satuan ?? "kg"},{" "}
            <span className="num font-semibold" style={{ color: heatTone(hover.delta).ink }}>{formatPersen(hover.delta, { signed: true })}</span>
            {hover.sumber === "EKSTERNAL" && <span className="text-muted-foreground"> (harga eksternal, belum ada data lapangan)</span>}
          </>
        ) : (
          <span className="text-muted-foreground">Arahkan kursor atau fokus ke sel untuk melihat detail. Klik untuk membuka prediksi.</span>
        )}
      </p>

      <div className="scrollbar-thin mt-2 overflow-x-auto pb-1 lg:min-h-0 lg:overflow-auto">
        {isLoading ? (
          <Shimmer className="h-64 w-full rounded-lg" />
        ) : !data ? (
          <PanelGalat judul="Peta panas harga gagal dimuat" error={error} onCoba={() => void refetch()} className="h-64" />
        ) : kab.length === 0 || data.komoditas.length === 0 ? (
          <EmptyState title="Belum ada harga tervalidasi untuk dipetakan di wilayah ini." className="h-64" />
        ) : komoditas.length === 0 ? (
          <EmptyState title="Tidak ada komoditas pilot pada data ini." className="h-64" />
        ) : (
          <motion.div
            initial="hidden"
            animate="show"
            className="grid min-w-[640px] gap-1"
            style={{ gridTemplateColumns: `minmax(180px,1.6fr) repeat(${kab.length}, minmax(46px,1fr))` }}
            onMouseLeave={() => setHover(null)}
          >
            <div className="sticky top-0 left-0 z-30 bg-card" />
            {kab.map((k) => (
              <div key={k.kode} className="sticky top-0 z-20 truncate bg-card px-1 pb-1 text-center text-[11px] text-muted-foreground" title={k.nama}>{k.nama}</div>
            ))}
            {komoditas.map((kom, r) => (
              <div key={kom.kode} className="contents">
                <div className="sticky left-0 z-10 flex min-w-0 items-center gap-1.5 bg-card pr-2 text-[13px]">
                  <KomoditasIcon kode={kom.kode} className="shrink-0 text-muted-foreground" />
                  <span className="min-w-0 truncate" title={kom.nama}>{kom.nama}</span>
                  {kom.pilot && <span className="shrink-0 rounded-md bg-navy-50 px-1 text-[10px] font-medium text-navy-600 dark:bg-white/10 dark:text-navy-200">Pilot</span>}
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
                      aria-label={`${kom.nama}, ${k.nama}: ${formatPersen(s.delta, { signed: true })}${s.sumber === "EKSTERNAL" ? " (harga eksternal)" : ""}`}
                      className={cn("num relative h-9 rounded-lg text-[11px] font-medium ring-white/70 transition-shadow hover:shadow-ambient hover:ring-2 dark:ring-white/20", s.sumber === "EKSTERNAL" && "outline-dashed outline-1 -outline-offset-2 outline-current")}
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

      <div className="mt-2 flex shrink-0 flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-muted-foreground">
        {LEGENDA.map((l) => (
          <span key={l.label} className="inline-flex items-center gap-1.5">
            <span className="size-3 rounded" style={{ background: l.fill }} /> {l.label}
          </span>
        ))}
      </div>
    </section>
  );
}
