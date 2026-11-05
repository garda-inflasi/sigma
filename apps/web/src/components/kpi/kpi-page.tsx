"use client";

import { motion } from "motion/react";
import {
  Building2,
  ChartNoAxesCombined,
  Database,
  Download,
  Loader2,
  Truck,
} from "lucide-react";
import { PageHeader } from "@/components/garda/page-header";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import { CountUp } from "@/components/motion/count-up";
import { Sparkline } from "@/components/motion/sparkline";
import {
  capaianPersen,
  formatKpi,
  KPI_KELOMPOK,
  type KpiIndikator,
  type KpiKelompok,
  type KpiStatus,
} from "@/lib/api/kpi";
import { garda } from "@/lib/design/tokens";
import { formatTanggal } from "@/lib/format";
import { ease, fadeUp, stagger } from "@/lib/motion/presets";
import { useEksporKpi, useKpi } from "@/lib/query/hooks";
import { cn } from "@/lib/utils";

const KELOMPOK: Record<KpiKelompok, { judul: string; icon: typeof Database }> =
  {
    DATA_SISTEM: { judul: "Data & sistem", icon: Database },
    LOGISTIK: { judul: "Logistik & infrastruktur", icon: Truck },
    DAMPAK_EKONOMI: { judul: "Dampak ekonomi", icon: ChartNoAxesCombined },
    TINDAK_LANJUT: { judul: "Tindak lanjut", icon: Building2 },
  };

const STATUS: Record<
  KpiStatus,
  {
    label: string;
    tone: "normal" | "waspada" | "defisit" | "netral" | "data";
    bar: string;
  }
> = {
  TERCAPAI: { label: "Tercapai", tone: "normal", bar: "bg-normal" },
  WASPADA: { label: "Waspada", tone: "waspada", bar: "bg-waspada" },
  TIDAK_TERCAPAI: {
    label: "Belum tercapai",
    tone: "defisit",
    bar: "bg-defisit",
  },
  TANPA_TARGET: { label: "Tanpa target", tone: "netral", bar: "bg-data-500" },
  MENUNGGU_DATA: { label: "Menunggu data", tone: "netral", bar: "bg-data-500" },
};

/** Alasan nilai kosong, supaya "menunggu" tidak terbaca sebagai nol atau galat. */
function alasanMenunggu(k: KpiIndikator): string {
  return k.otomatis
    ? "Belum ada data sumber untuk dihitung. Nilai muncul setelah data masuk."
    : "Butuh model atau data yang belum tersedia; dihitung di tahap berikutnya.";
}

function KpiBaris({ k }: { k: KpiIndikator }) {
  const status = STATUS[k.status];
  const pct = capaianPersen(k);
  return (
    <li
      className="grid grid-cols-[1fr_auto] items-center gap-x-4 gap-y-2"
      title={k.deskripsi || undefined}
    >
      <div className="min-w-0">
        <p className="truncate text-sm">{k.nama}</p>
        <p className="mt-0.5 text-xs text-muted-foreground">
          {k.target === null ? (
            "Target belum ditetapkan"
          ) : (
            <>
              Target{" "}
              <span className="num">{formatKpi(k.target, k.satuan)}</span>
              {k.naikBaik ? "" : ", makin rendah makin baik"}
            </>
          )}
        </p>
      </div>
      {k.nilai === null ? (
        <StatusBadge tone={status.tone}>{status.label}</StatusBadge>
      ) : (
        <div className="flex items-center gap-3">
          {k.tren.length >= 2 && (
            <Sparkline
              data={k.tren}
              color={
                k.status === "TERCAPAI"
                  ? garda.status.normal.base
                  : garda.data[500]
              }
              width={72}
              height={26}
              fill={false}
            />
          )}
          <CountUp
            value={k.nilai}
            format={(n) => formatKpi(n, k.satuan)}
            className="min-w-20 text-right text-lg font-semibold"
          />
        </div>
      )}
      {k.nilai === null ? (
        <p className="col-span-2 text-xs text-muted-foreground">
          {alasanMenunggu(k)}
        </p>
      ) : (
        <div className="col-span-2 space-y-1">
          {pct !== null && (
            <div className="h-1.5 overflow-hidden rounded-full bg-mist-200 dark:bg-white/10">
              <motion.div
                className={cn("h-full rounded-full", status.bar)}
                initial={{ width: 0 }}
                whileInView={{ width: `${pct}%` }}
                viewport={{ once: true }}
                transition={{ duration: 1, ease: ease.garda }}
              />
            </div>
          )}
          <p className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
            <span>{status.label}</span>
            {k.periode && (
              <span>Per {formatTanggal(k.periode, "MMMM yyyy")}</span>
            )}
          </p>
        </div>
      )}
    </li>
  );
}

/** Ekosistem KPI sesuai blueprint: capaian terhadap target dari hulu ke hilir. */
export function KpiPage() {
  const { data, isLoading, isError } = useKpi();
  const ekspor = useEksporKpi();

  return (
    <div className="mx-auto max-w-[1600px] space-y-6">
      <PageHeader
        title="Indikator kinerja"
        description="Capaian terhadap target PRD. Untuk indikator bertanda makin rendah makin baik, nilai lebih rendah berarti lebih baik. Indikator yang butuh model atau data yang belum ada tampil sebagai menunggu data."
        actions={
          <button
            type="button"
            onClick={() => ekspor.mutate()}
            disabled={ekspor.isPending}
            className="inline-flex h-10 items-center gap-2 rounded-xl border border-input bg-white/80 px-4 text-sm font-medium shadow-ambient-sm hover:bg-white disabled:opacity-60 dark:bg-white/[0.06]"
          >
            {ekspor.isPending ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              <Download className="size-4" />
            )}
            Unduh laporan CSV
          </button>
        }
      />
      {isLoading ? (
        <div className="grid gap-6 lg:grid-cols-2">
          <Shimmer className="h-[320px] w-full rounded-xl" />
          <Shimmer className="h-[320px] w-full rounded-xl" />
        </div>
      ) : isError || !data ? (
        <p
          role="alert"
          className="glass rounded-xl p-6 text-sm text-defisit-ink"
        >
          Indikator kinerja gagal dimuat. Muat ulang halaman atau coba lagi
          beberapa saat.
        </p>
      ) : (
        <motion.div
          variants={stagger(0.08)}
          initial="hidden"
          animate="show"
          className="grid gap-6 lg:grid-cols-2"
        >
          {KPI_KELOMPOK.map((kel) => {
            const items = data.filter((k) => k.kelompok === kel);
            if (items.length === 0) return null;
            const { judul, icon: Ikon } = KELOMPOK[kel];
            return (
              <motion.section
                key={kel}
                variants={fadeUp}
                className="glass rounded-xl p-5 sm:p-6"
              >
                <h2 className="flex items-center gap-2.5 text-display-sm font-semibold">
                  <span className="grid size-9 place-items-center rounded-xl bg-navy-50 text-navy-700 dark:bg-white/10 dark:text-navy-200">
                    <Ikon className="size-[18px]" />
                  </span>
                  {judul}
                </h2>
                <ul className="mt-5 space-y-5">
                  {items.map((k) => (
                    <KpiBaris key={k.kode} k={k} />
                  ))}
                </ul>
              </motion.section>
            );
          })}
        </motion.div>
      )}
    </div>
  );
}
