"use client";

import { ArrowDownRight, ArrowUpRight, Minus } from "lucide-react";
import Link from "next/link";
import { CountUp } from "@/components/motion/count-up";
import { Sparkline } from "@/components/motion/sparkline";
import { GlassCard } from "@/components/garda/glass-card";
import type { KpiOperasional } from "@/lib/data/types";
import { garda, toneClass } from "@/lib/design/tokens";
import { formatRupiah } from "@/lib/format";
import { fadeUp } from "@/lib/motion/presets";
import { cn } from "@/lib/utils";

const toneHex = { data: garda.data[500], normal: garda.status.normal.base, waspada: garda.status.waspada.base, defisit: garda.status.defisit.base, surplus: garda.status.surplus.base };

const fmt = (f: KpiOperasional["format"]) => (n: number) =>
  f === "persen" ? `${n.toLocaleString("id-ID", { minimumFractionDigits: 1, maximumFractionDigits: 1 })}%` : f === "rupiah" ? formatRupiah(n, { compact: true }) : Math.round(n).toLocaleString("id-ID");

/**
 * Satuan di belakang angka delta. `keterangan_delta` dari server sudah memuat satuan untuk indikator persentase
 * ("poin dari pekan lalu"), jadi " poin" tidak ditambahkan lagi agar tidak terbaca "poin poin".
 */
function satuanDelta(kpi: KpiOperasional): string {
  if (kpi.format === "persen") return /^poin\b/i.test(kpi.keteranganDelta) ? "" : " poin";
  return kpi.id === "waspada" ? "" : "%";
}

/** Kartu KPI: angka countUp, delta bermakna (naik belum tentu baik), sparkline tergambar. */
export function KpiCard({ kpi, naikBaik = true }: { kpi: KpiOperasional; naikBaik?: boolean }) {
  // Tanpa perubahan (setelah dibulatkan satu desimal seperti tampilannya): netral, bukan "naik" yang dinilai baik/buruk.
  const datar = Math.abs(kpi.delta) < 0.05;
  const naik = kpi.delta >= 0;
  const baik = naik === naikBaik;
  const Arrow = datar ? Minus : naik ? ArrowUpRight : ArrowDownRight;
  return (
    <Link href={kpi.href} className="block rounded-xl focus-visible:outline-offset-4">
      <GlassCard interactive variants={fadeUp} className="h-full p-5">
        <div className="flex items-start justify-between gap-3">
          <p className="text-sm text-muted-foreground">{kpi.label}</p>
          <span className={cn("mt-1.5 size-2 rounded-full", toneClass[kpi.tone].dot)} />
        </div>
        <CountUp value={kpi.nilai} format={fmt(kpi.format)} className="mt-3 block text-[2.125rem] leading-none font-semibold text-navy-900 dark:text-white" />
        <div className="mt-4 flex items-end justify-between gap-3">
          <p className="text-xs leading-snug">
            <span className={cn("num inline-flex items-center font-semibold", datar ? "text-muted-foreground" : baik ? "text-normal-ink dark:text-emerald-300" : "text-defisit-ink dark:text-rose-300")}>
              <Arrow className="size-3.5" />
              {Math.abs(kpi.delta).toLocaleString("id-ID", { maximumFractionDigits: 1 })}
              {satuanDelta(kpi)}
            </span>{" "}
            <span className="text-muted-foreground">{kpi.keteranganDelta}</span>
          </p>
          <Sparkline data={kpi.tren} color={toneHex[kpi.tone]} width={104} height={34} />
        </div>
      </GlassCard>
    </Link>
  );
}
