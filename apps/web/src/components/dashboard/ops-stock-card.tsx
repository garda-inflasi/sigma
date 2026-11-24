"use client";

import { AlertTriangle, Clock, Snowflake } from "lucide-react";
import { PanelTitle } from "@/components/garda/page-header";
import { EmptyState } from "@/components/garda/empty-state";
import { PanelGalat } from "@/components/garda/panel-galat";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import { formatAngka, formatPersen } from "@/lib/format";
import { useStokOperasional } from "@/lib/query/hooks";
import { cn } from "@/lib/utils";
import { useFilterOperasional } from "./use-filter-operasional";

/**
 * Kartu stok cold storage real-time: utilisasi kapasitas gudang, lot mendekati kedaluwarsa (<= 7 hari),
 * dan komoditas dengan risiko food loss tinggi.
 */
export function OpsStockCard({ className }: { className?: string } = {}) {
  const { saringan } = useFilterOperasional();
  const { data, isLoading, error, refetch } = useStokOperasional(saringan);
  const r = data?.ringkasan;
  const utilisasiTinggi = (r?.utilisasiPct ?? 0) >= 90;

  return (
    <section className={cn("glass flex w-full flex-col rounded-lg p-3.5 lg:max-h-full lg:min-h-0", className)}>
      <PanelTitle
        title="Stok cold storage"
        description={
          r
            ? `${formatAngka(r.totalTon, 1)} ton berlaku di ${formatAngka(r.jumlahFasilitas)} fasilitas (${formatAngka(r.jumlahLot)} lot)`
            : "Utilisasi kapasitas dan risiko food loss gudang pendingin"
        }
      />

      <div className="scrollbar-thin mt-3 lg:min-h-0 lg:overflow-y-auto">
        {isLoading ? (
          <Shimmer className="h-44 w-full" />
        ) : !data || !r ? (
          <PanelGalat
            judul="Stok cold storage gagal dimuat"
            error={error}
            onCoba={() => void refetch()}
          />
        ) : (
          <div className="space-y-3">
            <div className="rounded-lg border border-border bg-card p-3">
              <div className="flex items-center justify-between text-xs">
                <span className="flex items-center gap-1.5 font-medium text-muted-foreground">
                  <Snowflake className="size-3.5 text-data-600" />
                  Utilisasi kapasitas operasional
                </span>
                <span className="num font-semibold text-foreground">
                  {formatPersen(r.utilisasiPct)} ({formatAngka(r.totalTon, 1)} / {formatAngka(r.kapasitasTon, 0)} ton)
                </span>
              </div>
              <div className="mt-2 h-2 w-full overflow-hidden rounded-full bg-mist-100 dark:bg-white/10">
                <div
                  className={cn(
                    "h-full rounded-full transition-all",
                    utilisasiTinggi ? "bg-waspada" : "bg-data-500",
                  )}
                  style={{ width: `${Math.min(100, Math.max(0, r.utilisasiPct))}%` }}
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="rounded-lg border border-border bg-card p-3">
                <p className="flex items-center gap-1 text-[11px] font-medium text-muted-foreground">
                  <Clock className="size-3.5 text-waspada-ink" />
                  Kedaluwarsa ≤ {data.ambangKedaluwarsaHari} hari
                </p>
                <p className="num mt-1 text-base font-semibold text-foreground">
                  {formatAngka(r.mendekatiKedaluwarsaTon, 1)} ton
                </p>
              </div>
              <div className="rounded-lg border border-border bg-card p-3">
                <p className="flex items-center gap-1 text-[11px] font-medium text-muted-foreground">
                  <AlertTriangle className="size-3.5 text-defisit-ink" />
                  Risiko food loss tinggi
                </p>
                <p className="num mt-1 text-base font-semibold text-foreground">
                  {formatAngka(r.risikoTinggiTon, 1)} ton
                </p>
              </div>
            </div>

            {data.komoditas.length === 0 ? (
              <EmptyState
                title="Belum ada stok cold storage berlaku pada cakupan ini."
                className="py-6"
              />
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="text-left text-xs text-muted-foreground">
                      <th className="pb-2 font-medium">Komoditas</th>
                      <th className="pb-2 text-right font-medium">Stok</th>
                      <th className="pb-2 text-right font-medium">Peringatan</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.komoditas.slice(0, 5).map((k) => (
                      <tr key={k.komoditasKode} className="border-t border-border">
                        <td className="py-2 pr-2">
                          <p className="font-medium">{k.komoditasNama}</p>
                          <p className="text-xs text-muted-foreground">
                            {formatAngka(k.jumlahLot)} lot · {formatAngka(k.jumlahFasilitas)} fasilitas
                          </p>
                        </td>
                        <td className="num py-2 pl-2 text-right font-medium whitespace-nowrap">
                          {formatAngka(k.totalTon, 1)} ton
                        </td>
                        <td className="py-2 pl-2 text-right whitespace-nowrap">
                          {k.risikoTinggiTon > 0 ? (
                            <StatusBadge tone="defisit" dot={false}>
                              Risiko {formatAngka(k.risikoTinggiTon, 1)} t
                            </StatusBadge>
                          ) : k.mendekatiKedaluwarsaTon > 0 ? (
                            <StatusBadge tone="waspada" dot={false}>
                              Exp {formatAngka(k.mendekatiKedaluwarsaTon, 1)} t
                            </StatusBadge>
                          ) : (
                            <StatusBadge tone="normal" dot={false}>
                              Aman
                            </StatusBadge>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        )}
      </div>
    </section>
  );
}
