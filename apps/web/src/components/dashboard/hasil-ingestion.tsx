"use client";

import { CloudRain } from "lucide-react";
import { PanelTitle } from "@/components/garda/page-header";
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
  const { data, isLoading } = usePembandingHarga();
  const demo = data?.items.some((i) => i.demo) ?? false;

  return (
    <section className="glass h-full rounded-xl p-5 sm:p-6">
      <PanelTitle
        title="Validasi silang harga"
        description={
          data && data.pasangan > 0
            ? `${formatAngka(data.sepakat)} dari ${formatAngka(data.pasangan)} pasangan sepakat (selisih ≤ ${formatAngka(data.batasPct)}%), 7 hari terakhir${demo ? ", termasuk data contoh" : ""}`
            : "Harga lapangan tervalidasi vs harga eksternal, 7 hari terakhir"
        }
      />
      <div className="mt-4 overflow-x-auto">
        {isLoading ? (
          <Shimmer className="h-40 w-full" />
        ) : !data || data.items.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            Belum ada pasangan harga lapangan dan eksternal pada hari yang sama.
            Jalankan ingestion PIHPS di halaman Sumber data.
          </p>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-xs text-muted-foreground">
                <th className="pb-2 font-medium">Komoditas, wilayah</th>
                <th className="pb-2 text-right font-medium">Lapangan</th>
                <th className="pb-2 text-right font-medium">Eksternal</th>
                <th className="pb-2 text-right font-medium">Selisih</th>
              </tr>
            </thead>
            <tbody>
              {data.items.map((i) => (
                <tr
                  key={`${i.komoditasNama}:${i.kabupatenNama}`}
                  className="border-t border-border/50"
                >
                  <td className="py-2 pr-2">
                    <p className="font-medium">{i.komoditasNama}</p>
                    <p className="text-xs text-muted-foreground">
                      {i.kabupatenNama}, {formatTanggal(i.tanggal, "d MMM")}
                    </p>
                  </td>
                  <td className="num py-2 text-right">
                    {formatRupiah(i.hargaLapangan)}
                  </td>
                  <td className="num py-2 text-right">
                    {formatRupiah(i.hargaEksternal)}
                  </td>
                  <td className="py-2 text-right">
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
  const { data, isLoading } = useCuacaTerbaru();
  const anomali = data?.filter((c) => c.anomali).length ?? 0;
  const demo = data?.some((c) => c.demo) ?? false;

  return (
    <section className="glass h-full rounded-xl p-5 sm:p-6">
      <PanelTitle
        title="Cuaca wilayah"
        description={
          data && data.length > 0
            ? `${formatAngka(anomali)} wilayah hujan lebat (≥ 50 mm/hari)${demo ? ", termasuk data contoh" : ""}`
            : "Prakiraan harian BMKG per kabupaten/kota"
        }
      />
      <ul className="scrollbar-thin mt-4 max-h-80 space-y-2 overflow-y-auto pr-1">
        {isLoading ? (
          Array.from({ length: 5 }).map((_, i) => (
            <Shimmer key={i} className="h-9 w-full" />
          ))
        ) : !data || data.length === 0 ? (
          <li className="text-sm text-muted-foreground">
            Belum ada data cuaca. Jalankan ingestion BMKG di halaman Sumber
            data.
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
                    c.anomali ? "text-defisit-ink" : "text-muted-foreground",
                  )}
                />
                <span className="truncate">{c.nama}</span>
              </span>
              <span className="num shrink-0 text-xs text-muted-foreground">
                <span
                  className={cn(
                    "font-semibold",
                    c.anomali ? "text-defisit-ink" : "text-foreground",
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
