import { ArrowDownRight, ArrowUpRight, Minus } from "lucide-react";
import type { ItemRingkas } from "@/components/garda/halaman-pas";
import { CountUp } from "@/components/motion/count-up";
import { Sparkline } from "@/components/motion/sparkline";
import type { KpiOperasional } from "@/lib/data/types";
import { garda } from "@/lib/design/tokens";
import { formatRupiah } from "@/lib/format";
import { cn } from "@/lib/utils";

const toneHex = {
  data: garda.data[500],
  normal: garda.status.normal.base,
  waspada: garda.status.waspada.base,
  defisit: garda.status.defisit.base,
  surplus: garda.status.surplus.base,
};

const fmt = (f: KpiOperasional["format"]) => (n: number) =>
  f === "persen"
    ? `${n.toLocaleString("id-ID", { minimumFractionDigits: 1, maximumFractionDigits: 1 })}%`
    : f === "rupiah"
      ? formatRupiah(n, { compact: true })
      : Math.round(n).toLocaleString("id-ID");

/**
 * Satuan di belakang angka delta. `keterangan_delta` dari server sudah memuat satuan untuk indikator persentase
 * ("poin dari pekan lalu"), jadi " poin" tidak ditambahkan lagi agar tidak terbaca "poin poin".
 */
function satuanDelta(kpi: KpiOperasional): string {
  if (kpi.format === "persen")
    return /^poin\b/i.test(kpi.keteranganDelta) ? "" : " poin";
  return kpi.id === "waspada" ? "" : "%";
}

/** Sel ringkasan KPI: angka countUp, delta bermakna (naik belum tentu baik), dan sparkline. */
export function itemKpi(kpi: KpiOperasional, naikBaik = true): ItemRingkas {
  // Tanpa perubahan (setelah dibulatkan satu desimal seperti tampilannya): netral, bukan "naik" yang dinilai baik/buruk.
  const datar = Math.abs(kpi.delta) < 0.05;
  const naik = kpi.delta >= 0;
  const baik = naik === naikBaik;
  const Arrow = datar ? Minus : naik ? ArrowUpRight : ArrowDownRight;
  const angkaDelta = `${Math.abs(kpi.delta).toLocaleString("id-ID", { maximumFractionDigits: 1 })}${satuanDelta(kpi)}`;

  return {
    label: kpi.label,
    href: kpi.href,
    tone: kpi.tone,
    arti: `${kpi.label}: ${datar ? "tetap" : naik ? "naik" : "turun"} ${angkaDelta} ${kpi.keteranganDelta}`,
    nilai: (
      <span className="flex items-center justify-between gap-2">
        <CountUp
          value={kpi.nilai}
          format={fmt(kpi.format)}
          className="min-w-0 truncate"
        />
        <span className="hidden shrink-0 sm:block">
          <Sparkline
            data={kpi.tren}
            color={toneHex[kpi.tone]}
            width={64}
            height={20}
          />
        </span>
      </span>
    ),
    keterangan: (
      <span data-perubahan>
        <span
          className={cn(
            "num inline-flex items-center font-semibold",
            datar
              ? "text-muted-foreground"
              : baik
                ? "text-normal-ink dark:text-emerald-300"
                : "text-defisit-ink dark:text-rose-300",
          )}
        >
          <Arrow className="size-3" aria-hidden />
          {angkaDelta}
        </span>{" "}
        <span>{kpi.keteranganDelta}</span>
      </span>
    ),
  };
}
