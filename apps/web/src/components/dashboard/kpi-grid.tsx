"use client";

import { EmptyState } from "@/components/garda/empty-state";
import { RingkasanKeputusan } from "@/components/garda/halaman-pas";
import { PanelGalat } from "@/components/garda/panel-galat";
import { Shimmer } from "@/components/garda/shimmer";
import { useKpiOperasional } from "@/lib/query/hooks";
import { itemKpi } from "./kpi-card";
import { useFilterOperasional } from "./use-filter-operasional";

/**
 * Baris ringkasan KPI operasional. Kartu bernilai nol atau bernada netral (`data`) adalah data yang sah
 * (mis. Koordinator di kabupaten tanpa cold storage), bukan galat.
 */
export function KpiGrid() {
  const { filter } = useFilterOperasional();
  const { data, isLoading, error, refetch } = useKpiOperasional(filter);
  if (isLoading) {
    return (
      <div className="grid shrink-0 grid-cols-2 gap-2 lg:grid-cols-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <Shimmer key={i} className="h-[4.75rem] rounded-lg" />
        ))}
      </div>
    );
  }
  // Tanpa data setelah selesai memuat berarti gagal (atau sedang luring): bukan kerangka yang tak berujung.
  if (!data) {
    return (
      <div className="glass shrink-0 rounded-lg">
        <PanelGalat
          judul="Indikator KPI gagal dimuat"
          error={error}
          onCoba={() => void refetch()}
        />
      </div>
    );
  }
  if (data.length === 0) {
    return (
      <div className="glass shrink-0 rounded-lg">
        <EmptyState title="Belum ada indikator KPI untuk ditampilkan." />
      </div>
    );
  }
  return (
    <RingkasanKeputusan
      label="Indikator operasional"
      items={data.map((k) => itemKpi(k, k.id !== "waspada"))}
    />
  );
}
