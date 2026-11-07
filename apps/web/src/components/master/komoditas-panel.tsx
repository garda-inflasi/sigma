"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Archive, CheckCircle2, Download, FilePen, Pencil } from "lucide-react";
import { useMemo, useState } from "react";
import type { BulkAction } from "@/components/data-table/bulk-action-bar";
import { DataTable } from "@/components/data-table/data-table";
import { KomoditasIcon } from "@/components/garda/komoditas-icon";
import { StatusBadge } from "@/components/garda/status-badge";
import type { MasterBulkAksi } from "@/lib/api/master";
import { useCan } from "@/lib/auth/session-context";
import { exportCsv } from "@/lib/csv";
import type { MasterKomoditasRow } from "@/lib/data/types";
import { formatRupiah } from "@/lib/format";
import { useBulkMasterKomoditas, useMasterKomoditas } from "@/lib/query/hooks";
import { KomoditasDetailSheet } from "./komoditas-detail-sheet";
import { STATUS_KOMODITAS } from "./status";
import { UsulanDialog } from "./usulan-dialog";

/** Tabel komoditas: klik baris membuka detail; aksi massal status (ajukan/setujui/arsipkan) dan ekspor CSV. */
export function KomoditasPanel() {
  const { data, isLoading } = useMasterKomoditas();
  const bulk = useBulkMasterKomoditas();
  const bisaAjukan = useCan("master.ajukan");
  const [detail, setDetail] = useState<string | null>(null);
  const [ubah, setUbah] = useState<MasterKomoditasRow | null>(null);

  const kolom = useMemo<ColumnDef<MasterKomoditasRow>[]>(
    () => [
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
        cell: ({ row: { original: r } }) => (
          <span className="flex items-center gap-2 font-medium">
            <KomoditasIcon kode={r.kode} className="text-muted-foreground" />
            {r.nama}
          </span>
        ),
      },
      {
        id: "kategori",
        header: "Kategori",
        accessorFn: (k) =>
          `${k.kategori}, per ${k.satuan}${k.pilot ? ", pilot" : ""}`,
        cell: ({ getValue }) => (
          <span className="text-muted-foreground">{getValue<string>()}</span>
        ),
      },
      {
        id: "hargaAcuan",
        header: "Harga acuan",
        accessorFn: (k) => formatRupiah(k.hargaAcuan),
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
        accessorKey: "status",
        header: "Status",
        filterFn: "equals",
        cell: ({ getValue }) => {
          const s = STATUS_KOMODITAS[getValue<MasterKomoditasRow["status"]>()];
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
                  aria-label={`Ajukan perubahan ${r.kode}`}
                  title={
                    r.status === "AKTIF"
                      ? "Ajukan perubahan"
                      : "Hanya komoditas aktif yang dapat diubah"
                  }
                  disabled={r.status !== "AKTIF"}
                  onClick={(e) => {
                    e.stopPropagation();
                    setUbah(r);
                  }}
                  className="grid size-8 place-items-center rounded-md text-muted-foreground hover:bg-secondary hover:text-foreground disabled:opacity-40"
                >
                  <Pencil className="size-4" />
                </button>
              ),
            } satisfies ColumnDef<MasterKomoditasRow>,
          ]
        : []),
    ],
    [bisaAjukan],
  );

  const kirim = (aksi: MasterBulkAksi) => (r: MasterKomoditasRow[]) =>
    bulk
      .mutateAsync({ aksi, kode: r.map((x) => x.kode) })
      .catch(() => undefined);
  const actions: BulkAction<MasterKomoditasRow>[] = [
    {
      id: "ajukan",
      label: "Ajukan perubahan",
      icon: FilePen,
      perm: "master.ajukan",
      enabled: (r) => r.every((x) => x.status === "AKTIF"),
      run: kirim("ajukan_perubahan"),
    },
    {
      id: "setujui",
      label: "Setujui",
      icon: CheckCircle2,
      perm: "master.setujui",
      tone: "data",
      enabled: (r) => r.every((x) => x.status === "MENUNGGU_PERSETUJUAN"),
      run: kirim("setujui"),
    },
    {
      id: "arsip",
      label: "Arsipkan",
      icon: Archive,
      perm: "master.ajukan",
      tone: "danger",
      enabled: (r) => r.every((x) => x.status !== "DIARSIPKAN"),
      confirm: {
        title: "Arsipkan data terpilih?",
        description: (n) =>
          `${n} data tidak lagi muncul di form lapangan dan tidak dapat dipulihkan dari layar ini. Riwayat entri lama tetap tersimpan.`,
        actionLabel: "Arsipkan",
      },
      run: kirim("arsipkan"),
    },
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
            { header: "Kategori", value: (x) => x.kategori },
            { header: "Satuan", value: (x) => x.satuan },
            { header: "Harga acuan", value: (x) => x.hargaAcuan },
            { header: "Umur simpan (hari)", value: (x) => x.shelfLifeHari },
            { header: "Pilot", value: (x) => (x.pilot ? "Ya" : "Tidak") },
            { header: "Versi", value: (x) => x.versi },
            { header: "Status", value: (x) => x.status },
          ],
          "master-komoditas",
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
        searchPlaceholder="Cari komoditas"
        onRowClick={(r) => setDetail(r.kode)}
        facets={[
          {
            columnId: "status",
            label: "Status",
            options: Object.entries(STATUS_KOMODITAS).map(([value, v]) => ({
              value,
              label: v.label,
            })),
          },
        ]}
      />
      <KomoditasDetailSheet
        kode={detail}
        onClose={() => setDetail(null)}
        onUbah={setUbah}
      />
      <UsulanDialog
        target={ubah ? { entitas: "komoditas", awal: ubah } : null}
        open={!!ubah}
        onOpenChange={(o) => !o && setUbah(null)}
      />
    </>
  );
}
