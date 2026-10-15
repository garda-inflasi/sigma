"use client";

import { Area, CartesianGrid, ComposedChart, Line, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { PanelTitle } from "@/components/garda/page-header";
import { Shimmer } from "@/components/garda/shimmer";
import { garda } from "@/lib/design/tokens";
import { KOMODITAS, kabupatenByKode } from "@/lib/data/reference";
import { kabupatenBerpasar } from "@/lib/data/generators";
import { formatRupiah, formatTanggal } from "@/lib/format";
import { useTrenHarga } from "@/lib/query/hooks";
import { useState } from "react";
import { GlassTooltip } from "./glass-tooltip";

/** Tren harga produsen vs konsumen — area & garis tergambar saat dimuat. */
export function TrendChart() {
  const [kode, setKode] = useState("CBM-KRT");
  const [kab, setKab] = useState("1171");
  const { data, isLoading } = useTrenHarga(kode, kab);
  const kom = KOMODITAS.find((k) => k.kode === kode)!;

  return (
    <section className="glass flex h-full flex-col rounded-xl p-5 sm:p-6">
      <PanelTitle title="Tren harga 45 hari" description={`Produsen dan konsumen, ${kabupatenByKode[kab].nama}`} />
      <div className="mt-3 flex flex-wrap gap-2">
        <Select value={kode} onValueChange={setKode}>
          <SelectTrigger className="h-9 w-[200px] rounded-xl bg-white/70 dark:bg-white/5"><SelectValue /></SelectTrigger>
          <SelectContent>{KOMODITAS.map((k) => <SelectItem key={k.kode} value={k.kode}>{k.nama}</SelectItem>)}</SelectContent>
        </Select>
        <Select value={kab} onValueChange={setKab}>
          <SelectTrigger className="h-9 w-[170px] rounded-xl bg-white/70 dark:bg-white/5"><SelectValue /></SelectTrigger>
          <SelectContent>{kabupatenBerpasar().map((k) => <SelectItem key={k.kode} value={k.kode}>{k.nama}</SelectItem>)}</SelectContent>
        </Select>
      </div>
      <div className="mt-4 min-h-[240px] flex-1">
        {isLoading && !data ? (
          <Shimmer className="h-[240px] w-full rounded-xl" />
        ) : (
          <ResponsiveContainer width="100%" height={260}>
            <ComposedChart data={data} margin={{ top: 8, right: 4, bottom: 0, left: 0 }}>
              <defs>
                <linearGradient id="g-konsumen" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor={garda.navy[600]} stopOpacity={0.22} />
                  <stop offset="100%" stopColor={garda.navy[600]} stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid vertical={false} stroke="var(--border)" />
              <XAxis dataKey="tanggal" tickFormatter={(v) => formatTanggal(v, "d MMM")} tick={{ fontSize: 11, fill: "var(--muted-foreground)" }} axisLine={false} tickLine={false} minTickGap={28} />
              <YAxis tickFormatter={(v) => `${Math.round(v / 1000)}rb`} tick={{ fontSize: 11, fill: "var(--muted-foreground)", fontFamily: "var(--font-mono)" }} axisLine={false} tickLine={false} width={44} domain={["auto", "auto"]} />
              <Tooltip content={<GlassTooltip valueFormat={(v) => `${formatRupiah(v)}/${kom.satuan}`} />} cursor={{ stroke: garda.navy[300], strokeDasharray: "3 3" }} />
              <Area type="monotone" dataKey="konsumen" name="Konsumen" stroke={garda.navy[700]} strokeWidth={2} fill="url(#g-konsumen)" animationDuration={1200} animationEasing="ease-out" />
              <Line type="monotone" dataKey="produsen" name="Produsen" stroke={garda.data[500]} strokeWidth={2} dot={false} animationDuration={1400} animationEasing="ease-out" />
            </ComposedChart>
          </ResponsiveContainer>
        )}
      </div>
    </section>
  );
}
