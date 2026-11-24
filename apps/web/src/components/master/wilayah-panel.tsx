"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Download } from "lucide-react";
import { useState } from "react";
import type { BulkAction } from "@/components/data-table/bulk-action-bar";
import { DataTable } from "@/components/data-table/data-table";
import {
  InspectorRingkasan,
  InspectorSection,
  MetaGrid,
  MetaItem,
} from "@/components/garda/inspector";
import {
  Sheet,
  SheetBody,
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
import { CatatanSheet, MemuatSheet, formatKoordinat } from "./detail-baris";

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
    accessorFn: (w) => w.ibukota,
    cell: ({ getValue }) => (
      <span className="text-muted-foreground">{getValue<string>()}</span>
    ),
  },
  {
    id: "koordinat",
    header: "Koordinat",
    accessorFn: (w) => formatKoordinat(w.lat, w.lng),
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
        isiPenuh
        data={data}
        isLoading={isLoading}
        columns={kolom}
        getRowId={(r) => r.id}
        entityLabel="wilayah"
        bulkActions={actions}
        searchPlaceholder="Cari nama atau kode BPS"
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
      <SheetContent className="sm:max-w-lg">
        <SheetHeader>
          <SheetTitle>{data?.wilayah.nama ?? "Detail wilayah"}</SheetTitle>
          <SheetDescription>{kode}</SheetDescription>
        </SheetHeader>
        <SheetBody>
          {isLoading && <MemuatSheet />}
          {error && (
            <div className="px-5 py-4">
              <CatatanSheet galat>{pesanGalatMaster(error)}</CatatanSheet>
            </div>
          )}
          {data && (
            <>
              <InspectorRingkasan
                className="border-b-0"
                nilai={`${formatAngka(data.jumlahPasar)} pasar`}
                keterangan={`${formatAngka(data.jumlahKecamatan)} kecamatan · ${data.jenis}`}
              />
              <InspectorSection title="Rincian">
                <MetaGrid>
                  <MetaItem label="Kode">
                    <span className="num">{data.wilayah.kode}</span>
                  </MetaItem>
                  <MetaItem label="Jenis">{data.jenis}</MetaItem>
                  <MetaItem label="Ibu kota">{data.wilayah.ibukota}</MetaItem>
                  <MetaItem label="Koordinat">
                    <span className="num">
                      {formatKoordinat(data.wilayah.lat, data.wilayah.lng)}
                    </span>
                  </MetaItem>
                  <MetaItem label="Kecamatan">
                    <span className="num">
                      {formatAngka(data.jumlahKecamatan)}
                    </span>
                  </MetaItem>
                  <MetaItem label="Pasar">
                    <span className="num">{formatAngka(data.jumlahPasar)}</span>
                  </MetaItem>
                </MetaGrid>
              </InspectorSection>
            </>
          )}
        </SheetBody>
      </SheetContent>
    </Sheet>
  );
}
