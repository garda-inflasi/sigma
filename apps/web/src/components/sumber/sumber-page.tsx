"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { motion } from "motion/react";
import {
  BadgeCheck,
  CloudUpload,
  Download,
  Layers,
  Pause,
  Play,
  Radar,
  ScanSearch,
} from "lucide-react";
import { useMemo, useState } from "react";
import { DataTable } from "@/components/data-table/data-table";
import { DiagramAlur } from "@/components/flow/flow";
import { PageHeader, PanelTitle } from "@/components/garda/page-header";
import { StatusBadge } from "@/components/garda/status-badge";
import { CountUp } from "@/components/motion/count-up";
import { exportCsv } from "@/lib/csv";
import type { SumberData } from "@/lib/data/types";
import { formatAngka, formatPersen, formatRelatif } from "@/lib/format";
import { ease, fadeUp, stagger } from "@/lib/motion/presets";
import { useSumberBulk, useSumberData, useSumberPipeline } from "@/lib/query/hooks";
import { cn } from "@/lib/utils";
import { SumberDetail } from "./sumber-detail";

// Mutu data dihitung server dari eksekusi ingestion nyata (kunci = bidang PipelineSumber["mutu"]).
const MUTU = [
  { key: "kelengkapan", label: "Kelengkapan", d: "Baris terisi dan valid, 7 hari" },
  { key: "akurasi", label: "Akurasi", d: "Harga lapangan sepakat dengan eksternal (selisih ≤ 10%), 14 hari" },
  { key: "ketepatanWaktu", label: "Ketepatan waktu", d: "Sumber yang sukses dalam 2× selang jadwal" },
  { key: "konsistensi", label: "Konsistensi", d: "Kode wilayah dan komoditas terpetakan ke master" },
] as const;
const STATUS = {
  BELUM_AKTIF: { tone: "netral", label: "Belum aktif" },
  SEHAT: { tone: "normal", label: "Sehat" },
  TERTUNDA: { tone: "waspada", label: "Tertunda" },
  GAGAL: { tone: "defisit", label: "Gagal" },
  DIJEDA: { tone: "netral", label: "Dijeda" },
} as const;

export function SumberPage() {
  const { data, isLoading } = useSumberData();
  const bulk = useSumberBulk();
  const { data: pipeline } = useSumberPipeline();
  const [detailId, setDetailId] = useState<string | null>(null);
  const rows = useMemo(() => data ?? [], [data]);
  const detail = rows.find((r) => r.id === detailId) ?? null;
  const aktif = rows.filter(
    (r) => r.status !== "BELUM_AKTIF" && r.status !== "DIJEDA",
  ).length;

  const kolom = useMemo<ColumnDef<SumberData>[]>(
    () => [
      {
        accessorKey: "nama",
        header: "Sumber",
        cell: ({ row: { original: s } }) => (
          <div>
            <p className="font-medium">{s.nama}</p>
            <p className="text-xs text-muted-foreground">
              {s.penyedia}, {s.cakupan}
            </p>
          </div>
        ),
      },
      {
        accessorKey: "jenis",
        header: "Jenis",
        filterFn: "equals",
        cell: ({ getValue }) => (
          <span className="rounded-md bg-mist-100 px-1.5 py-0.5 text-xs dark:bg-white/5">
            {getValue<string>() === "API" ? "API" : "Scraper"}
          </span>
        ),
      },
      {
        accessorKey: "jadwal",
        header: "Jadwal",
        cell: ({ getValue }) => (
          <span className="whitespace-nowrap text-muted-foreground">
            {getValue<string>()}
          </span>
        ),
      },
      {
        accessorKey: "terakhir",
        header: "Terakhir jalan",
        cell: ({ getValue }) => {
          const v = getValue<string | null>();
          return v ? (
            <span className="whitespace-nowrap">{formatRelatif(v)}</span>
          ) : (
            <span className="whitespace-nowrap text-muted-foreground">
              Belum pernah
            </span>
          );
        },
      },
      {
        accessorKey: "keberhasilan7h",
        header: "Sukses 7 hari",
        cell: ({ getValue }) => {
          const v = getValue<number | null>();
          if (v === null)
            return <span className="num text-muted-foreground">—</span>;
          return (
            <span
              className={cn(
                "num",
                v < 80 ? "text-defisit-ink" : v < 95 ? "text-waspada-ink" : "",
              )}
            >
              {formatPersen(v)}
            </span>
          );
        },
      },
      {
        accessorKey: "latensiMs",
        header: "Latensi",
        cell: ({ getValue }) => (
          <span className="num text-muted-foreground">
            {getValue<number>() ? `${formatAngka(getValue<number>())} ms` : "—"}
          </span>
        ),
      },
      {
        accessorKey: "status",
        header: "Status",
        filterFn: "equals",
        cell: ({ getValue }) => {
          const s = STATUS[getValue<SumberData["status"]>()];
          return <StatusBadge tone={s.tone}>{s.label}</StatusBadge>;
        },
      },
    ],
    [],
  );

  return (
    <div className="mx-auto max-w-[1600px] space-y-6">
      <PageHeader
        title="Sumber data & API"
        description="Katalog integrasi API resmi dan scraper panel harga, lengkap dengan jadwal, kepatuhan robots.txt, dan atribusi. Sumber berstatus Belum aktif sampai pernah dijalankan; klik baris untuk riwayat eksekusi dan menjalankan sumber yang punya konektor."
      />
      <section className="space-y-4">
        <PanelTitle
          title="Pipeline data"
          description={
            pipeline?.demo
              ? "Jumlah baris per lapisan dari eksekusi ingestion; termasuk eksekusi contoh (data demo)"
              : "Jumlah baris per lapisan dari eksekusi ingestion"
          }
        />
        <DiagramAlur
          aliran
          nodes={[
            {
              judul: "Sumber",
              icon: CloudUpload,
              tone: "netral",
              metrik: `${aktif} aktif dari ${rows.length}`,
            },
            {
              judul: "Bronze",
              icon: Layers,
              tone: "waspada",
              deskripsi: "Mentah + lineage",
              metrik: pipeline ? formatAngka(pipeline.bronze) : "—",
            },
            {
              judul: "Silver",
              icon: ScanSearch,
              tone: "data",
              deskripsi: "Bersih & ternormalisasi",
              metrik: pipeline ? formatAngka(pipeline.silver) : "—",
            },
            {
              judul: "Gold",
              icon: BadgeCheck,
              tone: "normal",
              deskripsi: "Agregat harian siap pakai",
              metrik: pipeline ? formatAngka(pipeline.gold) : "—",
            },
            {
              judul: "Model & EWS",
              icon: Radar,
              tone: "surplus",
              deskripsi: "Fitur & deteksi",
              metrik: "agregat tiap 5 mnt",
            },
          ]}
        />
      </section>
      <motion.div
        variants={stagger(0.06)}
        initial="hidden"
        animate="show"
        className="grid grid-cols-2 gap-4 xl:grid-cols-4"
      >
        {MUTU.map((m) => {
          const v = pipeline?.mutu[m.key] ?? null;
          return (
            <motion.div
              key={m.key}
              variants={fadeUp}
              className="glass rounded-xl p-4"
            >
              <p className="text-sm text-muted-foreground">{m.label}</p>
              {v === null ? (
                <span className="mt-2 block text-2xl font-semibold text-muted-foreground">
                  —
                </span>
              ) : (
                <CountUp
                  value={v}
                  format={(n) => formatPersen(n)}
                  className="mt-2 block text-2xl font-semibold"
                />
              )}
              <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-mist-200 dark:bg-white/10">
                {v !== null && (
                  <motion.div
                    className={cn(
                      "h-full rounded-full",
                      v >= 95 ? "bg-normal" : "bg-data-500",
                    )}
                    initial={{ width: 0 }}
                    animate={{ width: `${v}%` }}
                    transition={{ duration: 1, ease: ease.garda }}
                  />
                )}
              </div>
              <p className="mt-2 text-xs text-muted-foreground">
                {v === null ? "Belum ada eksekusi untuk dihitung" : m.d}
              </p>
            </motion.div>
          );
        })}
      </motion.div>
      <DataTable
        data={rows}
        isLoading={isLoading}
        columns={kolom}
        getRowId={(r) => r.id}
        entityLabel="sumber"
        searchPlaceholder="Cari sumber atau penyedia"
        facets={[
          {
            columnId: "status",
            label: "Status",
            options: Object.entries(STATUS).map(([value, v]) => ({
              value,
              label: v.label,
            })),
          },
          {
            columnId: "jenis",
            label: "Jenis",
            options: [
              { value: "API", label: "API" },
              { value: "SCRAPER", label: "Scraper" },
            ],
          },
        ]}
        onRowClick={(s) => setDetailId(s.id)}
        rowTone={(s) =>
          s.status === "GAGAL"
            ? "defisit"
            : s.status === "TERTUNDA"
              ? "waspada"
              : undefined
        }
        bulkActions={[
          {
            id: "jeda",
            label: "Jeda",
            icon: Pause,
            perm: "sumber.kelola",
            enabled: (r) => r.every((x) => x.status !== "DIJEDA"),
            run: (r) =>
              bulk
                .mutateAsync({
                  aksi: "jeda",
                  ids: r.map((x) => x.id),
                  label: "Dijeda",
                })
                ,
          },
          {
            id: "lanjut",
            label: "Lanjutkan",
            icon: Play,
            perm: "sumber.kelola",
            enabled: (r) => r.every((x) => x.status === "DIJEDA"),
            run: (r) =>
              bulk
                .mutateAsync({
                  aksi: "lanjutkan",
                  ids: r.map((x) => x.id),
                  label: "Dilanjutkan",
                })
                ,
          },
          {
            id: "ekspor",
            label: "Ekspor CSV",
            icon: Download,
            run: (r) =>
              exportCsv(
                r,
                [
                  { header: "Sumber", value: (x) => x.nama },
                  { header: "Penyedia", value: (x) => x.penyedia },
                  { header: "Status", value: (x) => x.status },
                  {
                    header: "Sukses 7 hari",
                    value: (x) => x.keberhasilan7h ?? "",
                  },
                ],
                "sumber-data",
              ),
          },
        ]}
      />
      <SumberDetail sumber={detail} onClose={() => setDetailId(null)} />
    </div>
  );
}
