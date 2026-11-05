"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Download, Flag, Loader2 } from "lucide-react";
import { useMemo, useState } from "react";
import { toast } from "sonner";
import { DataTable } from "@/components/data-table/data-table";
import { PageHeader } from "@/components/garda/page-header";
import { StatusBadge } from "@/components/garda/status-badge";
import type { AuditFilter } from "@/lib/api/audit";
import { exportCsv } from "@/lib/csv";
import type { AuditRow } from "@/lib/data/types";
import { formatTanggal } from "@/lib/format";
import { useAudit, useEksporAudit } from "@/lib/query/hooks";
import { AuditDetailSheet } from "./audit-detail-sheet";
import { AuditFilterBar } from "./audit-filter-bar";

const HASIL_BADGE = {
  BERHASIL: <StatusBadge tone="normal">Berhasil</StatusBadge>,
  GAGAL: <StatusBadge tone="defisit">Gagal</StatusBadge>,
  DITOLAK: <StatusBadge tone="defisit">Ditolak</StatusBadge>,
} as const;

export function AuditPage() {
  const [filter, setFilter] = useState<AuditFilter>({});
  const { data, isLoading, hasNextPage, fetchNextPage, isFetchingNextPage } =
    useAudit(filter);
  const rows = useMemo(() => data?.pages.flatMap((p) => p.rows), [data]);
  const [detailId, setDetailId] = useState<string>();
  const ekspor = useEksporAudit();

  const kolom = useMemo<ColumnDef<AuditRow>[]>(
    () => [
      {
        accessorKey: "waktu",
        header: "Waktu",
        cell: ({ getValue }) => (
          <span className="num whitespace-nowrap text-xs">
            {formatTanggal(getValue<string>(), "d MMM yyyy HH.mm.ss")}
          </span>
        ),
      },
      {
        accessorKey: "aktor",
        header: "Aktor",
        cell: ({ row: { original: a } }) => (
          <div>
            <p className="font-medium">{a.aktor}</p>
            <p className="text-xs text-muted-foreground">{a.peran}</p>
          </div>
        ),
      },
      {
        accessorKey: "aksi",
        header: "Aksi",
        cell: ({ getValue }) => (
          <span className="num rounded-md bg-mist-100 px-1.5 py-0.5 text-[11px] dark:bg-white/5">
            {getValue<string>()}
          </span>
        ),
      },
      {
        id: "entitas",
        accessorFn: (a) => `${a.entitas} ${a.entitasId}`,
        header: "Entitas",
        cell: ({ row: { original: a } }) => (
          <span className="whitespace-nowrap">
            {a.entitas}{" "}
            <span className="num text-xs text-muted-foreground">
              {a.entitasId}
            </span>
          </span>
        ),
      },
      {
        accessorKey: "ip",
        header: "IP",
        cell: ({ getValue }) => (
          <span className="num text-xs text-muted-foreground">
            {getValue<string>()}
          </span>
        ),
      },
      {
        accessorKey: "hasil",
        header: "Hasil",
        cell: ({ getValue }) => HASIL_BADGE[getValue<AuditRow["hasil"]>()],
      },
    ],
    [],
  );

  return (
    <div className="mx-auto max-w-[1600px] space-y-6">
      <PageHeader
        title="Audit trail"
        description="Jejak tak terhapus atas setiap aksi: validasi, publikasi, persetujuan, perubahan ambang, dan akses. Mendukung kepatuhan UU PDP dan SPBE."
        actions={
          <button
            type="button"
            onClick={() => ekspor.mutate(filter)}
            disabled={ekspor.isPending}
            className="inline-flex h-10 items-center gap-2 rounded-xl border border-input bg-white/80 px-4 text-sm font-medium shadow-ambient-sm hover:bg-white disabled:opacity-60 dark:bg-white/[0.06]"
          >
            {ekspor.isPending ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              <Download className="size-4" />
            )}
            Ekspor CSV sesuai filter
          </button>
        }
      />
      <AuditFilterBar filter={filter} onChange={setFilter} />
      <AuditDetailSheet
        id={detailId}
        onClose={() => setDetailId(undefined)}
        onFilter={(patch) => setFilter((f) => ({ ...f, ...patch }))}
      />
      <DataTable
        onRowClick={(r) => setDetailId(r.id)}
        data={rows}
        isLoading={isLoading}
        columns={kolom}
        getRowId={(r) => r.id}
        entityLabel="catatan"
        pageSize={25}
        searchPlaceholder="Cari di catatan yang sudah dimuat"
        initialSorting={[{ id: "waktu", desc: true }]}
        rowTone={(a) => (a.hasil === "BERHASIL" ? undefined : "defisit")}
        bulkActions={[
          {
            id: "tinjau",
            label: "Tandai untuk ditinjau",
            icon: Flag,
            perm: "audit.lihat",
            run: (r) => {
              toast.success(`${r.length} catatan ditandai`, {
                description:
                  "Masuk ke antrean tinjauan Unit Monitoring & Risiko.",
              });
            },
          },
          {
            id: "ekspor",
            label: "Ekspor terpilih (CSV)",
            icon: Download,
            run: (r) =>
              exportCsv(
                r,
                [
                  { header: "Waktu", value: (a) => a.waktu },
                  { header: "Aktor", value: (a) => a.aktor },
                  { header: "Peran", value: (a) => a.peran },
                  { header: "Aksi", value: (a) => a.aksi },
                  {
                    header: "Entitas",
                    value: (a) => `${a.entitas} ${a.entitasId}`,
                  },
                  { header: "Hasil", value: (a) => a.hasil },
                  { header: "IP", value: (a) => a.ip },
                ],
                "audit-trail",
              ),
          },
        ]}
      />
      {hasNextPage && (
        <div className="flex justify-center">
          <button
            type="button"
            onClick={() => fetchNextPage()}
            disabled={isFetchingNextPage}
            className="inline-flex h-10 items-center gap-2 rounded-xl border border-input bg-white/80 px-5 text-sm font-medium shadow-ambient-sm hover:bg-white disabled:opacity-60 dark:bg-white/[0.06]"
          >
            {isFetchingNextPage && <Loader2 className="size-4 animate-spin" />}{" "}
            Muat catatan lebih lama
          </button>
        </div>
      )}
    </div>
  );
}
