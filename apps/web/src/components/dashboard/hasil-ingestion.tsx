"use client";

import { CloudRain } from "lucide-react";
import { PanelTitle } from "@/components/garda/page-header";
import { EmptyState } from "@/components/garda/empty-state";
import { PanelGalat } from "@/components/garda/panel-galat";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import {
  formatAngka,
  formatPersen,
  formatRupiah,
  formatTanggal,
} from "@/lib/format";
import { useCuacaTerbaru, usePembandingHarga } from "@/lib/query/hooks";
import { cn } from "@/lib/utils";

/** Validasi silang: harga lapangan tervalidasi dibandingkan dengan harga eksternal (PIHPS) pada hari yang sama. */
export function PembandingHargaCard() {
  const { data, isLoading, error, refetch } = usePembandingHarga();
  const demo = data?.items.some((i) => i.demo) ?? false;

  return (
    <section className="glass flex w-full flex-col rounded-lg p-3.5 lg:max-h-full lg:min-h-0">
      <PanelTitle
        title="Validasi silang harga"
        description={
          data && data.pasangan > 0
            ? `${formatAngka(data.sepakat)} dari ${formatAngka(data.pasangan)} pasangan sepakat (selisih ≤ ${formatAngka(data.batasPct)}%), 7 hari terakhir${demo ? ", termasuk data contoh" : ""}`
            : "Harga lapangan tervalidasi vs harga eksternal, 7 hari terakhir"
        }
      />
      <div className="scrollbar-thin mt-2 overflow-x-auto lg:min-h-0 lg:overflow-y-auto">
        {isLoading ? (
          <Shimmer className="h-40 w-full" />
        ) : !data ? (
          <PanelGalat
            judul="Validasi silang gagal dimuat"
            error={error}
            onCoba={() => void refetch()}
          />
        ) : data.items.length === 0 ? (
          <EmptyState
            title="Belum ada pasangan harga lapangan dan eksternal pada hari yang sama."
            description="Jalankan ingestion PIHPS di halaman Sumber data."
            className="py-6"
          />
        ) : (
          <table className="w-full text-[13px]">
            <thead>
              <tr className="text-left text-xs text-muted-foreground">
                <th className="pb-2 font-medium">Komoditas, wilayah</th>
                <th className="pb-2 pl-2 text-right font-medium">Lapangan</th>
                <th className="pb-2 pl-2 text-right font-medium">Eksternal</th>
                <th className="pb-2 pl-2 text-right font-medium">Selisih</th>
              </tr>
            </thead>
            <tbody>
              {data.items.map((i) => (
                <tr
                  key={`${i.komoditasNama}:${i.kabupatenNama}`}
                  className="border-t border-border"
                >
                  <td className="py-2 pr-2">
                    <p className="font-medium">{i.komoditasNama}</p>
                    <p className="text-xs text-muted-foreground">
                      {i.kabupatenNama}, {formatTanggal(i.tanggal, "d MMM")}
                    </p>
                  </td>
                  <td className="num py-2 pl-2 text-right text-xs whitespace-nowrap">
                    {formatRupiah(i.hargaLapangan)}
                  </td>
                  <td className="num py-2 pl-2 text-right text-xs whitespace-nowrap">
                    {formatRupiah(i.hargaEksternal)}
                  </td>
                  <td className="py-2 pl-2 text-right whitespace-nowrap">
                    <StatusBadge
                      tone={i.sepakat ? "normal" : "waspada"}
                      dot={false}
                    >
                      {formatPersen(i.selisihPct, { signed: true })}
                    </StatusBadge>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </section>
  );
}

/** Cuaca harian terbaru per kabupaten/kota (BMKG); yang hujan lebat di depan. */
export function CuacaWilayahCard() {
  const { data, isLoading, error, refetch } = useCuacaTerbaru();
  const anomali = data?.filter((c) => c.anomali).length ?? 0;
  const demo = data?.some((c) => c.demo) ?? false;

  return (
    <section className="glass flex w-full flex-col rounded-lg p-3.5 lg:max-h-full lg:min-h-0">
      <PanelTitle
        title="Cuaca wilayah"
        description={
          data && data.length > 0
            ? `${formatAngka(anomali)} wilayah hujan lebat (≥ 50 mm/hari)${demo ? ", termasuk data contoh" : ""}`
            : "Prakiraan harian BMKG per kabupaten/kota"
        }
      />
      <ul className="scrollbar-thin mt-2 max-h-80 space-y-2 overflow-y-auto pr-1 lg:max-h-none lg:min-h-0">
        {isLoading ? (
          Array.from({ length: 5 }).map((_, i) => (
            <Shimmer key={i} className="h-9 w-full" />
          ))
        ) : !data ? (
          <li>
            <PanelGalat
              judul="Cuaca wilayah gagal dimuat"
              error={error}
              onCoba={() => void refetch()}
            />
          </li>
        ) : data.length === 0 ? (
          <li>
            <EmptyState
              title="Belum ada data cuaca."
              description="Jalankan ingestion BMKG di halaman Sumber data."
              className="py-6"
            />
          </li>
        ) : (
          data.map((c) => (
            <li
              key={c.kode}
              className="flex items-center justify-between gap-3 text-sm"
            >
              <span className="flex min-w-0 items-center gap-2">
                <CloudRain
                  className={cn(
                    "size-4 shrink-0",
                    c.anomali ? "text-defisit-ink dark:text-rose-300" : "text-muted-foreground",
                  )}
                />
                <span className="truncate">{c.nama}</span>
              </span>
              <span className="num shrink-0 text-xs text-muted-foreground">
                <span
                  className={cn(
                    "font-semibold",
                    c.anomali ? "text-defisit-ink dark:text-rose-300" : "text-foreground",
                  )}
                >
                  {formatAngka(c.curahHujanMm, 1)} mm
                </span>
                , {formatAngka(c.suhuC, 1)}°C, {c.kelembapan}%
              </span>
            </li>
          ))
        )}
      </ul>
    </section>
  );
}
