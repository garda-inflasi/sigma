"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Download, Flag } from "lucide-react";
import { useMemo } from "react";
import { toast } from "sonner";
import { DataTable } from "@/components/data-table/data-table";
import { PageHeader } from "@/components/garda/page-header";
import { StatusBadge } from "@/components/garda/status-badge";
import { exportCsv } from "@/lib/csv";
import type { AuditRow } from "@/lib/data/types";
import { formatTanggal } from "@/lib/format";
import { useAudit } from "@/lib/query/hooks";

export function AuditPage() {
  const { data, isLoading } = useAudit();
  const kolom = useMemo<ColumnDef<AuditRow>[]>(() => [
    { accessorKey: "waktu", header: "Waktu", cell: ({ getValue }) => <span className="num whitespace-nowrap text-xs">{formatTanggal(getValue<string>(), "d MMM yyyy HH.mm.ss")}</span> },
    { accessorKey: "aktor", header: "Aktor", cell: ({ row: { original: a } }) => <div><p className="font-medium">{a.aktor}</p><p className="text-xs text-muted-foreground">{a.peran}</p></div> },
    { accessorKey: "aksi", header: "Aksi", filterFn: "equals", cell: ({ getValue }) => <span className="num rounded-md bg-mist-100 px-1.5 py-0.5 text-[11px] dark:bg-white/5">{getValue<string>()}</span> },
    { id: "entitas", accessorFn: (a) => `${a.entitas} ${a.entitasId}`, header: "Entitas", cell: ({ row: { original: a } }) => <span className="whitespace-nowrap">{a.entitas} <span className="num text-xs text-muted-foreground">{a.entitasId}</span></span> },
    { accessorKey: "ip", header: "IP", cell: ({ getValue }) => <span className="num text-xs text-muted-foreground">{getValue<string>()}</span> },
    { accessorKey: "hasil", header: "Hasil", filterFn: "equals", cell: ({ getValue }) => (getValue<string>() === "BERHASIL" ? <StatusBadge tone="normal">Berhasil</StatusBadge> : <StatusBadge tone="defisit">Ditolak</StatusBadge>) },
  ], []);
  const aksi = [...new Set((data ?? []).map((a) => a.aksi))];
  return (
    <div className="mx-auto max-w-[1600px] space-y-6">
      <PageHeader title="Audit trail" description="Jejak tak terhapus atas setiap aksi: validasi, publikasi, persetujuan, perubahan ambang, dan akses. Mendukung kepatuhan UU PDP dan SPBE." />
      <DataTable data={data} isLoading={isLoading} columns={kolom} getRowId={(r) => r.id} entityLabel="catatan" pageSize={25} searchPlaceholder="Cari aktor, aksi, atau ID entitas"
        initialSorting={[{ id: "waktu", desc: true }]}
        rowTone={(a) => (a.hasil === "DITOLAK" ? "defisit" : undefined)}
        facets={[{ columnId: "hasil", label: "Hasil", options: [{ value: "BERHASIL", label: "Berhasil" }, { value: "DITOLAK", label: "Ditolak" }] }, { columnId: "aksi", label: "Aksi", options: aksi.map((a) => ({ value: a, label: a })) }]}
        bulkActions={[
          { id: "tinjau", label: "Tandai untuk ditinjau", icon: Flag, perm: "audit.lihat", run: (r) => { toast.success(`${r.length} catatan ditandai`, { description: "Masuk ke antrean tinjauan Unit Monitoring & Risiko." }); } },
          { id: "ekspor", label: "Ekspor CSV", icon: Download, run: (r) => exportCsv(r, [{ header: "Waktu", value: (a) => a.waktu }, { header: "Aktor", value: (a) => a.aktor }, { header: "Peran", value: (a) => a.peran }, { header: "Aksi", value: (a) => a.aksi }, { header: "Entitas", value: (a) => `${a.entitas} ${a.entitasId}` }, { header: "Hasil", value: (a) => a.hasil }, { header: "IP", value: (a) => a.ip }], "audit-trail") },
        ]} />
    </div>
  );
}
