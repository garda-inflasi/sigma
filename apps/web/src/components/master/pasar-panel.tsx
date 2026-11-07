"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Download, Pencil } from "lucide-react";
import { useMemo, useState } from "react";
import type { BulkAction } from "@/components/data-table/bulk-action-bar";
import { DataTable } from "@/components/data-table/data-table";
import { StatusBadge } from "@/components/garda/status-badge";
import { useCan } from "@/lib/auth/session-context";
import { exportCsv } from "@/lib/csv";
import type { MasterPasarRow } from "@/lib/data/types";
import { useMasterPasar } from "@/lib/query/hooks";
import { PasarDetailSheet } from "./pasar-detail-sheet";
import { STATUS_PASAR } from "./status";
import { UsulanDialog } from "./usulan-dialog";

type StatusPasar = keyof typeof STATUS_PASAR;
const statusPasar = (p: MasterPasarRow): StatusPasar =>
  p.aktif ? "AKTIF" : "NONAKTIF";

/** Tabel pasar dengan kolom kecamatan dan versi; klik baris membuka detail, ikon pensil mengajukan perubahan. */
export function PasarPanel() {
  const { data, isLoading } = useMasterPasar();
  const bisaAjukan = useCan("master.ajukan");
  const [detail, setDetail] = useState<string | null>(null);
  const [ubah, setUbah] = useState<MasterPasarRow | null>(null);

  const kolom = useMemo<ColumnDef<MasterPasarRow>[]>(
    () => [
      {
        accessorKey: "id",
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
        accessorKey: "kabupaten",
        header: "Kabupaten/kota",
        cell: ({ getValue }) => (
          <span className="text-muted-foreground">{getValue<string>()}</span>
        ),
      },
      {
        id: "kecamatan",
        header: "Kecamatan",
        accessorFn: (p) => p.kecamatanNama ?? "",
        cell: ({ getValue }) =>
          getValue<string>() ? (
            <span>{getValue<string>()}</span>
          ) : (
            <span className="text-muted-foreground">—</span>
          ),
      },
      {
        id: "geofence",
        header: "Geofence",
        accessorFn: (p) => `Geofence ${p.radiusMeter} m`,
        cell: ({ getValue }) => (
          <span className="num text-xs">{getValue<string>()}</span>
        ),
      },
      {
        accessorKey: "versi",
        header: "Versi",
        cell: ({ getValue }) => (
          <span className="num">v{getValue<number>()}</span>
        ),
      },
      {
        id: "status",
        header: "Status",
        accessorFn: statusPasar,
        filterFn: "equals",
        cell: ({ getValue }) => {
          const s = STATUS_PASAR[getValue<StatusPasar>()];
          return <StatusBadge tone={s.tone}>{s.label}</StatusBadge>;
        },
      },
      ...(bisaAjukan
        ? [
            {
              id: "aksi",
              header: () => <span className="sr-only">Aksi</span>,
              cell: ({ row: { original: r } }) => (
                <button
                  type="button"
                  aria-label={`Ajukan perubahan ${r.id}`}
                  title="Ajukan perubahan"
                  onClick={(e) => {
                    e.stopPropagation();
                    setUbah(r);
                  }}
                  className="grid size-8 place-items-center rounded-md text-muted-foreground hover:bg-secondary hover:text-foreground"
                >
                  <Pencil className="size-4" />
                </button>
              ),
            } satisfies ColumnDef<MasterPasarRow>,
          ]
        : []),
    ],
    [bisaAjukan],
  );

  // Pasar tidak punya siklus status massal (hanya usulan per pasar); aksi massalnya cukup ekspor CSV.
  const actions: BulkAction<MasterPasarRow>[] = [
    {
      id: "ekspor",
      label: "Ekspor CSV",
      icon: Download,
      run: (r) =>
        exportCsv(
          r,
          [
            { header: "Kode", value: (x) => x.id },
            { header: "Nama", value: (x) => x.nama },
            { header: "Kabupaten/kota", value: (x) => x.kabupaten },
            { header: "Kecamatan", value: (x) => x.kecamatanNama },
            { header: "Kode kecamatan", value: (x) => x.kecamatanKode },
            { header: "Lintang", value: (x) => x.lat },
            { header: "Bujur", value: (x) => x.lng },
            { header: "Radius geofence (m)", value: (x) => x.radiusMeter },
            { header: "Versi", value: (x) => x.versi },
            { header: "Status", value: (x) => statusPasar(x) },
          ],
          "master-pasar",
        ),
    },
  ];

  return (
    <>
      <DataTable
        data={data}
        isLoading={isLoading}
        columns={kolom}
        getRowId={(r) => r.id}
        entityLabel="data"
        bulkActions={actions}
        searchPlaceholder="Cari pasar"
        onRowClick={(r) => setDetail(r.id)}
        facets={[
          {
            columnId: "status",
            label: "Status",
            options: Object.entries(STATUS_PASAR).map(([value, v]) => ({
              value,
              label: v.label,
            })),
          },
        ]}
      />
      <PasarDetailSheet
        kode={detail}
        onClose={() => setDetail(null)}
        onUbah={setUbah}
      />
      <UsulanDialog
        target={ubah ? { entitas: "pasar", awal: ubah } : null}
        open={!!ubah}
        onOpenChange={(o) => !o && setUbah(null)}
      />
    </>
  );
}
