"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { motion } from "motion/react";
import {
  Activity,
  BadgeCheck,
  CloudUpload,
  Download,
  Layers,
  Pause,
  Play,
  Radar,
  ScanSearch,
  Workflow,
} from "lucide-react";
import { useMemo, useState } from "react";
import { DataTable, SelTeks } from "@/components/data-table/data-table";
import { DiagramAlur } from "@/components/flow/flow";
import {
  HalamanPas,
  IsiPas,
  RingkasanKeputusan,
  type ItemRingkas,
} from "@/components/garda/halaman-pas";
import { PageHeader, PanelTitle } from "@/components/garda/page-header";
import { StatusBadge } from "@/components/garda/status-badge";
import { ViewTabs } from "@/components/garda/view-tabs";
import { CountUp } from "@/components/motion/count-up";
import { Button } from "@/components/ui/button";
import { useCan } from "@/lib/auth/session-context";
import { exportCsv } from "@/lib/csv";
import type { SumberData } from "@/lib/data/types";
import { formatAngka, formatPersen, formatRelatif } from "@/lib/format";
import { useTabHalaman } from "@/lib/hooks/use-tab-halaman";
import { ease, fadeUp, stagger } from "@/lib/motion/presets";
import {
  useSumberBulk,
  useSumberData,
  useSumberPipeline,
} from "@/lib/query/hooks";
import { cn } from "@/lib/utils";
import { STATUS_SUMBER, langkahSumber } from "./status";
import { SumberDetail } from "./sumber-detail";

type StatusSumber = SumberData["status"];

const TAB = ["kesehatan", "pipeline"] as const;

// Mutu data dihitung server dari eksekusi ingestion nyata (kunci = bidang PipelineSumber["mutu"]).
const MUTU = [
  {
    key: "kelengkapan",
    label: "Kelengkapan",
    d: "Baris terisi dan valid, 7 hari",
  },
  {
    key: "akurasi",
    label: "Akurasi",
    d: "Harga lapangan sepakat dengan eksternal (selisih ≤ 10%), 14 hari",
  },
  {
    key: "ketepatanWaktu",
    label: "Ketepatan waktu",
    d: "Sumber yang sukses dalam 2× selang jadwal",
  },
  {
    key: "konsistensi",
    label: "Konsistensi",
    d: "Kode wilayah dan komoditas terpetakan ke master",
  },
] as const;

const SEL_RINGKASAN: {
  status: StatusSumber;
  keterangan: string;
  arti: string;
}[] = [
  {
    status: "GAGAL",
    keterangan: "Perlu diperbaiki",
    arti: "Eksekusi terakhir gagal, jadi data sumber ini tidak bertambah.",
  },
  {
    status: "TERTUNDA",
    keterangan: "Perlu dipantau",
    arti: "Eksekusi terakhir hanya sebagian berhasil, atau sumber menunggu eksekusi berikutnya.",
  },
  {
    status: "SEHAT",
    keterangan: "Berjalan normal",
    arti: "Eksekusi terakhir berhasil.",
  },
  {
    status: "DIJEDA",
    keterangan: "Dihentikan sementara",
    arti: "Jadwal dihentikan pengelola sampai dilanjutkan.",
  },
  {
    status: "BELUM_AKTIF",
    keterangan: "Belum pernah jalan",
    arti: "Sumber sudah terdaftar tetapi belum pernah dijalankan.",
  },
];

const urutKesehatan = (a: SumberData, b: SumberData) =>
  STATUS_SUMBER[a.status].urutan - STATUS_SUMBER[b.status].urutan ||
  (a.keberhasilan7h ?? 101) - (b.keberhasilan7h ?? 101) ||
  a.nama.localeCompare(b.nama, "id");

export function SumberPage() {
  const { data, isLoading } = useSumberData();
  const bulk = useSumberBulk();
  const { data: pipeline } = useSumberPipeline();
  const bolehKelola = useCan("sumber.kelola");
  const [tab, setTab] = useTabHalaman(TAB, "kesehatan");
  const [saring, setSaring] = useState<StatusSumber>();
  const [detailId, setDetailId] = useState<string | null>(null);
  const rows = useMemo(() => [...(data ?? [])].sort(urutKesehatan), [data]);
  const tampil = useMemo(
    () => (saring ? rows.filter((r) => r.status === saring) : rows),
    [rows, saring],
  );
  const detail = rows.find((r) => r.id === detailId) ?? null;
  const aktif = rows.filter(
    (r) => r.status !== "BELUM_AKTIF" && r.status !== "DIJEDA",
  ).length;

  const ringkasan: ItemRingkas[] = SEL_RINGKASAN.map((c) => ({
    label: STATUS_SUMBER[c.status].label,
    nilai: isLoading
      ? "–"
      : `${rows.filter((r) => r.status === c.status).length} sumber`,
    keterangan: c.keterangan,
    tone: STATUS_SUMBER[c.status].tone,
    arti: c.arti,
    aktif: saring === c.status,
    onClick: () => setSaring((v) => (v === c.status ? undefined : c.status)),
  }));

  const kolom = useMemo<ColumnDef<SumberData>[]>(
    () => [
      {
        id: "nama",
        accessorFn: (s) => `${s.nama} ${s.penyedia}`,
        header: "Sumber",
        cell: ({ row: { original: s } }) => (
          <div className="min-w-0">
            <SelTeks className="max-w-[20rem] font-medium">{s.nama}</SelTeks>
            <SelTeks className="max-w-[20rem] text-xs text-muted-foreground">
              {`${s.jenis === "API" ? "API" : "Scraper"} · ${s.penyedia}, ${s.cakupan}`}
            </SelTeks>
          </div>
        ),
      },
      { accessorKey: "jenis", header: "Jenis", filterFn: "equals" },
      {
        accessorKey: "status",
        header: "Status",
        cell: ({ getValue }) => {
          const s = STATUS_SUMBER[getValue<StatusSumber>()];
          return <StatusBadge tone={s.tone}>{s.label}</StatusBadge>;
        },
      },
      {
        id: "langkah",
        header: "Langkah berikutnya",
        enableSorting: false,
        enableGlobalFilter: false,
        cell: ({ row: { original: s } }) => (
          <SelTeks
            className={cn(
              "max-w-[17rem]",
              s.status === "SEHAT"
                ? "text-muted-foreground"
                : s.status === "GAGAL" && "font-medium text-defisit-ink",
            )}
          >
            {langkahSumber(s, bolehKelola)}
          </SelTeks>
        ),
      },
      {
        accessorKey: "terakhir",
        header: "Terakhir jalan",
        cell: ({ row: { original: s } }) => (
          <div className="min-w-0">
            {s.terakhir ? (
              <span className="num block whitespace-nowrap">
                {formatRelatif(s.terakhir)}
              </span>
            ) : (
              <span className="block whitespace-nowrap text-muted-foreground">
                Belum pernah
              </span>
            )}
            <SelTeks className="num max-w-[11rem] text-xs text-muted-foreground">
              {s.jadwal}
            </SelTeks>
          </div>
        ),
      },
      {
        accessorKey: "keberhasilan7h",
        header: "Sukses 7 hari",
        cell: ({ row: { original: s } }) => {
          const v = s.keberhasilan7h;
          return (
            <div className="whitespace-nowrap">
              <span
                className={cn(
                  "num block",
                  v === null
                    ? "text-muted-foreground"
                    : v < 80
                      ? "text-defisit-ink"
                      : v < 95
                        ? "text-waspada-ink"
                        : "",
                )}
              >
                {v === null ? "—" : formatPersen(v)}
              </span>
              <span className="num block text-xs text-muted-foreground">
                {s.latensiMs ? `${formatAngka(s.latensiMs)} ms` : "—"}
              </span>
            </div>
          );
        },
      },
    ],
    [bolehKelola],
  );

  return (
    <HalamanPas>
      <PageHeader
        title="Sumber data & API"
        description="Pantau kesehatan tiap sumber, perbaiki yang gagal, dan klik baris untuk riwayat eksekusi."
        tabs={
          <ViewTabs
            layoutId="tab-sumber"
            label="Tampilan sumber data"
            value={tab}
            onChange={setTab}
            tabs={[
              {
                value: "kesehatan",
                label: "Kesehatan sumber",
                icon: <Activity />,
                count: data?.length,
              },
              {
                value: "pipeline",
                label: "Pipeline & mutu",
                icon: <Workflow />,
              },
            ]}
          />
        }
      />

      {tab === "kesehatan" && (
        <>
          <RingkasanKeputusan
            items={ringkasan}
            label="Ringkasan kesehatan sumber"
          />
          <IsiPas>
            <DataTable
              key={saring ?? "semua"}
              isiPenuh
              data={tampil}
              isLoading={isLoading}
              columns={kolom}
              getRowId={(r) => r.id}
              entityLabel="sumber"
              hiddenColumns={["jenis"]}
              searchPlaceholder="Cari sumber atau penyedia"
              facets={[
                {
                  columnId: "jenis",
                  label: "Jenis",
                  options: [
                    { value: "API", label: "API" },
                    { value: "SCRAPER", label: "Scraper" },
                  ],
                },
              ]}
              toolbar={
                saring && (
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-9 text-[13px]"
                    onClick={() => setSaring(undefined)}
                  >
                    Semua status
                  </Button>
                )
              }
              empty={
                saring
                  ? {
                      title: `Tidak ada sumber berstatus ${STATUS_SUMBER[saring].label.toLowerCase()}`,
                      description:
                        "Ubah kata kunci, atau tampilkan semua status.",
                      action: (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => setSaring(undefined)}
                        >
                          Tampilkan semua status
                        </Button>
                      ),
                    }
                  : undefined
              }
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
                    bulk.mutateAsync({
                      aksi: "jeda",
                      ids: r.map((x) => x.id),
                      label: "Dijeda",
                    }),
                },
                {
                  id: "lanjut",
                  label: "Lanjutkan",
                  icon: Play,
                  perm: "sumber.kelola",
                  enabled: (r) => r.every((x) => x.status === "DIJEDA"),
                  run: (r) =>
                    bulk.mutateAsync({
                      aksi: "lanjutkan",
                      ids: r.map((x) => x.id),
                      label: "Dilanjutkan",
                    }),
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
          </IsiPas>
        </>
      )}

      {tab === "pipeline" && (
        <div className="scrollbar-thin min-w-0 space-y-4 lg:min-h-0 lg:flex-1 lg:overflow-y-auto">
          <section className="space-y-3">
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
            className="grid grid-cols-2 gap-3 xl:grid-cols-4"
          >
            {MUTU.map((m) => {
              const v = pipeline?.mutu[m.key] ?? null;
              return (
                <motion.div
                  key={m.key}
                  variants={fadeUp}
                  className="glass rounded-lg p-3.5"
                >
                  <p className="text-[13px] text-muted-foreground">{m.label}</p>
                  {v === null ? (
                    <span className="mt-1 block text-xl font-semibold text-muted-foreground">
                      —
                    </span>
                  ) : (
                    <CountUp
                      value={v}
                      format={(n) => formatPersen(n)}
                      className="mt-1 block text-xl font-semibold"
                    />
                  )}
                  <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-mist-200 dark:bg-white/10">
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
                  <p className="mt-1.5 text-xs text-muted-foreground">
                    {v === null ? "Belum ada eksekusi untuk dihitung" : m.d}
                  </p>
                </motion.div>
              );
            })}
          </motion.div>
        </div>
      )}

      <SumberDetail sumber={detail} onClose={() => setDetailId(null)} />
    </HalamanPas>
  );
}
