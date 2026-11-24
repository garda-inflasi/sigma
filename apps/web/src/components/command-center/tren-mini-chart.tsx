"use client";

import {
  Area,
  CartesianGrid,
  ComposedChart,
  Line,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { GlassTooltip } from "@/components/charts/glass-tooltip";
import { EmptyState } from "@/components/garda/empty-state";
import { Shimmer } from "@/components/garda/shimmer";
import { garda } from "@/lib/design/tokens";
import { formatRupiah, formatTanggal } from "@/lib/format";

export interface TitikTrenHarga {
  tanggal: string;
  konsumen: number;
  produsen: number;
}

export default function TrenMiniChart({
  data,
  isLoading,
  satuan = "kg",
}: {
  data?: TitikTrenHarga[];
  isLoading: boolean;
  satuan?: string;
}) {
  if (isLoading && !data) {
    return <Shimmer className="h-36 w-full rounded-lg" />;
  }
  if (!data || data.length === 0) {
    return (
      <EmptyState
        title="Belum ada data tren tervalidasi"
        description="Pilih kabupaten atau komoditas lain."
        className="py-6"
      />
    );
  }

  return (
    <div className="h-36 w-full">
      <ResponsiveContainer width="100%" height="100%">
        <ComposedChart data={data} margin={{ top: 6, right: 6, bottom: 0, left: 0 }}>
          <defs>
            <linearGradient id="cc-g-konsumen" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={garda.navy[600]} stopOpacity={0.24} />
              <stop offset="100%" stopColor={garda.navy[600]} stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid vertical={false} stroke="var(--border)" strokeDasharray="3 3" />
          <XAxis
            dataKey="tanggal"
            tickFormatter={(v: string) => formatTanggal(v, "d MMM")}
            tick={{ fontSize: 10, fill: "var(--muted-foreground)" }}
            axisLine={false}
            tickLine={false}
            minTickGap={28}
          />
          <YAxis
            width={52}
            tickFormatter={(v: number) => `${Math.round(v / 1000)}rb`}
            tick={{ fontSize: 10, fill: "var(--muted-foreground)" }}
            axisLine={false}
            tickLine={false}
            domain={["auto", "auto"]}
          />
          <Tooltip
            content={
              <GlassTooltip
                labelFormat={(l) => formatTanggal(String(l), "d MMM yyyy")}
                valueFormat={(v) => `${formatRupiah(v)}/${satuan}`}
              />
            }
          />
          <Area
            type="monotone"
            dataKey="konsumen"
            name="Konsumen"
            stroke={garda.navy[600]}
            strokeWidth={2}
            fill="url(#cc-g-konsumen)"
            isAnimationActive={false}
          />
          <Line
            type="monotone"
            dataKey="produsen"
            name="Produsen"
            stroke={garda.data[500]}
            strokeWidth={1.75}
            dot={false}
            isAnimationActive={false}
          />
        </ComposedChart>
      </ResponsiveContainer>
    </div>
  );
}
