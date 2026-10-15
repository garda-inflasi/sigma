"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { motion } from "motion/react";
import { BadgeCheck, CloudUpload, Download, Layers, Pause, Play, Radar, RotateCw, ScanSearch } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { DataTable } from "@/components/data-table/data-table";
import { DiagramAlur } from "@/components/flow/flow";
import { PageHeader, PanelTitle } from "@/components/garda/page-header";
import { StatusBadge } from "@/components/garda/status-badge";
import { CountUp } from "@/components/motion/count-up";
import { exportCsv } from "@/lib/csv";
import type { SumberData } from "@/lib/data/types";
import { formatAngka, formatPersen, formatRelatif } from "@/lib/format";
import { ease, fadeUp, stagger } from "@/lib/motion/presets";
import { useSumberData } from "@/lib/query/hooks";
import { cn } from "@/lib/utils";

const MUTU = [
  { label: "Kelengkapan", v: 97.2, d: "Field wajib terisi" },
  { label: "Akurasi", v: 94.8, d: "Lolos validasi silang sumber" },
  { label: "Ketepatan waktu", v: 91.5, d: "Tiba sebelum tenggat harian" },
  { label: "Konsistensi", v: 96.1, d: "Satuan & kode seragam" },
];
const STATUS = { SEHAT: { tone: "normal", label: "Sehat" }, TERTUNDA: { tone: "waspada", label: "Tertunda" }, GAGAL: { tone: "defisit", label: "Gagal" }, DIJEDA: { tone: "netral", label: "Dijeda" } } as const;

export function SumberPage() {
  const { data, isLoading } = useSumberData();
  const [rows, setRows] = useState<SumberData[]>([]);
  useEffect(() => { if (data) setRows(data); }, [data]);
  const ubah = (ids: string[], patch: Partial<SumberData>) => { const s = new Set(ids); setRows((r) => r.map((x) => (s.has(x.id) ? { ...x, ...patch } : x))); };

  const kolom = useMemo<ColumnDef<SumberData>[]>(() => [
    { accessorKey: "nama", header: "Sumber", cell: ({ row: { original: s } }) => <div><p className="font-medium">{s.nama}</p><p className="text-xs text-muted-foreground">{s.penyedia}, {s.cakupan}</p></div> },
    { accessorKey: "jenis", header: "Jenis", filterFn: "equals", cell: ({ getValue }) => <span className="rounded-md bg-mist-100 px-1.5 py-0.5 text-xs dark:bg-white/5">{getValue<string>() === "API" ? "API" : "Scraper"}</span> },
    { accessorKey: "jadwal", header: "Jadwal", cell: ({ getValue }) => <span className="whitespace-nowrap text-muted-foreground">{getValue<string>()}</span> },
    { accessorKey: "terakhir", header: "Terakhir jalan", cell: ({ getValue }) => <span className="whitespace-nowrap">{formatRelatif(getValue<string>())}</span> },
    { accessorKey: "keberhasilan7h", header: "Sukses 7 hari", cell: ({ getValue }) => { const v = getValue<number>(); return <span className={cn("num", v < 80 ? "text-defisit-ink" : v < 95 ? "text-waspada-ink" : "")}>{formatPersen(v)}</span>; } },
    { accessorKey: "latensiMs", header: "Latensi", cell: ({ getValue }) => <span className="num text-muted-foreground">{getValue<number>() ? `${formatAngka(getValue<number>())} ms` : "—"}</span> },
    { accessorKey: "status", header: "Status", filterFn: "equals", cell: ({ getValue }) => { const s = STATUS[getValue<SumberData["status"]>()]; return <StatusBadge tone={s.tone}>{s.label}</StatusBadge>; } },
  ], []);

  return (
    <div className="mx-auto max-w-[1600px] space-y-6">
      <PageHeader title="Sumber data & API" description="Integrasi API resmi dan scraper panel harga dengan penjadwal, retry, rate limiting, dan kepatuhan robots.txt serta atribusi sumber." />
      <section className="space-y-4">
        <PanelTitle title="Pipeline data" description="Rekaman hari ini mengalir dari sumber ke gold layer" />
        <DiagramAlur aliran nodes={[
          { judul: "Sumber", icon: CloudUpload, tone: "netral", metrik: `${rows.length || 8} aktif` },
          { judul: "Bronze", icon: Layers, tone: "waspada", deskripsi: "Mentah + lineage", metrik: "48.312" },
          { judul: "Silver", icon: ScanSearch, tone: "data", deskripsi: "Bersih & ternormalisasi", metrik: "46.980" },
          { judul: "Gold", icon: BadgeCheck, tone: "normal", deskripsi: "Terpublikasi", metrik: "44.105" },
          { judul: "Model & EWS", icon: Radar, tone: "surplus", deskripsi: "Fitur & deteksi", metrik: "tiap 15 mnt" },
        ]} />
      </section>
      <motion.div variants={stagger(0.06)} initial="hidden" animate="show" className="grid grid-cols-2 gap-4 xl:grid-cols-4">
        {MUTU.map((m) => (
          <motion.div key={m.label} variants={fadeUp} className="glass rounded-xl p-4">
            <p className="text-sm text-muted-foreground">{m.label}</p>
            <CountUp value={m.v} format={(n) => formatPersen(n)} className="mt-2 block text-2xl font-semibold" />
            <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-mist-200 dark:bg-white/10"><motion.div className={cn("h-full rounded-full", m.v >= 95 ? "bg-normal" : "bg-data-500")} initial={{ width: 0 }} animate={{ width: `${m.v}%` }} transition={{ duration: 1, ease: ease.garda }} /></div>
            <p className="mt-2 text-xs text-muted-foreground">{m.d}</p>
          </motion.div>
        ))}
      </motion.div>
      <DataTable data={rows} isLoading={isLoading} columns={kolom} getRowId={(r) => r.id} entityLabel="sumber" searchPlaceholder="Cari sumber atau penyedia"
        facets={[{ columnId: "status", label: "Status", options: Object.entries(STATUS).map(([value, v]) => ({ value, label: v.label })) }, { columnId: "jenis", label: "Jenis", options: [{ value: "API", label: "API" }, { value: "SCRAPER", label: "Scraper" }] }]}
        rowTone={(s) => (s.status === "GAGAL" ? "defisit" : s.status === "TERTUNDA" ? "waspada" : undefined)}
        bulkActions={[
          { id: "jalankan", label: "Jalankan sekarang", icon: RotateCw, perm: "sumber.kelola", tone: "data", enabled: (r) => r.every((x) => x.status !== "DIJEDA"), run: async (r) => { await new Promise((ok) => setTimeout(ok, 900)); ubah(r.map((x) => x.id), { status: "SEHAT", terakhir: new Date().toISOString() }); toast.success(`${r.length} sumber dijalankan ulang`); } },
          { id: "jeda", label: "Jeda", icon: Pause, perm: "sumber.kelola", enabled: (r) => r.every((x) => x.status !== "DIJEDA"), run: (r) => { ubah(r.map((x) => x.id), { status: "DIJEDA" }); toast(`${r.length} sumber dijeda`); } },
          { id: "lanjut", label: "Lanjutkan", icon: Play, perm: "sumber.kelola", enabled: (r) => r.every((x) => x.status === "DIJEDA"), run: (r) => { ubah(r.map((x) => x.id), { status: "SEHAT" }); toast.success(`${r.length} sumber dilanjutkan`); } },
          { id: "ekspor", label: "Ekspor CSV", icon: Download, run: (r) => exportCsv(r, [{ header: "Sumber", value: (x) => x.nama }, { header: "Penyedia", value: (x) => x.penyedia }, { header: "Status", value: (x) => x.status }, { header: "Sukses 7 hari", value: (x) => x.keberhasilan7h }], "sumber-data") },
        ]} />
    </div>
  );
}
