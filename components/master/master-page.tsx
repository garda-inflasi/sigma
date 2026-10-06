"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { motion } from "motion/react";
import { Archive, CheckCircle2, Download, FilePen } from "lucide-react";
import Link from "next/link";
import { useMemo, useState } from "react";
import { toast } from "sonner";
import type { BulkAction } from "@/components/data-table/bulk-action-bar";
import { DataTable } from "@/components/data-table/data-table";
import { KomoditasIcon } from "@/components/garda/komoditas-icon";
import { PageHeader } from "@/components/garda/page-header";
import { StatusBadge } from "@/components/garda/status-badge";
import { exportCsv } from "@/lib/csv";
import { KABUPATEN, KOMODITAS, PASAR, kabupatenByKode } from "@/lib/data/reference";
import { ENTITAS, type Entitas } from "@/lib/data/master";
import { formatRupiah } from "@/lib/format";
import { spring } from "@/lib/motion/presets";
import { cn } from "@/lib/utils";

type Status = "AKTIF" | "MENUNGGU_PERSETUJUAN" | "DIARSIPKAN";
interface Baris { id: string; kode: string; nama: string; info: string; info2: string; versi: number; status: Status }

function bangun(e: Entitas): Baris[] {
  if (e === "komoditas") return KOMODITAS.map((k, i) => ({ id: k.kode, kode: k.kode, nama: k.nama, info: `${k.kategori}, per ${k.satuan}${k.pilot ? ", pilot" : ""}`, info2: formatRupiah(k.hargaAcuan), versi: 1 + (i % 3), status: k.kode === "CBR-RWT" ? "MENUNGGU_PERSETUJUAN" : "AKTIF" }));
  if (e === "wilayah") return KABUPATEN.map((k) => ({ id: k.kode, kode: k.kode, nama: `${k.jenis === "Kota" ? "Kota" : "Kab."} ${k.nama}`, info: `Ibu kota ${k.ibukota}`, info2: `${k.lat.toFixed(2)}, ${k.lng.toFixed(2)}`, versi: 1, status: "AKTIF" }));
  return PASAR.map((p) => ({ id: p.id, kode: p.id, nama: p.nama, info: kabupatenByKode[p.kabupatenKode].nama, info2: `Geofence ${p.radiusMeter} m`, versi: 1, status: "AKTIF" }));
}

const STATUS: Record<Status, { tone: "normal" | "waspada" | "netral"; label: string }> = { AKTIF: { tone: "normal", label: "Aktif" }, MENUNGGU_PERSETUJUAN: { tone: "waspada", label: "Menunggu persetujuan" }, DIARSIPKAN: { tone: "netral", label: "Diarsipkan" } };

/** Master data dengan versioning & persetujuan (pengaju ≠ penyetuju). */
export function MasterPage({ entitas }: { entitas: Entitas }) {
  const [rows, setRows] = useState<Baris[]>(() => bangun(entitas));
  const kolom = useMemo<ColumnDef<Baris>[]>(() => [
    { accessorKey: "kode", header: "Kode", cell: ({ getValue }) => <span className="num text-xs">{getValue<string>()}</span> },
    { accessorKey: "nama", header: "Nama", cell: ({ row: { original: r } }) => <span className="flex items-center gap-2 font-medium">{entitas === "komoditas" && <KomoditasIcon kode={r.kode} className="text-muted-foreground" />}{r.nama}</span> },
    { accessorKey: "info", header: entitas === "komoditas" ? "Kategori" : entitas === "wilayah" ? "Ibu kota" : "Kabupaten/kota", cell: ({ getValue }) => <span className="text-muted-foreground">{getValue<string>()}</span> },
    { accessorKey: "info2", header: entitas === "komoditas" ? "Harga acuan" : entitas === "wilayah" ? "Koordinat" : "Geofence", cell: ({ getValue }) => <span className="num text-xs">{getValue<string>()}</span> },
    { accessorKey: "versi", header: "Versi", cell: ({ getValue }) => <span className="num">v{getValue<number>()}</span> },
    { accessorKey: "status", header: "Status", filterFn: "equals", cell: ({ getValue }) => { const s = STATUS[getValue<Status>()]; return <StatusBadge tone={s.tone}>{s.label}</StatusBadge>; } },
  ], [entitas]);
  const ubah = (ids: string[], fn: (r: Baris) => Baris) => { const set = new Set(ids); setRows((rs) => rs.map((r) => (set.has(r.id) ? fn(r) : r))); };
  const actions: BulkAction<Baris>[] = [
    { id: "ajukan", label: "Ajukan perubahan", icon: FilePen, perm: "master.ajukan", enabled: (r) => r.every((x) => x.status === "AKTIF"), run: (r) => { ubah(r.map((x) => x.id), (x) => ({ ...x, status: "MENUNGGU_PERSETUJUAN" })); toast.success(`${r.length} perubahan diajukan`, { description: "Menunggu persetujuan Pimpinan FMIPA atau BPS." }); } },
    { id: "setujui", label: "Setujui", icon: CheckCircle2, perm: "master.setujui", tone: "data", enabled: (r) => r.every((x) => x.status === "MENUNGGU_PERSETUJUAN"), run: (r) => { ubah(r.map((x) => x.id), (x) => ({ ...x, status: "AKTIF", versi: x.versi + 1 })); toast.success(`${r.length} perubahan disetujui`, { description: "Versi baru berlaku untuk entri berikutnya." }); } },
    { id: "arsip", label: "Arsipkan", icon: Archive, perm: "master.ajukan", tone: "danger", confirm: { title: "Arsipkan data terpilih?", description: (n) => `${n} data tidak lagi muncul di form lapangan. Riwayat entri lama tetap tersimpan.`, actionLabel: "Arsipkan" }, run: (r) => { ubah(r.map((x) => x.id), (x) => ({ ...x, status: "DIARSIPKAN" })); toast.success(`${r.length} data diarsipkan`); } },
    { id: "ekspor", label: "Ekspor CSV", icon: Download, run: (r) => exportCsv(r, [{ header: "Kode", value: (x) => x.kode }, { header: "Nama", value: (x) => x.nama }, { header: "Keterangan", value: (x) => x.info }, { header: "Versi", value: (x) => x.versi }, { header: "Status", value: (x) => x.status }], `master-${entitas}`) },
  ];
  return (
    <div className="mx-auto max-w-[1600px] space-y-6">
      <PageHeader title="Master data" description="Standar kode wilayah BPS dan komoditas nasional. Setiap perubahan berversi dan membutuhkan persetujuan." />
      <nav className="glass-subtle inline-flex rounded-xl p-1" aria-label="Entitas master">
        {(Object.keys(ENTITAS) as Entitas[]).map((e) => (
          <Link key={e} href={`/master/${e}`} className={cn("relative h-9 rounded-lg px-4 text-sm font-medium leading-9", e === entitas ? "text-white" : "text-muted-foreground hover:text-foreground")}>
            {e === entitas && <motion.span layoutId="master-tab" transition={spring.layout} className="absolute inset-0 rounded-lg bg-navy-800 dark:bg-navy-600" />}
            <span className="relative">{ENTITAS[e]}</span>
          </Link>
        ))}
      </nav>
      <DataTable data={rows} columns={kolom} getRowId={(r) => r.id} entityLabel="data" bulkActions={actions} searchPlaceholder={`Cari ${ENTITAS[entitas].toLowerCase()}`}
        facets={[{ columnId: "status", label: "Status", options: Object.entries(STATUS).map(([value, v]) => ({ value, label: v.label })) }]} />
    </div>
  );
}
