"use client";

import { memo } from "react";
import {
  Activity,
  Flame,
  Siren,
  Snowflake,
  TrendingUp,
} from "lucide-react";
import { CountUp } from "@/components/motion/count-up";
import { PulseRing } from "@/components/motion/pulse-ring";
import { Sparkline } from "@/components/motion/sparkline";
import type { AktivitasLapangan } from "@/lib/api/dashboard";
import type { StokOperasional } from "@/lib/api/dashboard-operasional";
import type { EwsAlert, KpiOperasional, RingkasanEksekutif } from "@/lib/data/types";
import { cn } from "@/lib/utils";

export interface KpiPulseStripProps {
  eksekutif?: RingkasanEksekutif;
  kpi?: KpiOperasional[];
  alerts?: EwsAlert[];
  aktivitas?: AktivitasLapangan[];
  stok?: StokOperasional;
  bolehEksekutif: boolean;
  bolehOperasional: boolean;
  bolehEws: boolean;
  bolehCold: boolean;
}

export const KpiPulseStrip = memo(function KpiPulseStrip({
  eksekutif,
  kpi,
  alerts,
  aktivitas,
  stok,
  bolehEksekutif,
  bolehOperasional,
  bolehEws,
  bolehCold,
}: KpiPulseStripProps) {
  const yoy = eksekutif?.inflasi?.yoy ?? null;
  const sasaran = eksekutif?.inflasi?.sasaran;
  const seriInflasi = eksekutif?.inflasi?.seri.map((s) => s.yoy) ?? [];
  const inflasiDiLuarSasaran =
    yoy !== null && sasaran && (yoy > sasaran.atas || yoy < sasaran.bawah);

  const ewsAktif = (alerts ?? []).filter((a) => a.status !== "SELESAI");
  const jumlahKritis = ewsAktif.filter((a) => a.severity === "KRITIS").length;
  const jumlahSiaga = ewsAktif.filter((a) => a.severity === "SIAGA").length;
  const jumlahWaspada = ewsAktif.filter((a) => a.severity === "WASPADA").length;

  const pendorong = eksekutif?.inflasi?.penyumbang?.[0];
  const topKpi = kpi?.[0];

  const totalEntri = (aktivitas ?? []).reduce((s, a) => s + a.entri, 0);
  const totalTarget = (aktivitas ?? []).reduce((s, a) => s + a.target, 0);
  const pctLapangan =
    totalTarget > 0 ? Math.min(100, Math.round((totalEntri / totalTarget) * 100)) : 0;

  const utilisasi = stok?.ringkasan.utilisasiPct ?? null;
  const mendekatiTon = stok?.ringkasan.mendekatiKedaluwarsaTon ?? 0;

  return (
    <section
      aria-label="Indikator denyut utama"
      className="grid shrink-0 grid-cols-2 gap-2.5 lg:grid-cols-5"
    >
      {/* 1. Inflasi YoY */}
      <div className="cc-bento glass flex items-center justify-between gap-2 rounded-xl px-3.5 py-2.5">
        <div className="min-w-0">
          <p className="flex items-center gap-1.5 text-[11px] font-medium text-muted-foreground">
            <TrendingUp className="size-3.5 text-data-600" />
            Inflasi Aceh (YoY)
          </p>
          <p
            className={cn(
              "mt-0.5 text-xl font-semibold tracking-tight",
              inflasiDiLuarSasaran ? "text-defisit" : "text-foreground",
            )}
          >
            {!bolehEksekutif ? (
              <span className="text-sm text-muted-foreground">Terbatas</span>
            ) : yoy !== null ? (
              <CountUp
                value={yoy}
                onView={false}
                format={(v) => `${v.toFixed(2)}%`}
              />
            ) : (
              "—"
            )}
          </p>
          <p className="truncate text-[11px] text-muted-foreground">
            {sasaran
              ? `Sasaran ${sasaran.bawah}–${sasaran.atas}% · MtM ${eksekutif?.inflasi?.mtm?.toFixed(2) ?? "—"}%`
              : "Menunggu rilis BPS"}
          </p>
        </div>
        {seriInflasi.length >= 2 && (
          <Sparkline
            data={seriInflasi}
            width={68}
            height={28}
            color={inflasiDiLuarSasaran ? "#ef4444" : "#0284c7"}
          />
        )}
      </div>

      {/* 2. Peringatan Dini Live */}
      <div
        className={cn(
          "cc-bento glass flex items-center justify-between gap-2 rounded-xl px-3.5 py-2.5",
          jumlahKritis > 0 && "border-rose-300/80 dark:border-rose-900/80",
        )}
      >
        <div className="min-w-0">
          <p className="flex items-center gap-1.5 text-[11px] font-medium text-muted-foreground">
            <Siren
              className={cn(
                "size-3.5",
                jumlahKritis > 0 ? "text-rose-500" : "text-muted-foreground",
              )}
            />
            Peringatan Aktif
          </p>
          <div className="mt-0.5 flex items-center gap-2">
            <span className="text-xl font-semibold tracking-tight text-foreground">
              {!bolehEws ? (
                <span className="text-sm text-muted-foreground">Terbatas</span>
              ) : (
                <CountUp value={ewsAktif.length} onView={false} />
              )}
            </span>
            {jumlahKritis > 0 && (
              <span className="inline-flex items-center gap-1 rounded-full bg-rose-500/15 px-1.5 py-0.5 text-[10px] font-semibold text-rose-600 dark:text-rose-300">
                <PulseRing tone="kritis" rings={1} />
                {jumlahKritis} KRITIS
              </span>
            )}
          </div>
          <p className="truncate text-[11px] text-muted-foreground">
            {bolehEws
              ? `${jumlahSiaga} siaga · ${jumlahWaspada} waspada`
              : "Perlu izin EWS"}
          </p>
        </div>
      </div>

      {/* 3. Pendorong Harga / KPI Operasional */}
      <div className="cc-bento glass flex items-center justify-between gap-2 rounded-xl px-3.5 py-2.5">
        <div className="min-w-0">
          <p className="flex items-center gap-1.5 text-[11px] font-medium text-muted-foreground">
            <Flame className="size-3.5 text-amber-500" />
            {pendorong ? "Pendorong Utama" : (topKpi?.label ?? "Tekanan Harga")}
          </p>
          <p className="mt-0.5 truncate text-base font-semibold tracking-tight text-foreground sm:text-lg">
            {pendorong
              ? pendorong.nama
              : topKpi
                ? `${topKpi.nilai.toLocaleString("id-ID")}`
                : "Stabil"}
          </p>
          <p className="truncate text-[11px] text-muted-foreground">
            {pendorong
              ? `Andil inflasi +${pendorong.andil.toFixed(2)}%`
              : (topKpi?.keteranganDelta ?? "Pantauan 14 hari")}
          </p>
        </div>
        {topKpi?.tren && topKpi.tren.length >= 2 && (
          <Sparkline
            data={topKpi.tren}
            width={64}
            height={28}
            color="#f59e0b"
          />
        )}
      </div>

      {/* 4. Denyut Input Lapangan */}
      <div className="cc-bento glass flex items-center justify-between gap-2 rounded-xl px-3.5 py-2.5">
        <div className="min-w-0 flex-1">
          <p className="flex items-center gap-1.5 text-[11px] font-medium text-muted-foreground">
            <Activity className="size-3.5 text-emerald-600" />
            Input Pasar Hari Ini
          </p>
          <p className="mt-0.5 text-xl font-semibold tracking-tight text-foreground">
            {!bolehOperasional ? (
              <span className="text-sm text-muted-foreground">Terbatas</span>
            ) : (
              <CountUp
                value={pctLapangan}
                onView={false}
                format={(v) => `${Math.round(v)}%`}
              />
            )}
          </p>
          <p className="truncate text-[11px] text-muted-foreground">
            {bolehOperasional
              ? `${totalEntri.toLocaleString("id-ID")} dari ${totalTarget.toLocaleString("id-ID")} target`
              : "Perlu izin operasional"}
          </p>
        </div>
      </div>

      {/* 5. Utilisasi Cold Storage */}
      <div className="cc-bento glass col-span-2 flex items-center justify-between gap-2 rounded-xl px-3.5 py-2.5 lg:col-span-1">
        <div className="min-w-0">
          <p className="flex items-center gap-1.5 text-[11px] font-medium text-muted-foreground">
            <Snowflake className="size-3.5 text-purple-600" />
            Cold Storage
          </p>
          <p className="mt-0.5 text-xl font-semibold tracking-tight text-foreground">
            {!bolehCold && !bolehOperasional ? (
              <span className="text-sm text-muted-foreground">Terbatas</span>
            ) : utilisasi !== null ? (
              <CountUp
                value={utilisasi}
                onView={false}
                format={(v) => `${Math.round(v)}%`}
              />
            ) : (
              "—"
            )}
          </p>
          <p className="truncate text-[11px] text-muted-foreground">
            {stok
              ? `${mendekatiTon.toFixed(1)} ton mendekati kedaluwarsa`
              : "Memantau stok fasilitas"}
          </p>
        </div>
      </div>
    </section>
  );
});
