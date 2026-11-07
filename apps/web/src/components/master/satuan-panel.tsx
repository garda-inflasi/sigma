"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Download } from "lucide-react";
import type { BulkAction } from "@/components/data-table/bulk-action-bar";
import { DataTable } from "@/components/data-table/data-table";
import type { SatuanRow } from "@/lib/api/master";
import { exportCsv } from "@/lib/csv";
import { formatAngka } from "@/lib/format";
import { useSatuan } from "@/lib/query/hooks";

const kolom: ColumnDef<SatuanRow>[] = [
  {
    accessorKey: "kode",
    header: "Kode",
    cell: ({ getValue }) => (
      <span className="num text-xs">{getValue<string>()}</span>
    ),
  },
  {
    accessorKey: "nama",
    header: "Nama",
    cell: ({ getValue }) => (
      <span className="font-medium">{getValue<string>()}</span>
    ),
  },
  {
    accessorKey: "jumlahKomoditas",
    header: "Komoditas memakai",
    cell: ({ getValue }) => (
      <span className="num">{formatAngka(getValue<number>())}</span>
    ),
  },
];

const actions: BulkAction<SatuanRow>[] = [
  {
    id: "ekspor",
    label: "Ekspor CSV",
    icon: Download,
    run: (r) =>
      exportCsv(
        r,
        [
          { header: "Kode", value: (x) => x.kode },
          { header: "Nama", value: (x) => x.nama },
          { header: "Komoditas memakai", value: (x) => x.jumlahKomoditas },
        ],
        "master-satuan",
      ),
  },
];

/** Satuan komoditas (daftar kecil, baca-saja): kode, nama, dan jumlah komoditas yang memakainya. */
export function SatuanPanel() {
  const { data, isLoading } = useSatuan();
  return (
    <DataTable
      data={data}
      isLoading={isLoading}
      columns={kolom}
      getRowId={(r) => r.id}
      entityLabel="satuan"
      bulkActions={actions}
      searchPlaceholder="Cari satuan"
    />
  );
}
