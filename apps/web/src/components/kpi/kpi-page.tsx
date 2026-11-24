"use client";

import { useMemo, useState } from "react";
import { motion } from "motion/react";
import {
  Building2,
  ChartNoAxesCombined,
  Database,
  Download,
  Loader2,
  Truck,
} from "lucide-react";
import { KpiRiwayatSheet } from "@/components/kpi/kpi-riwayat-sheet";
import { EmptyState } from "@/components/garda/empty-state";
import {
  HalamanPas,
  IsiPas,
  PanelPas,
  RingkasanKeputusan,
  type ItemRingkas,
} from "@/components/garda/halaman-pas";
import { PageHeader } from "@/components/garda/page-header";
import { PanelGalat } from "@/components/garda/panel-galat";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import { CountUp } from "@/components/motion/count-up";
import { Sparkline } from "@/components/motion/sparkline";
import { Button } from "@/components/ui/button";
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
import { ease } from "@/lib/motion/presets";
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
    keterangan: string;
    arti: string;
    urutan: number;
  }
> = {
  TIDAK_TERCAPAI: {
    label: "Belum tercapai",
    tone: "defisit",
    bar: "bg-defisit",
    keterangan: "Belum memenuhi target",
    arti: "Nilai terbaru belum memenuhi target dan melewati batas waspada. Klik untuk menyaring.",
    urutan: 0,
  },
  WASPADA: {
    label: "Waspada",
    tone: "waspada",
    bar: "bg-waspada",
    keterangan: "Hampir memenuhi target",
    arti: "Nilai belum memenuhi target tetapi masih dalam batas waspada. Klik untuk menyaring.",
    urutan: 1,
  },
  MENUNGGU_DATA: {
    label: "Menunggu data",
    tone: "netral",
    bar: "bg-data-500",
    keterangan: "Belum bisa dihitung",
    arti: "Indikator belum punya nilai karena data sumber atau model belum tersedia. Klik untuk menyaring.",
    urutan: 2,
  },
  TANPA_TARGET: {
    label: "Tanpa target",
    tone: "netral",
    bar: "bg-data-500",
    keterangan: "Target belum ditetapkan",
    arti: "Nilai sudah ada, tetapi belum ada target pembanding. Klik untuk menyaring.",
    urutan: 3,
  },
  TERCAPAI: {
    label: "Tercapai",
    tone: "normal",
    bar: "bg-normal",
    keterangan: "Memenuhi target",
    arti: "Nilai terbaru sudah memenuhi target. Klik untuk menyaring.",
    urutan: 4,
  },
};

const URUTAN_STATUS = (Object.keys(STATUS) as KpiStatus[]).sort(
  (a, b) => STATUS[a].urutan - STATUS[b].urutan,
);

/** Alasan nilai kosong, supaya "menunggu" tidak terbaca sebagai nol atau galat. */
function alasanMenunggu(k: KpiIndikator): string {
  return k.otomatis
    ? "Belum ada data sumber untuk dihitung. Nilai muncul setelah data masuk."
    : "Butuh model atau data yang belum tersedia; dihitung di tahap berikutnya.";
}

function KpiBaris({
  k,
  onBuka,
}: {
  k: KpiIndikator;
  onBuka: (kode: string) => void;
}) {
  const status = STATUS[k.status];
  const pct = capaianPersen(k);
  const penjelasan = k.deskripsi;
  return (
    <li title={penjelasan || undefined}>
      <button
        type="button"
        onClick={() => onBuka(k.kode)}
        aria-label={`Riwayat ${k.nama}`}
        className="grid w-full grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 gap-y-1 px-3.5 py-2.5 text-left transition-colors hover:bg-secondary/60 focus-visible:ring-[3px] focus-visible:ring-ring/30 focus-visible:outline-none focus-visible:ring-inset"
      >
        <span className="min-w-0">
          <span className="block truncate text-[13px] font-medium">
            {k.nama}
          </span>
          {penjelasan && (
            <span className="line-clamp-2 text-xs text-muted-foreground lg:line-clamp-1">
              {penjelasan}
            </span>
          )}
        </span>
        {k.nilai === null ? (
          <StatusBadge tone={status.tone}>{status.label}</StatusBadge>
        ) : (
          <span className="flex items-center gap-2.5">
            {k.tren.length >= 2 && (
              <span className="hidden sm:block">
                <Sparkline
                  data={k.tren}
                  color={
                    k.status === "TERCAPAI"
                      ? garda.status.normal.base
                      : garda.data[500]
                  }
                  width={56}
                  height={22}
                  fill={false}
                />
              </span>
            )}
            <CountUp
              value={k.nilai}
              format={(n) => formatKpi(n, k.satuan)}
              className="min-w-16 text-right text-[15px] font-semibold"
            />
          </span>
        )}
        <span className="col-span-2 flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
          {k.nilai !== null && pct !== null && (
            <span className="h-1 w-16 shrink-0 overflow-hidden rounded-full bg-mist-200 sm:w-24 dark:bg-white/10">
              <motion.span
                className={cn("block h-full rounded-full", status.bar)}
                initial={{ width: 0 }}
                animate={{ width: `${pct}%` }}
                transition={{ duration: 0.8, ease: ease.garda }}
              />
            </span>
          )}
          <span className="min-w-0 flex-1 lg:truncate">
            {k.nilai === null ? (
              alasanMenunggu(k)
            ) : k.target === null ? (
              "Target belum ditetapkan"
            ) : (
              <>
                {status.label} · target{" "}
                <span className="num">{formatKpi(k.target, k.satuan)}</span>
                {k.naikBaik ? "" : ", makin rendah makin baik"}
              </>
            )}
          </span>
          {k.nilai !== null && k.periode && (
            <span className="hidden shrink-0 sm:inline">
              Per {formatTanggal(k.periode, "MMM yyyy")}
            </span>
          )}
        </span>
      </button>
    </li>
  );
}

/** Ekosistem KPI sesuai blueprint: capaian terhadap target dari hulu ke hilir. */
export function KpiPage() {
  const { data, isLoading, isError, error, refetch } = useKpi();
  const ekspor = useEksporKpi();
  const [dibuka, setDibuka] = useState<string>();
  const [saring, setSaring] = useState<KpiStatus>();

  const jumlah = useMemo(() => {
    const n: Record<KpiStatus, number> = {
      TERCAPAI: 0,
      WASPADA: 0,
      TIDAK_TERCAPAI: 0,
      TANPA_TARGET: 0,
      MENUNGGU_DATA: 0,
    };
    for (const k of data ?? []) n[k.status]++;
    return n;
  }, [data]);

  const ringkasan: ItemRingkas[] = URUTAN_STATUS.filter(
    (s) => s !== "TANPA_TARGET" || jumlah[s] > 0,
  ).map((s) => ({
    label: STATUS[s].label,
    nilai: isLoading || !data ? "–" : `${jumlah[s]} indikator`,
    keterangan: STATUS[s].keterangan,
    tone: STATUS[s].tone,
    arti: STATUS[s].arti,
    aktif: saring === s,
    onClick: () => setSaring((v) => (v === s ? undefined : s)),
  }));

  return (
    <HalamanPas>
      <PageHeader
        title="Indikator kinerja"
        description="Capaian tiap indikator terhadap target program. Klik indikator untuk melihat riwayatnya."
        actions={
          <>
            {saring && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setSaring(undefined)}
              >
                Reset filter
              </Button>
            )}
            <Button
              variant="outline"
              size="sm"
              onClick={() => ekspor.mutate()}
              disabled={ekspor.isPending}
            >
              {ekspor.isPending ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                <Download className="size-4" />
              )}
              Unduh laporan CSV
            </Button>
          </>
        }
      />
      <RingkasanKeputusan
        items={ringkasan}
        label="Ringkasan capaian indikator"
        className="[&>li:last-child:nth-child(odd)]:col-span-2 lg:[&>li:last-child:nth-child(odd)]:col-auto"
      />
      {isLoading ? (
        <IsiPas className="lg:grid lg:grid-cols-2 lg:grid-rows-[minmax(0,1fr)_minmax(0,1fr)]">
          {KPI_KELOMPOK.map((kel) => (
            <Shimmer key={kel} className="h-48 w-full rounded-lg lg:h-full" />
          ))}
        </IsiPas>
      ) : isError || !data ? (
        <PanelGalat
          judul="Indikator kinerja gagal dimuat."
          error={error}
          onCoba={() => void refetch()}
          className="lg:flex-1"
        />
      ) : (
        <IsiPas className="lg:grid lg:grid-cols-2 lg:grid-rows-[minmax(0,1fr)_minmax(0,1fr)]">
          {KPI_KELOMPOK.map((kel) => {
            const semua = data.filter((k) => k.kelompok === kel);
            if (semua.length === 0) return null;
            const items = semua
              .filter((k) => !saring || k.status === saring)
              .sort(
                (a, b) => STATUS[a.status].urutan - STATUS[b.status].urutan,
              );
            const tercapai = semua.filter(
              (k) => k.status === "TERCAPAI",
            ).length;
            const { judul, icon: Ikon } = KELOMPOK[kel];
            return (
              <PanelPas
                key={kel}
                judul={
                  <span className="flex items-center gap-2">
                    <Ikon className="size-4 shrink-0 text-muted-foreground" />
                    {judul}
                  </span>
                }
                deskripsi={`${tercapai} dari ${semua.length} indikator tercapai`}
                tanpaPadding
              >
                {items.length === 0 ? (
                  <EmptyState
                    title="Tidak ada indikator"
                    description="Tidak ada indikator kelompok ini dengan status terpilih."
                    className="py-6"
                  />
                ) : (
                  <ul className="divide-y divide-border">
                    {items.map((k) => (
                      <KpiBaris key={k.kode} k={k} onBuka={setDibuka} />
                    ))}
                  </ul>
                )}
              </PanelPas>
            );
          })}
        </IsiPas>
      )}
      <KpiRiwayatSheet kode={dibuka} onClose={() => setDibuka(undefined)} />
    </HalamanPas>
  );
}
