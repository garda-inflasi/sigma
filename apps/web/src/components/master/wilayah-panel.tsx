"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Download, Loader2 } from "lucide-react";
import { useState } from "react";
import type { BulkAction } from "@/components/data-table/bulk-action-bar";
import { DataTable } from "@/components/data-table/data-table";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { pesanGalatMaster } from "@/lib/api/master-perubahan";
import { exportCsv } from "@/lib/csv";
import type { MasterWilayahRow } from "@/lib/data/types";
import { formatAngka } from "@/lib/format";
import { useMasterWilayah, useWilayahDetail } from "@/lib/query/hooks";
import { CatatanSheet, DlBaris } from "./detail-baris";

const kolom: ColumnDef<MasterWilayahRow>[] = [
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
    id: "ibukota",
    header: "Ibu kota",
    accessorFn: (w) => `Ibu kota ${w.ibukota}`,
    cell: ({ getValue }) => (
      <span className="text-muted-foreground">{getValue<string>()}</span>
    ),
  },
  {
    id: "koordinat",
    header: "Koordinat",
    accessorFn: (w) => `${w.lat.toFixed(2)}, ${w.lng.toFixed(2)}`,
    cell: ({ getValue }) => (
      <span className="num text-xs">{getValue<string>()}</span>
    ),
  },
];

const actions: BulkAction<MasterWilayahRow>[] = [
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
          { header: "Ibu kota", value: (x) => x.ibukota },
          { header: "Lintang", value: (x) => x.lat },
          { header: "Bujur", value: (x) => x.lng },
        ],
        "master-wilayah",
      ),
  },
];

/** Tabel wilayah (kode BPS), baca-saja; klik baris menampilkan jumlah kecamatan dan pasar di wilayah itu. */
export function WilayahPanel() {
  const { data, isLoading } = useMasterWilayah();
  const [kode, setKode] = useState<string | null>(null);
  return (
    <>
      <DataTable
        data={data}
        isLoading={isLoading}
        columns={kolom}
        getRowId={(r) => r.id}
        entityLabel="data"
        bulkActions={actions}
        searchPlaceholder="Cari wilayah (kode bps)"
        onRowClick={(r) => setKode(r.kode)}
      />
      <WilayahSheet kode={kode} onClose={() => setKode(null)} />
    </>
  );
}

function WilayahSheet({
  kode,
  onClose,
}: {
  kode: string | null;
  onClose: () => void;
}) {
  const { data, isLoading, error } = useWilayahDetail(kode);
  return (
    <Sheet open={!!kode} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="glass-strong w-full gap-0 overflow-y-auto border-0 sm:max-w-md">
        <SheetHeader>
          <SheetTitle>{data?.wilayah.nama ?? "Detail wilayah"}</SheetTitle>
          <SheetDescription>{kode}</SheetDescription>
        </SheetHeader>
        <div className="space-y-5 px-4 pb-6 text-sm">
          {isLoading && (
            <Loader2 className="mx-auto size-5 animate-spin text-muted-foreground" />
          )}
          {error && (
            <CatatanSheet galat>{pesanGalatMaster(error)}</CatatanSheet>
          )}
          {data && (
            <dl className="grid grid-cols-[8rem_1fr] gap-x-3 gap-y-2">
              <DlBaris label="Kode">
                <span className="num">{data.wilayah.kode}</span>
              </DlBaris>
              <DlBaris label="Jenis">{data.jenis}</DlBaris>
              <DlBaris label="Ibu kota">{data.wilayah.ibukota}</DlBaris>
              <DlBaris label="Koordinat">
                <span className="num">
                  {data.wilayah.lat}, {data.wilayah.lng}
                </span>
              </DlBaris>
              <DlBaris label="Kecamatan">
                <span className="num">{formatAngka(data.jumlahKecamatan)}</span>
              </DlBaris>
              <DlBaris label="Pasar">
                <span className="num">{formatAngka(data.jumlahPasar)}</span>
              </DlBaris>
            </dl>
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}
