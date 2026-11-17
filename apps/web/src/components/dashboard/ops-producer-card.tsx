"use client";

import { PanelTitle } from "@/components/garda/page-header";
import { PanelGalat } from "@/components/garda/panel-galat";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import { formatAngka, formatPersen, formatRupiah, formatTanggal } from "@/lib/format";
import { useProdusenOperasional } from "@/lib/query/hooks";
import { cn } from "@/lib/utils";
import { useFilterOperasional } from "./use-filter-operasional";

/**
 * Kartu harga produsen vs konsumen & margin distribusi pangan pokok.
 */
export function OpsProducerCard({ className }: { className?: string } = {}) {
  const { saringan } = useFilterOperasional();
  const { data, isLoading, error, refetch } = useProdusenOperasional(saringan);

  return (
    <section className={cn("glass flex h-full flex-col rounded-xl p-5 sm:p-6", className)}>
      <PanelTitle
        title="Harga produsen & margin"
        description={
          data
            ? `Rata-rata tertimbang ${formatTanggal(data.dari, "d MMM")} – ${formatTanggal(data.sampai, "d MMM")}`
            : "Perbandingan harga tingkat produsen vs konsumen dan persentase margin"
        }
      />

      <div className="mt-4 flex-1 overflow-x-auto">
        {isLoading ? (
          <Shimmer className="h-44 w-full" />
        ) : !data ? (
          <PanelGalat
            judul="Harga produsen & margin gagal dimuat"
            error={error}
            onCoba={() => void refetch()}
          />
        ) : data.komoditas.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            Belum ada data harga produsen atau produksi pada cakupan ini.
          </p>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-xs text-muted-foreground">
                <th className="pb-2 font-medium">Komoditas</th>
                <th className="pb-2 text-right font-medium">Produsen</th>
                <th className="pb-2 text-right font-medium">Konsumen</th>
                <th className="pb-2 text-right font-medium">Margin</th>
              </tr>
            </thead>
            <tbody>
              {data.komoditas.slice(0, 6).map((k) => {
                const marginPct = k.margin?.persenKonsumen;
                const marginTinggi = marginPct !== undefined && marginPct > 30;
                return (
                  <tr key={k.komoditasKode} className="border-t border-border/50">
                    <td className="py-2 pr-2">
                      <p className="font-medium">{k.komoditasNama}</p>
                      <p className="text-xs text-muted-foreground">
                        {k.produksi
                          ? `Produksi ${formatAngka(k.produksi.ton, 0)} ton${k.produksi.demo ? " (contoh)" : ""}`
                          : `Satuan /${k.satuan}`}
                      </p>
                    </td>
                    <td className="num py-2 text-right">
                      {k.produsen ? formatRupiah(k.produsen.harga) : "—"}
                    </td>
                    <td className="num py-2 text-right">
                      {k.konsumen ? formatRupiah(k.konsumen.harga) : "—"}
                    </td>
                    <td className="py-2 text-right">
                      {k.margin ? (
                        <div className="inline-flex flex-col items-end gap-0.5">
                          <StatusBadge
                            tone={marginTinggi ? "waspada" : "normal"}
                            dot={false}
                          >
                            {formatPersen(k.margin.persenKonsumen)}
                          </StatusBadge>
                          <span className="num text-[11px] text-muted-foreground">
                            {k.margin.selisih >= 0 ? "+" : ""}
                            {formatRupiah(k.margin.selisih)}
                          </span>
                        </div>
                      ) : (
                        <span className="text-xs text-muted-foreground">—</span>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>
    </section>
  );
}
