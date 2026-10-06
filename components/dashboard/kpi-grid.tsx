"use client";

import { motion } from "motion/react";
import { KpiSkeleton } from "@/components/garda/shimmer";
import { stagger } from "@/lib/motion/presets";
import { useKpiOperasional } from "@/lib/query/hooks";
import { KpiCard } from "./kpi-card";

/** Momen orkestrasi tunggal halaman: kartu KPI masuk bertahap & angka menghitung naik. */
export function KpiGrid() {
  const { data, isLoading } = useKpiOperasional();
  if (isLoading || !data) {
    return <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">{Array.from({ length: 4 }).map((_, i) => <KpiSkeleton key={i} />)}</div>;
  }
  return (
    <motion.div variants={stagger(0.08)} initial="hidden" animate="show" className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      {data.map((k) => <KpiCard key={k.id} kpi={k} naikBaik={k.id !== "waspada"} />)}
    </motion.div>
  );
}
