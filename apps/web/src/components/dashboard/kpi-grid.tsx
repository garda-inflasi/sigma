"use client";

import { motion } from "motion/react";
import { PanelGalat } from "@/components/garda/panel-galat";
import { KpiSkeleton } from "@/components/garda/shimmer";
import { stagger } from "@/lib/motion/presets";
import { useKpiOperasional } from "@/lib/query/hooks";
import { KpiCard } from "./kpi-card";
import { useFilterOperasional } from "./use-filter-operasional";

/**
 * Momen orkestrasi tunggal halaman: kartu KPI masuk bertahap & angka menghitung naik. Kartu bernilai nol atau bernada
 * netral (`data`) adalah data yang sah (mis. Koordinator di kabupaten tanpa cold storage), bukan galat.
 */
export function KpiGrid() {
  const { filter } = useFilterOperasional();
  const { data, isLoading, error, refetch } = useKpiOperasional(filter);
  if (isLoading) {
    return <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">{Array.from({ length: 4 }).map((_, i) => <KpiSkeleton key={i} />)}</div>;
  }
  // Tanpa data setelah selesai memuat berarti gagal (atau sedang luring): bukan kerangka yang tak berujung.
  if (!data) {
    return <div className="glass rounded-xl"><PanelGalat judul="Indikator KPI gagal dimuat" error={error} onCoba={() => void refetch()} /></div>;
  }
  if (data.length === 0) {
    return <p className="glass rounded-xl p-5 text-sm text-muted-foreground">Belum ada indikator KPI untuk ditampilkan.</p>;
  }
  return (
    <motion.div variants={stagger(0.08)} initial="hidden" animate="show" className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      {data.map((k) => <KpiCard key={k.id} kpi={k} naikBaik={k.id !== "waspada"} />)}
    </motion.div>
  );
}
