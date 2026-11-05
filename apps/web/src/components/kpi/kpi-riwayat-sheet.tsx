"use client";

import {
  CartesianGrid,
  Line,
  LineChart,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { GlassTooltip } from "@/components/charts/glass-tooltip";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { formatKpi, type KpiRiwayat, type KpiStatus } from "@/lib/api/kpi";
import { garda } from "@/lib/design/tokens";
import { formatTanggal } from "@/lib/format";
import { useKpiRiwayat } from "@/lib/query/hooks";

const STATUS: Record<
  KpiStatus,
  { label: string; tone: "normal" | "waspada" | "defisit" | "netral" }
> = {
  TERCAPAI: { label: "Tercapai", tone: "normal" },
  WASPADA: { label: "Waspada", tone: "waspada" },
  TIDAK_TERCAPAI: { label: "Belum tercapai", tone: "defisit" },
  TANPA_TARGET: { label: "Tanpa target", tone: "netral" },
  MENUNGGU_DATA: { label: "Menunggu data", tone: "netral" },
};

/**
 * Lembar detail satu indikator: nilai berlaku tiap periode terhadap target saat ini.
 * `kode` kosong = tertutup; permintaan riwayat baru dikirim saat lembar dibuka.
 */
export function KpiRiwayatSheet({
  kode,
  onClose,
}: {
  kode: string | undefined;
  onClose: () => void;
}) {
  const { data, isLoading, isError, refetch, isFetching } = useKpiRiwayat(kode);

  return (
    <Sheet open={!!kode} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="glass-strong w-full gap-0 overflow-y-auto sm:max-w-xl">
        <SheetHeader>
          <SheetTitle>{data ? data.kpi.nama : "Riwayat indikator"}</SheetTitle>
          <SheetDescription>
            {data?.kpi.deskripsi ||
              "Nilai berlaku tiap periode dibandingkan dengan target saat ini."}
          </SheetDescription>
        </SheetHeader>
        <div className="space-y-5 px-4 pb-6 text-sm">
          {isLoading ? (
            <div aria-label="Memuat riwayat" className="space-y-3">
              <Shimmer className="h-40 w-full rounded-xl" />
              <Shimmer className="h-32 w-full rounded-xl" />
            </div>
          ) : isError || !data ? (
            <div role="alert" className="space-y-2 text-defisit-ink">
              <p>Riwayat indikator gagal dimuat.</p>
              <button
                type="button"
                onClick={() => refetch()}
                disabled={isFetching}
                className="text-sm font-medium underline disabled:opacity-60"
              >
                Coba lagi
              </button>
            </div>
          ) : data.baris.length === 0 ? (
            <p className="text-muted-foreground">
              Belum ada nilai tersimpan untuk indikator ini.{" "}
              {data.kpi.otomatis
                ? "Nilai muncul setelah data sumber masuk dan dihitung."
                : "Butuh model atau data yang belum tersedia; dihitung di tahap berikutnya."}
            </p>
          ) : (
            <Isi riwayat={data} />
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}

function Isi({ riwayat }: { riwayat: KpiRiwayat }) {
  const { kpi, baris } = riwayat;
  const grafik = baris.map((b) => ({
    periode: formatTanggal(b.periode, "MMM yy"),
    nilai: b.nilai,
  }));
  const terbaru = [...baris].reverse();

  return (
    <>
      <p className="text-muted-foreground">
        {kpi.target === null ? (
          "Target belum ditetapkan."
        ) : (
          <>
            Target{" "}
            <span className="num">{formatKpi(kpi.target, kpi.satuan)}</span>
            {kpi.naikBaik ? "" : ", makin rendah makin baik"}. Status tiap
            periode dinilai terhadap target saat ini.
          </>
        )}
      </p>

      {baris.length >= 2 && (
        <div className="h-44" aria-hidden="true">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart
              data={grafik}
              margin={{ top: 8, right: 8, bottom: 0, left: 0 }}
            >
              <CartesianGrid vertical={false} stroke="var(--border)" />
              <XAxis
                dataKey="periode"
                tick={{ fontSize: 11, fill: "var(--muted-foreground)" }}
                axisLine={false}
                tickLine={false}
                minTickGap={24}
              />
              <YAxis
                tick={{
                  fontSize: 11,
                  fill: "var(--muted-foreground)",
                  fontFamily: "var(--font-mono)",
                }}
                axisLine={false}
                tickLine={false}
                width={40}
                domain={["auto", "auto"]}
              />
              <Tooltip
                content={
                  <GlassTooltip valueFormat={(v) => formatKpi(v, kpi.satuan)} />
                }
              />
              {kpi.target !== null && (
                <ReferenceLine
                  y={kpi.target}
                  stroke={garda.navy[300]}
                  strokeDasharray="3 3"
                  label={{
                    value: "Target",
                    position: "insideTopRight",
                    fontSize: 10,
                    fill: garda.navy[500],
                  }}
                />
              )}
              <Line
                dataKey="nilai"
                name="Realisasi"
                stroke={garda.navy[700]}
                strokeWidth={2}
                dot={{ r: 2 }}
                isAnimationActive={false}
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}

      <div className="overflow-x-auto rounded-lg border border-border">
        <table className="w-full text-left text-xs">
          <caption className="sr-only">Riwayat nilai {kpi.nama}</caption>
          <thead className="bg-secondary/60 text-muted-foreground">
            <tr>
              <th scope="col" className="px-2 py-1.5 font-medium">
                Periode
              </th>
              <th scope="col" className="px-2 py-1.5 text-right font-medium">
                Realisasi
              </th>
              <th scope="col" className="px-2 py-1.5 text-right font-medium">
                Target
              </th>
              <th scope="col" className="px-2 py-1.5 font-medium">
                Status
              </th>
            </tr>
          </thead>
          <tbody>
            {terbaru.map((b) => {
              const status = STATUS[b.status];
              return (
                <tr key={b.periode} className="border-t border-border">
                  <td className="px-2 py-1.5">
                    {formatTanggal(b.periode, "MMMM yyyy")}
                    {b.versi > 1 && (
                      <span className="ml-1 text-[10px] text-muted-foreground">
                        revisi {b.versi}
                      </span>
                    )}
                  </td>
                  <td className="num px-2 py-1.5 text-right font-medium">
                    {formatKpi(b.nilai, kpi.satuan)}
                  </td>
                  <td className="num px-2 py-1.5 text-right text-muted-foreground">
                    {kpi.target === null
                      ? "-"
                      : formatKpi(kpi.target, kpi.satuan)}
                  </td>
                  <td className="px-2 py-1.5">
                    <StatusBadge tone={status.tone}>{status.label}</StatusBadge>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </>
  );
}
