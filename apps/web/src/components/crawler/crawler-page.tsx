"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { useQueryClient } from "@tanstack/react-query";
import { motion } from "motion/react";
import {
  Activity,
  AlertTriangle,
  BrainCircuit,
  CalendarClock,
  CheckCircle2,
  Clock,
  Play,
  RefreshCw,
  RotateCcw,
  Terminal,
  Workflow,
} from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { DataTable, SelTeks } from "@/components/data-table/data-table";
import {
  HalamanPas,
  IsiPas,
  RingkasanKeputusan,
  type ItemRingkas,
} from "@/components/garda/halaman-pas";
import { PageHeader } from "@/components/garda/page-header";
import { StatusBadge } from "@/components/garda/status-badge";
import { ViewTabs } from "@/components/garda/view-tabs";
import { Button } from "@/components/ui/button";
import {
  subscribePipelineJobsStream,
  type PipelineJobsSnapshot,
} from "@/lib/api/pipeline-jobs";
import type {
  SiklusMlResponse,
  SumberKesegaranMlResponse,
} from "@/lib/api/ml-prod";
import { useCan, useOptionalSession } from "@/lib/auth/session-context";
import type { Tone } from "@/lib/design/tokens";
import { formatRelatif } from "@/lib/format";
import { useTabHalaman } from "@/lib/hooks/use-tab-halaman";
import { ease } from "@/lib/motion/presets";
import {
  useCrawlerFreshness,
  useMlCycles,
  usePipelineJobs,
  useTriggerCrawler,
  useTriggerPipeline,
} from "@/lib/query/hooks";
import { qk } from "@/lib/query/keys";
import { cn } from "@/lib/utils";

const TAB = ["monitoring", "siklus"] as const;

const KATEGORI_TONE: Record<
  SumberKesegaranMlResponse["kategori"],
  { tone: Tone; label: string }
> = {
  SEGAR: { tone: "normal", label: "Segar (FRESH)" },
  BASI: { tone: "waspada", label: "Basi / Carry-forward" },
  GAGAL: { tone: "defisit", label: "Gagal (ERROR)" },
  DILEWATI: { tone: "data", label: "Dilewati" },
  BELUM_ADA_RUN_TERJADWAL: { tone: "netral", label: "Belum terjadwal" },
};

const SIKLUS_TONE: Record<string, { tone: Tone; label: string }> = {
  BERHASIL: { tone: "normal", label: "Berhasil" },
  SEBAGIAN: { tone: "waspada", label: "Sebagian" },
  GAGAL: { tone: "defisit", label: "Gagal" },
  BERJALAN: { tone: "data", label: "Berjalan" },
};

const JADWAL_DEFAULT: Record<string, string> = {
  BPS: "Harian · 06:00 WIB",
  PIHPS: "Harian · 06:00 WIB",
  BMKG: "Harian · 06:00 WIB",
  OPEN_METEO: "Harian · 06:00 WIB",
  SP2KP: "Harian · 06:00 WIB",
  BAPANAS: "Harian · 06:00 WIB",
  LAPANGAN: "Tiap 1 jam (xx:00 WIB)",
};

function hitungMundurJamBerikutnya(): string {
  const now = new Date();
  const nextHour = new Date(now);
  nextHour.setUTCMinutes(0, 0, 0);
  nextHour.setUTCHours(nextHour.getUTCHours() + 1);
  const diffS = Math.max(0, Math.floor((nextHour.getTime() - now.getTime()) / 1000));
  const m = Math.floor(diffS / 60);
  const s = diffS % 60;
  return `${String(m).padStart(2, "0")}m ${String(s).padStart(2, "0")}d`;
}

export function CrawlerPage() {
  const qc = useQueryClient();
  const sesi = useOptionalSession();
  const bolehKelolaSumber = useCan("sumber.kelola") || sesi?.role === "ADMIN_SISTEM";
  const bolehKelolaModel = useCan("model.kelola") || sesi?.role === "ADMIN_SISTEM";
  const bolehEksekusi = bolehKelolaSumber || bolehKelolaModel;

  const [tab, setTab] = useTabHalaman(TAB, "monitoring");
  const [sseConnected, setSseConnected] = useState(false);
  const [showLogs, setShowLogs] = useState(false);
  const [countdown, setCountdown] = useState(() => hitungMundurJamBerikutnya());

  const freshness = useCrawlerFreshness();
  const cycles = useMlCycles(24);
  const jobs = usePipelineJobs();

  const triggerCrawler = useTriggerCrawler();
  const triggerPipeline = useTriggerPipeline();

  const wasActiveRef = useRef(false);

  // Perbarui hitung mundur slot per jam setiap detik secara ringan.
  useEffect(() => {
    const id = window.setInterval(() => setCountdown(hitungMundurJamBerikutnya()), 1000);
    return () => window.clearInterval(id);
  }, []);

  // Langganan SSE /v1/ml/jobs/stream untuk update status crawler & training ML secara realtime.
  useEffect(() => {
    const off = subscribePipelineJobsStream(
      (snap: PipelineJobsSnapshot) => {
        qc.setQueryData(qk.crawler.jobs(), snap);
        const sedangAktif =
          snap.crawler.berjalan || snap.pipeline.berjalan || snap.chain.aktif;
        if (wasActiveRef.current && !sedangAktif) {
          qc.invalidateQueries({ queryKey: qk.crawler.freshness() });
          qc.invalidateQueries({ queryKey: qk.crawler.cycles(24) });
          qc.invalidateQueries({ queryKey: qk.eksekutif.ringkasan() });
          qc.invalidateQueries({ queryKey: qk.prediksi.all() });
          qc.invalidateQueries({ queryKey: qk.ews.all() });
          qc.invalidateQueries({ queryKey: qk.sumber.list() });
          qc.invalidateQueries({ queryKey: qk.siklus() });
        }
        wasActiveRef.current = sedangAktif;
      },
      (ok) => setSseConnected(ok),
    );
    return off;
  }, [qc]);

  const snap = jobs.data;
  const sumberRows = freshness.data?.sumber ?? [];
  const gagalSumberList = useMemo(
    () =>
      Array.from(
        new Set(
          sumberRows
            .filter((r) => r.kategori === "GAGAL" || r.kategori === "BASI")
            .map((r) => r.sumber),
        ),
      ),
    [sumberRows],
  );

  const ringkasanData = freshness.data?.ringkasan;
  const itemsRingkas: ItemRingkas[] = [
    {
      label: "Dataset segar (FRESH)",
      nilai: ringkasanData?.segar ?? 0,
      tone: "normal",
    },
    {
      label: "Basi / carry-forward",
      nilai: ringkasanData?.basi ?? 0,
      tone: (ringkasanData?.basi ?? 0) > 0 ? "waspada" : "netral",
    },
    {
      label: "Gagal (ERROR)",
      nilai: ringkasanData?.gagal ?? 0,
      tone: (ringkasanData?.gagal ?? 0) > 0 ? "defisit" : "netral",
    },
    {
      label: "Siklus ML ml_predictions",
      nilai: cycles.data?.items?.[0]?.cycle_id ?? "Belum ada",
      tone: cycles.data?.items?.[0]?.status === "BERHASIL" ? "normal" : "data",
      keterangan: cycles.data?.items?.[0]?.status ?? "Menunggu siklus",
    },
  ];

  const sedangBerjalan = Boolean(
    snap?.crawler.berjalan || snap?.pipeline.berjalan || snap?.chain.aktif,
  );

  const progresAktifPct = snap?.chain.aktif
    ? snap.chain.progress_pct
    : snap?.pipeline.berjalan
      ? snap.pipeline.progress_pct
      : snap?.crawler.berjalan
        ? snap.crawler.progress_pct
        : snap?.chain.tahap === "SELESAI"
          ? 100
          : snap?.pipeline.progress_pct || snap?.crawler.progress_pct || 0;

  const tahapLabel = snap?.chain.aktif
    ? `Rantai ${snap.chain.tahap}: ${snap.chain.pesan ?? ""}`
    : snap?.pipeline.berjalan
      ? `Training ML · ${snap.pipeline.tahap ?? "PROSES"}`
      : snap?.crawler.berjalan
        ? `Crawler · ${snap.crawler.tahap ?? "CRAWL"}${snap.crawler.dataset_aktif ? ` (${snap.crawler.dataset_aktif})` : ""}`
        : snap?.chain.error || snap?.pipeline.error || snap?.crawler.error
          ? `Terakhir gagal: ${snap?.chain.error || snap?.pipeline.error || snap?.crawler.error}`
          : "Worker siaga menunggu jadwal berikutnya";

  const logsGabungan = useMemo(() => {
    if (!snap) return [];
    const arr = [
      ...(snap.chain.logs ?? []),
      ...(snap.crawler.logs ?? []),
      ...(snap.pipeline.logs ?? []),
    ];
    return arr.slice(-25);
  }, [snap]);

  const kolomDataset = useMemo<ColumnDef<SumberKesegaranMlResponse>[]>(
    () => [
      {
        accessorKey: "sumber",
        header: "Sumber",
        cell: ({ row }) => (
          <div className="min-w-0">
            <div className="font-mono text-xs font-semibold text-foreground">
              {row.original.sumber}
            </div>
            <div className="text-[11px] text-muted-foreground">
              {row.original.dataset}
            </div>
          </div>
        ),
      },
      {
        id: "jadwal",
        header: "Jadwal siklus",
        cell: ({ row }) => (
          <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground whitespace-nowrap">
            <Clock className="size-3.5 text-muted-foreground/70" />
            {JADWAL_DEFAULT[row.original.sumber] ?? "Harian · 06:00 WIB"}
          </span>
        ),
      },
      {
        accessorKey: "kategori",
        header: "Status",
        cell: ({ row }) => {
          const st = KATEGORI_TONE[row.original.kategori];
          return <StatusBadge tone={st.tone}>{st.label}</StatusBadge>;
        },
        filterFn: (row, id, val) =>
          !val || val === "semua" || row.getValue(id) === val,
      },
      {
        accessorKey: "last_available_date",
        header: "Tanggal data terakhir",
        cell: ({ row }) => (
          <span className="font-mono text-xs tabular-nums">
            {row.original.last_available_date ?? "—"}
          </span>
        ),
      },
      {
        id: "dokumen",
        header: "Dokumen baru",
        cell: ({ row }) => (
          <span className="font-mono text-xs tabular-nums">
            {row.original.run_terakhir?.jumlah_baru ?? 0}
          </span>
        ),
      },
      {
        id: "terakhir",
        header: "Crawl terakhir",
        cell: ({ row }) => (
          <span className="text-xs text-muted-foreground whitespace-nowrap">
            {row.original.run_terakhir?.selesai
              ? formatRelatif(row.original.run_terakhir.selesai)
              : "Belum ada"}
          </span>
        ),
      },
      {
        accessorKey: "pesan",
        header: "Keterangan",
        cell: ({ row }) => <SelTeks>{row.original.pesan}</SelTeks>,
      },
      {
        id: "aksi",
        header: "Aksi manual",
        meta: { sematKanan: true },
        cell: ({ row }) => {
          const isError =
            row.original.kategori === "GAGAL" ||
            row.original.kategori === "BASI";
          return (
            <div className="flex items-center justify-end gap-1.5">
              <Button
                size="sm"
                variant={isError ? "default" : "outline"}
                className="h-7 px-2.5 text-xs"
                disabled={!bolehEksekusi || sedangBerjalan || triggerCrawler.isPending}
                onClick={(e) => {
                  e.stopPropagation();
                  triggerCrawler.mutate({
                    sumber: [row.original.sumber],
                    lanjut_ml: false,
                  });
                }}
              >
                <RotateCcw className="size-3 mr-1" />
                {isError ? "Retry" : "Crawl"}
              </Button>
              <Button
                size="sm"
                variant="outline"
                className="h-7 px-2.5 text-xs"
                disabled={!bolehEksekusi || sedangBerjalan || triggerCrawler.isPending}
                onClick={(e) => {
                  e.stopPropagation();
                  triggerCrawler.mutate({
                    sumber: [row.original.sumber],
                    lanjut_ml: true,
                  });
                }}
                title="Crawl sumber ini lalu langsung latih ulang model ML"
              >
                <Workflow className="size-3 mr-1" />
                + ML
              </Button>
            </div>
          );
        },
      },
    ],
    [bolehEksekusi, sedangBerjalan, triggerCrawler],
  );

  const kolomSiklus = useMemo<ColumnDef<SiklusMlResponse>[]>(
    () => [
      {
        accessorKey: "cycle_id",
        header: "Cycle ID",
        cell: ({ row }) => (
          <span className="font-mono text-xs font-semibold">
            {row.original.cycle_id}
          </span>
        ),
      },
      {
        accessorKey: "status",
        header: "Status",
        cell: ({ row }) => {
          const s = SIKLUS_TONE[row.original.status] ?? {
            tone: "netral" as Tone,
            label: row.original.status,
          };
          return <StatusBadge tone={s.tone}>{s.label}</StatusBadge>;
        },
        filterFn: (row, id, val) =>
          !val || val === "semua" || row.getValue(id) === val,
      },
      {
        accessorKey: "cycle_timestamp",
        header: "Waktu training",
        cell: ({ row }) => (
          <span className="text-xs text-muted-foreground whitespace-nowrap">
            {formatRelatif(row.original.cycle_timestamp)}
          </span>
        ),
      },
      {
        accessorKey: "data_cutoff",
        header: "Cutoff data",
        cell: ({ row }) => (
          <span className="font-mono text-xs tabular-nums">
            {row.original.data_cutoff ?? "—"}
          </span>
        ),
      },
      {
        accessorKey: "label_target",
        header: "Target inflasi",
        cell: ({ row }) => (
          <SelTeks>
            {row.original.label_target ?? row.original.target_inflasi ?? "—"}
          </SelTeks>
        ),
      },
    ],
    [],
  );

  return (
    <HalamanPas>
      <PageHeader
        title="Pipeline & Crawler"
        description="Monitoring jadwal crawler Medallion Bronze, retry manual sumber gagal, serta orkestrasi training ML ml_predictions realtime."
        tabs={
          <ViewTabs
            layoutId="tab-crawler"
            label="Tampilan Pipeline & Crawler"
            tabs={[
              { value: "monitoring", label: "Monitoring & Jadwal" },
              { value: "siklus", label: "Riwayat Siklus ML" },
            ]}
            value={tab}
            onChange={setTab}
          />
        }
        actions={
          bolehEksekusi ? (
            <div className="flex flex-wrap items-center gap-2">
              {gagalSumberList.length > 0 && (
                <Button
                  size="sm"
                  variant="outline"
                  disabled={sedangBerjalan || triggerCrawler.isPending}
                  onClick={() =>
                    triggerCrawler.mutate({
                      sumber: gagalSumberList,
                      gagal_saja: true,
                      lanjut_ml: false,
                    })
                  }
                >
                  <RotateCcw className="size-3.5 mr-1.5" />
                  Retry Sumber Gagal ({gagalSumberList.length})
                </Button>
              )}
              <Button
                size="sm"
                variant="outline"
                disabled={sedangBerjalan || triggerPipeline.isPending}
                onClick={() => triggerPipeline.mutate()}
              >
                <BrainCircuit className="size-3.5 mr-1.5" />
                Latih Ulang ML
              </Button>
              <Button
                size="sm"
                disabled={sedangBerjalan || triggerCrawler.isPending}
                onClick={() =>
                  triggerCrawler.mutate({
                    gagal_saja: gagalSumberList.length > 0,
                    sumber: gagalSumberList.length > 0 ? gagalSumberList : undefined,
                    lanjut_ml: true,
                  })
                }
              >
                <Play className="size-3.5 mr-1.5" />
                Crawler → Latih ML
              </Button>
            </div>
          ) : null
        }
      />

      <RingkasanKeputusan items={itemsRingkas} />

      {/* Strip jadwal & panel status progres realtime */}
      <div className="shrink-0 rounded-2xl border border-border bg-card p-3.5 shadow-xs space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="flex items-center gap-2.5 min-w-0">
            <span
              className={cn(
                "inline-flex size-8 shrink-0 items-center justify-center rounded-xl border",
                sedangBerjalan
                  ? "border-sky-200 bg-sky-50 text-sky-600"
                  : "border-emerald-200 bg-emerald-50 text-emerald-600",
              )}
            >
              {sedangBerjalan ? (
                <RefreshCw className="size-4 animate-spin" />
              ) : (
                <Activity className="size-4" />
              )}
            </span>
            <div className="min-w-0">
              <div className="flex items-center gap-2 flex-wrap">
                <p className="text-sm font-medium text-foreground truncate">
                  {tahapLabel}
                </p>
                <span
                  className={cn(
                    "inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium",
                    sseConnected
                      ? "bg-emerald-50 text-emerald-700"
                      : "bg-slate-100 text-slate-600",
                  )}
                >
                  <span
                    className={cn(
                      "size-1.5 rounded-full",
                      sseConnected ? "bg-emerald-500 animate-pulse" : "bg-slate-400",
                    )}
                  />
                  {sseConnected ? "SSE Realtime" : "Polling"}
                </span>
              </div>
              <div className="mt-0.5 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
                <span className="inline-flex items-center gap-1">
                  <CalendarClock className="size-3.5" />
                  Harian: 06:00 WIB · Lapangan & ML: Tiap jam (slot berikutnya dalam{" "}
                  <span className="font-mono font-medium text-foreground tabular-nums">
                    {countdown}
                  </span>
                  )
                </span>
                {snap && !snap.crawler.configured && (
                  <span className="inline-flex items-center gap-1 text-amber-700">
                    <AlertTriangle className="size-3.5" />
                    Kontrol worker lokal mode baca (set{" "}
                    <code className="font-mono">CRAWLER_SERVICE_URL</code> untuk eksekusi live)
                  </span>
                )}
                {!sedangBerjalan && snap?.pipeline.cycle_id && (
                  <span className="inline-flex items-center gap-1 text-emerald-700">
                    <CheckCircle2 className="size-3.5" />
                    Siklus terakhir:{" "}
                    <code className="font-mono">{snap.pipeline.cycle_id}</code>
                  </span>
                )}
              </div>
            </div>
          </div>

          <div className="flex items-center gap-2 shrink-0">
            <span className="font-mono text-xs font-semibold tabular-nums text-foreground">
              {progresAktifPct}%
            </span>
            <Button
              size="sm"
              variant="ghost"
              className="h-7 px-2 text-xs"
              onClick={() => setShowLogs((v) => !v)}
            >
              <Terminal className="size-3.5 mr-1" />
              {showLogs ? "Tutup log" : `Log (${logsGabungan.length})`}
            </Button>
          </div>
        </div>

        {/* Progress bar */}
        <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
          <motion.div
            className={cn(
              "h-full rounded-full",
              snap?.chain.tahap === "GAGAL" || snap?.crawler.error || snap?.pipeline.error
                ? "bg-rose-500"
                : sedangBerjalan
                  ? "bg-sky-500"
                  : "bg-emerald-500",
            )}
            initial={false}
            animate={{ width: `${Math.max(4, progresAktifPct)}%` }}
            transition={{ duration: 0.4, ease: ease.garda }}
          />
        </div>

        {showLogs && (
          <div className="max-h-40 overflow-y-auto rounded-xl border border-border bg-slate-950 p-3 font-mono text-[11px] leading-relaxed text-slate-200">
            {logsGabungan.length === 0 ? (
              <p className="text-slate-400">Belum ada log eksekusi manual pada sesi worker saat ini.</p>
            ) : (
              logsGabungan.map((line, i) => (
                <div key={`${i}-${line.slice(0, 12)}`} className="whitespace-pre-wrap">
                  {line}
                </div>
              ))
            )}
          </div>
        )}
      </div>

      <IsiPas>
        {tab === "monitoring" ? (
          <DataTable
            isiPenuh
            data={sumberRows}
            columns={kolomDataset}
            getRowId={(r) => `${r.sumber}:${r.dataset}`}
            entityLabel="dataset crawler"
            isLoading={freshness.isLoading}
            searchPlaceholder="Cari sumber, dataset, atau keterangan…"
            facets={[
              {
                columnId: "kategori",
                label: "Status",
                options: [
                  { value: "SEGAR", label: "Segar (FRESH)" },
                  { value: "BASI", label: "Basi / Carry-forward" },
                  { value: "GAGAL", label: "Gagal (ERROR)" },
                  { value: "DILEWATI", label: "Dilewati" },
                ],
              },
            ]}
            empty={{
              title: "Belum ada catatan run crawler",
              description:
                freshness.data?.alasan ??
                "Jalankan worker crawler atau tekan tombol Crawl untuk mengisi koleksi crawl_runs di MongoDB ml_predictions.",
            }}
          />
        ) : (
          <DataTable
            isiPenuh
            data={cycles.data?.items ?? []}
            columns={kolomSiklus}
            getRowId={(r) => r.cycle_id}
            entityLabel="siklus ML"
            isLoading={cycles.isLoading}
            searchPlaceholder="Cari cycle ID atau target…"
            facets={[
              {
                columnId: "status",
                label: "Status",
                options: [
                  { value: "BERHASIL", label: "Berhasil" },
                  { value: "SEBAGIAN", label: "Sebagian" },
                  { value: "GAGAL", label: "Gagal" },
                  { value: "BERJALAN", label: "Berjalan" },
                ],
              },
            ]}
            empty={{
              title: "Belum ada riwayat siklus ML",
              description:
                cycles.data?.alasan ??
                "Picu ML pipeline setelah crawler selesai untuk menghasilkan siklus baru di MongoDB ml_predictions.",
            }}
          />
        )}
      </IsiPas>
    </HalamanPas>
  );
}
