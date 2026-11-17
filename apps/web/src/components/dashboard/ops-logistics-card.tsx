"use client";

import { ArrowDownToLine, ArrowLeftRight, ArrowUpFromLine, Truck } from "lucide-react";
import { PanelTitle } from "@/components/garda/page-header";
import { PanelGalat } from "@/components/garda/panel-galat";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import { formatAngka, formatTanggal } from "@/lib/format";
import { useLogistikOperasional } from "@/lib/query/hooks";
import { cn } from "@/lib/utils";
import { useFilterOperasional } from "./use-filter-operasional";

/**
 * Kartu pergerakan logistik 7 hari terakhir (masuk/keluar/pindah) dan daftar rute distribusi pangan aktif.
 */
export function OpsLogisticsCard({ className }: { className?: string } = {}) {
  const { saringan } = useFilterOperasional();
  const { data, isLoading, error, refetch } = useLogistikOperasional(saringan);
  const ringkas = data?.ringkasan;

  return (
    <section className={cn("glass flex h-full flex-col rounded-xl p-5 sm:p-6", className)}>
      <PanelTitle
        title="Pergerakan logistik & rute aktif"
        description={
          data
            ? `${formatTanggal(data.dari, "d MMM")} – ${formatTanggal(data.sampai, "d MMM")} · ${formatAngka(data.rute.jumlah)} rute aktif (${formatAngka(data.rute.volumeTon, 1)} ton)`
            : "Ringkasan arus stok 7 hari terakhir dan rute distribusi aktif"
        }
      />

      <div className="mt-4 flex-1">
        {isLoading ? (
          <Shimmer className="h-44 w-full" />
        ) : !data || !ringkas ? (
          <PanelGalat
            judul="Pergerakan logistik gagal dimuat"
            error={error}
            onCoba={() => void refetch()}
          />
        ) : (
          <div className="space-y-4">
            <div className="grid grid-cols-3 gap-2.5">
              <div className="rounded-lg border border-border/60 bg-white/40 p-2.5 dark:bg-white/5">
                <p className="flex items-center gap-1 text-[11px] font-medium text-muted-foreground">
                  <ArrowDownToLine className="size-3.5 text-normal" />
                  Masuk
                </p>
                <p className="num mt-1 text-sm font-semibold text-foreground">
                  {formatAngka(ringkas.masuk.volumeTon, 1)} t
                </p>
                <p className="text-[11px] text-muted-foreground">
                  {formatAngka(ringkas.masuk.jumlah)} transaksi
                </p>
              </div>
              <div className="rounded-lg border border-border/60 bg-white/40 p-2.5 dark:bg-white/5">
                <p className="flex items-center gap-1 text-[11px] font-medium text-muted-foreground">
                  <ArrowUpFromLine className="size-3.5 text-waspada-ink" />
                  Keluar
                </p>
                <p className="num mt-1 text-sm font-semibold text-foreground">
                  {formatAngka(ringkas.keluar.volumeTon, 1)} t
                </p>
                <p className="text-[11px] text-muted-foreground">
                  {formatAngka(ringkas.keluar.jumlah)} transaksi
                </p>
              </div>
              <div className="rounded-lg border border-border/60 bg-white/40 p-2.5 dark:bg-white/5">
                <p className="flex items-center gap-1 text-[11px] font-medium text-muted-foreground">
                  <ArrowLeftRight className="size-3.5 text-data-600" />
                  Pindah
                </p>
                <p className="num mt-1 text-sm font-semibold text-foreground">
                  {formatAngka(ringkas.pindah.volumeTon, 1)} t
                </p>
                <p className="text-[11px] text-muted-foreground">
                  {formatAngka(ringkas.pindah.jumlah)} transaksi
                </p>
              </div>
            </div>

            {data.rute.items.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                Belum ada rute distribusi aktif pada cakupan ini.
              </p>
            ) : (
              <div className="space-y-2">
                <p className="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
                  <Truck className="size-3.5" /> Rute distribusi aktif
                </p>
                <ul className="divide-y divide-border/50 rounded-lg border border-border/50 bg-white/30 dark:bg-white/[0.03]">
                  {data.rute.items.slice(0, 5).map((r) => (
                    <li key={r.id} className="flex items-center justify-between gap-2 px-3 py-2 text-xs">
                      <div className="min-w-0">
                        <p className="truncate font-medium text-foreground">
                          {r.asalNama} → {r.tujuanNama}
                        </p>
                        <p className="truncate text-[11px] text-muted-foreground">
                          {r.komoditasNama} · {formatAngka(r.jarakKm, 0)} km
                        </p>
                      </div>
                      <div className="flex shrink-0 items-center gap-2">
                        <span className="num font-semibold">{formatAngka(r.volumeTon, 1)} t</span>
                        <StatusBadge tone="data" dot={false}>
                          {r.status}
                        </StatusBadge>
                      </div>
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        )}
      </div>
    </section>
  );
}
