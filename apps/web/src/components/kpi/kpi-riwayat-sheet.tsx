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
import { EmptyState } from "@/components/garda/empty-state";
import {
  InspectorRingkasan,
  InspectorSection,
  MetaGrid,
  MetaItem,
} from "@/components/garda/inspector";
import { PanelGalat } from "@/components/garda/panel-galat";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import {
  Sheet,
  SheetBody,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import {
  formatKpi,
  type KpiIndikator,
  type KpiRiwayat,
  type KpiStatus,
} from "@/lib/api/kpi";
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

function keteranganTarget(kpi: KpiIndikator) {
  if (kpi.target === null) return "Target belum ditetapkan.";
  return `Target ${formatKpi(kpi.target, kpi.satuan)}${kpi.naikBaik ? "" : ", makin rendah makin baik"}.`;
}

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
  const { data, isLoading, isError, error, refetch, isFetching } =
    useKpiRiwayat(kode);

  return (
    <Sheet open={!!kode} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="sm:max-w-xl">
        <SheetHeader>
          <SheetTitle>{data ? data.kpi.nama : "Riwayat indikator"}</SheetTitle>
          <SheetDescription>
            {data?.kpi.deskripsi ||
              "Nilai berlaku tiap periode dibandingkan dengan target saat ini."}
          </SheetDescription>
        </SheetHeader>
        <SheetBody>
          {isLoading ? (
            <div
              role="status"
              aria-label="Memuat riwayat"
              className="space-y-3 px-5 py-4"
            >
              <Shimmer className="h-40 w-full rounded-lg" />
              <Shimmer className="h-32 w-full rounded-lg" />
            </div>
          ) : isError || !data ? (
            <div className="px-5 py-4">
              <PanelGalat
                judul="Riwayat indikator gagal dimuat."
                error={error}
                onCoba={() => {
                  if (!isFetching) void refetch();
                }}
              />
            </div>
          ) : data.baris.length === 0 ? (
            <>
              <InspectorRingkasan
                status={
                  <StatusBadge tone={STATUS[data.kpi.status].tone}>
                    {STATUS[data.kpi.status].label}
                  </StatusBadge>
                }
                keterangan={keteranganTarget(data.kpi)}
              />
              <EmptyState
                title="Belum ada nilai tersimpan untuk indikator ini."
                description={
                  data.kpi.otomatis
                    ? "Nilai muncul setelah data sumber masuk dan dihitung."
                    : "Butuh model atau data yang belum tersedia; dihitung di tahap berikutnya."
                }
              />
            </>
          ) : (
            <Isi riwayat={data} />
          )}
        </SheetBody>
      </SheetContent>
    </Sheet>
  );
}

function Isi({ riwayat }: { riwayat: KpiRiwayat }) {
  const { kpi, baris, ambangWaspada } = riwayat;
  const grafik = baris.map((b) => ({
    periode: formatTanggal(b.periode, "MMM yy"),
    nilai: b.nilai,
  }));
  const terbaru = [...baris].reverse();

  return (
    <>
      <InspectorRingkasan
        status={
          <StatusBadge tone={STATUS[kpi.status].tone}>
            {STATUS[kpi.status].label}
          </StatusBadge>
        }
        nilai={
          kpi.nilai === null ? undefined : formatKpi(kpi.nilai, kpi.satuan)
        }
        keterangan={`${keteranganTarget(kpi)} Status tiap periode dinilai terhadap target saat ini.`}
      />

      <InspectorSection title="Rincian" className="border-t-0">
        <MetaGrid>
          {kpi.periode && (
            <MetaItem label="Periode terbaru">
              <span className="num">
                {formatTanggal(kpi.periode, "MMMM yyyy")}
              </span>
            </MetaItem>
          )}
          <MetaItem label="Arah yang baik">
            {kpi.naikBaik
              ? "Makin tinggi makin baik"
              : "Makin rendah makin baik"}
          </MetaItem>
          {ambangWaspada !== null && (
            <MetaItem label="Batas waspada">
              <span className="num">
                {formatKpi(ambangWaspada, kpi.satuan)}
              </span>
            </MetaItem>
          )}
          {kpi.rumus && (
            <MetaItem label="Cara hitung" wide>
              {kpi.rumus}
            </MetaItem>
          )}
        </MetaGrid>
      </InspectorSection>

      {baris.length >= 2 && (
        <InspectorSection title="Tren">
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
                    <GlassTooltip
                      valueFormat={(v) => formatKpi(v, kpi.satuan)}
                    />
                  }
                />
                {kpi.target !== null && (
                  <ReferenceLine
                    y={kpi.target}
                    ifOverflow="extendDomain"
                    stroke={garda.navy[300]}
                    strokeDasharray="3 3"
                    label={{
                      value: "Target",
                      position: "insideTopRight",
                      fontSize: 10,
                      fill: "var(--muted-foreground)",
                    }}
                  />
                )}
                <Line
                  dataKey="nilai"
                  name="Realisasi"
                  stroke="var(--foreground)"
                  strokeWidth={2}
                  dot={{ r: 2 }}
                  isAnimationActive={false}
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </InspectorSection>
      )}

      <InspectorSection title="Riwayat nilai">
        <div className="overflow-x-auto rounded-lg border border-border">
          <table className="w-full text-left text-[13px]">
            <caption className="sr-only">Riwayat nilai {kpi.nama}</caption>
            <thead className="bg-secondary text-[11px] tracking-wide text-muted-foreground uppercase">
              <tr>
                <th scope="col" className="h-9 px-3 font-medium">
                  Periode
                </th>
                <th scope="col" className="h-9 px-3 text-right font-medium">
                  Realisasi
                </th>
                <th scope="col" className="h-9 px-3 text-right font-medium">
                  Target
                </th>
                <th scope="col" className="h-9 px-3 font-medium">
                  Status
                </th>
              </tr>
            </thead>
            <tbody>
              {terbaru.map((b) => {
                const status = STATUS[b.status];
                return (
                  <tr
                    key={b.periode}
                    className="border-t border-border hover:bg-secondary/60"
                  >
                    <td className="num h-11 px-3">
                      {formatTanggal(b.periode, "MMMM yyyy")}
                      {b.versi > 1 && (
                        <span className="ml-1 text-[11px] text-muted-foreground">
                          revisi {b.versi}
                        </span>
                      )}
                    </td>
                    <td className="num h-11 px-3 text-right font-medium">
                      {formatKpi(b.nilai, kpi.satuan)}
                    </td>
                    <td className="num h-11 px-3 text-right text-muted-foreground">
                      {kpi.target === null
                        ? "-"
                        : formatKpi(kpi.target, kpi.satuan)}
                    </td>
                    <td className="h-11 px-3">
                      <StatusBadge tone={status.tone}>
                        {status.label}
                      </StatusBadge>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </InspectorSection>
    </>
  );
}
