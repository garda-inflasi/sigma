"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { motion } from "motion/react";
import { Download, Factory, Thermometer, Truck } from "lucide-react";
import { toast } from "sonner";
import { useMemo } from "react";
import { DataTable } from "@/components/data-table/data-table";
import { KomoditasIcon } from "@/components/garda/komoditas-icon";
import { PageHeader, PanelTitle } from "@/components/garda/page-header";
import { PanelSkeleton } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import { CountUp } from "@/components/motion/count-up";
import { exportCsv } from "@/lib/csv";
import type { FasilitasColdStorage, StokColdStorage } from "@/lib/data/types";
import { formatAngka, formatPersen, formatTanggal } from "@/lib/format";
import { ease, fadeUp, stagger } from "@/lib/motion/presets";
import { useColdStorage } from "@/lib/query/hooks";
import { cn } from "@/lib/utils";

function KartuFasilitas({ f }: { f: FasilitasColdStorage }) {
  const util = f.kapasitasTon ? (f.terisiTon / f.kapasitasTon) * 100 : 0;
  const deviasiSuhu = Math.abs(f.suhuAktualC - f.suhuTargetC);
  const st = { OPERASIONAL: { tone: "normal", label: "Operasional" }, PERAWATAN: { tone: "waspada", label: "Perawatan" }, RENCANA: { tone: "netral", label: "Rencana" } } as const;
  return (
    <motion.article variants={fadeUp} className="glass rounded-xl p-5">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0"><p className="truncate font-display font-semibold">{f.nama}</p><p className="text-xs text-muted-foreground">{f.wilayah}</p></div>
        <StatusBadge tone={st[f.status].tone}>{st[f.status].label}</StatusBadge>
      </div>
      {f.status === "RENCANA" ? (
        <p className="mt-6 text-sm text-muted-foreground">Studi kelayakan oleh Unit Pembiayaan & Investasi. Kapasitas rencana <span className="num">{f.kapasitasTon} ton</span>.</p>
      ) : (
        <>
          <div className="mt-5 flex items-end justify-between">
            <p className="text-3xl font-semibold"><CountUp value={util} format={(n) => `${Math.round(n)}%`} /></p>
            <p className="num text-xs text-muted-foreground">{formatAngka(f.terisiTon)} / {formatAngka(f.kapasitasTon)} ton</p>
          </div>
          <div className="mt-2 h-2 overflow-hidden rounded-full bg-mist-200 dark:bg-white/10">
            <motion.div className={cn("h-full rounded-full", util > 85 ? "bg-defisit" : util > 65 ? "bg-data-500" : "bg-surplus")} initial={{ width: 0 }} animate={{ width: `${util}%` }} transition={{ duration: 1, ease: ease.garda }} />
          </div>
          <p className={cn("mt-4 flex items-center gap-1.5 text-sm", deviasiSuhu > 1.5 ? "text-waspada-ink" : "text-muted-foreground")}>
            <Thermometer className="size-4" /><span className="num">{formatAngka(f.suhuAktualC, 1)}°C</span> (target <span className="num">{f.suhuTargetC}°C</span>)
          </p>
        </>
      )}
    </motion.article>
  );
}

export function ColdStoragePage() {
  const { data, isLoading } = useColdStorage();
  const kolom = useMemo<ColumnDef<StokColdStorage>[]>(() => [
    { id: "komoditas", accessorKey: "komoditasNama", header: "Komoditas", cell: ({ row: { original: s } }) => <span className="flex items-center gap-2 font-medium"><KomoditasIcon kode={s.komoditasKode} className="text-muted-foreground" />{s.komoditasNama}</span> },
    { accessorKey: "fasilitasNama", header: "Fasilitas", filterFn: "equals" },
    { accessorKey: "volumeTon", header: "Volume", cell: ({ getValue }) => <span className="num">{formatAngka(getValue<number>(), 1)} ton</span> },
    { accessorKey: "masuk", header: "Masuk", cell: ({ getValue }) => <span className="whitespace-nowrap">{formatTanggal(getValue<string>())}</span> },
    { accessorKey: "sisaShelfLifeHari", header: "Sisa masa simpan", cell: ({ getValue }) => { const h = getValue<number>(); return <span className={cn("num inline-flex h-6 items-center rounded-full px-2 text-xs font-medium", h <= 5 ? "bg-defisit-soft text-defisit-ink" : h <= 14 ? "bg-waspada-soft text-waspada-ink" : "bg-mist-100 text-muted-foreground dark:bg-white/5")}>{h} hari</span>; } },
    { accessorKey: "risikoFoodLoss", header: "Risiko food loss", cell: ({ getValue }) => { const v = getValue<number>() * 100; return <span className="flex items-center gap-2"><span className="h-1.5 w-16 overflow-hidden rounded-full bg-mist-200 dark:bg-white/10"><span className={cn("block h-full rounded-full", v > 60 ? "bg-defisit" : v > 35 ? "bg-waspada" : "bg-normal")} style={{ width: `${v}%` }} /></span><span className="num text-xs">{formatPersen(v, { digits: 0 })}</span></span>; } },
    { accessorKey: "pemilik", header: "Pemilik", cell: ({ getValue }) => <span className="text-muted-foreground">{getValue<string>()}</span> },
  ], []);
  const stok = data?.stok ?? [];
  const risiko = stok.filter((s) => s.risikoFoodLoss > 0.6).reduce((a, s) => a + s.volumeTon, 0);

  return (
    <div className="mx-auto max-w-[1600px] space-y-6">
      <PageHeader title="Cold storage & food loss" description="Utilisasi fasilitas, suhu, dan masa simpan stok. Kepemilikan komoditas tetap di tangan produsen dan agregator." />
      {isLoading || !data ? <PanelSkeleton /> : (
        <motion.div variants={stagger(0.07)} initial="hidden" animate="show" className="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
          {data.fasilitas.map((f) => <KartuFasilitas key={f.id} f={f} />)}
        </motion.div>
      )}
      <section className="space-y-3">
        <PanelTitle title="Stok tersimpan" description={`${formatAngka(risiko, 1)} ton berisiko tinggi food loss. Urutkan menurut sisa masa simpan untuk menentukan prioritas penyaluran.`} />
        <DataTable data={stok} isLoading={isLoading} columns={kolom} getRowId={(r) => r.id} entityLabel="lot stok" pageSize={15}
          facets={[{ columnId: "fasilitasNama", label: "Fasilitas", options: (data?.fasilitas ?? []).filter((f) => f.status !== "RENCANA").map((f) => ({ value: f.nama, label: f.nama })) }]}
          initialSorting={[{ id: "sisaShelfLifeHari", desc: false }]} searchPlaceholder="Cari komoditas atau pemilik"
          rowTone={(s) => (s.sisaShelfLifeHari <= 5 ? "defisit" : s.sisaShelfLifeHari <= 14 ? "waspada" : undefined)}
          bulkActions={[
            { id: "distribusi", label: "Rencanakan distribusi", icon: Truck, perm: "logistik.kelola", tone: "data", run: (r) => { toast.success(`Rencana distribusi ${formatAngka(r.reduce((a, s) => a + s.volumeTon, 0), 1)} ton dibuat`, { description: "Rute dioptimalkan ke wilayah defisit terdekat." }); } },
            { id: "olahan", label: "Rekomendasikan olahan", icon: Factory, perm: "coldstorage.kelola", run: (r) => { toast.success(`${r.length} lot diusulkan untuk pengolahan turunan`, { description: "Diteruskan ke Unit Mutu & Pascapanen untuk uji mutu." }); } },
            { id: "ekspor", label: "Ekspor CSV", icon: Download, run: (r) => exportCsv(r, [{ header: "Komoditas", value: (x) => x.komoditasNama }, { header: "Fasilitas", value: (x) => x.fasilitasNama }, { header: "Volume (ton)", value: (x) => x.volumeTon }, { header: "Sisa masa simpan (hari)", value: (x) => x.sisaShelfLifeHari }, { header: "Risiko food loss", value: (x) => x.risikoFoodLoss }], "stok-cold-storage") },
          ]} />
      </section>
    </div>
  );
}
