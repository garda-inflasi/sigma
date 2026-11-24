"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Download, MapPin, Pencil, Trash2 } from "lucide-react";
import { useMemo, useState } from "react";
import type { BulkAction } from "@/components/data-table/bulk-action-bar";
import { DataTable, SelTeks } from "@/components/data-table/data-table";
import { StatusBadge } from "@/components/garda/status-badge";
import type { LokasiSpasialRow } from "@/lib/api/master-langsung";
import type { JenisTitikDesa } from "@/lib/api/spasial-jarak";
import { useOptionalSession } from "@/lib/auth/session-context";
import { exportCsv } from "@/lib/csv";
import { formatAngka } from "@/lib/format";
import {
  useHapusMasterLangsung,
  useLokasiSpasial,
  useMasterWilayah,
} from "@/lib/query/hooks";
import { FilterSelect } from "./filter-select";
import { LangsungDialog } from "./langsung-dialog";
import { useTabelServer } from "./tabel-server";

export function LokasiSpasialPanel({ jenis }: { jenis: JenisTitikDesa }) {
  const sesi = useOptionalSession();
  const isSuperadmin = sesi?.role === "ADMIN_SISTEM";
  const { state, setFilter, tabel } = useTabelServer();
  const kabupatenKode = state.filter.kabupaten;
  const { data, isLoading, isPlaceholderData } = useLokasiSpasial({
    jenis,
    kabupatenKode,
    q: state.search,
    page: state.page,
    limit: state.limit,
  });
  const wilayah = useMasterWilayah();
  const hapus = useHapusMasterLangsung();
  const [editRow, setEditRow] = useState<LokasiSpasialRow | null>(null);

  const entitas = jenis === "PERTANIAN" ? "pertanian" : "perkampungan";

  const kolom = useMemo<ColumnDef<LokasiSpasialRow>[]>(
    () => [
      {
        accessorKey: "kode",
        header: "Kode",
        enableSorting: false,
        cell: ({ getValue }) => (
          <span className="num text-xs">{getValue<string>()}</span>
        ),
      },
      {
        accessorKey: "nama",
        header: "Nama lokasi",
        enableSorting: false,
        cell: ({ getValue }) => (
          <SelTeks className="font-medium">{getValue<string>()}</SelTeks>
        ),
      },
      {
        accessorKey: "kabupaten_nama",
        header: "Kabupaten/Kota",
        enableSorting: false,
        cell: ({ getValue }) => (
          <SelTeks className="text-muted-foreground">
            {getValue<string>()}
          </SelTeks>
        ),
      },
      {
        accessorKey: "komoditas_utama",
        header: jenis === "PERTANIAN" ? "Komoditas utama" : "Fokus pangan",
        enableSorting: false,
        cell: ({ getValue }) => <SelTeks>{getValue<string>() || "—"}</SelTeks>,
      },
      {
        id: "kapasitas",
        header: jenis === "PERTANIAN" ? "Luas / Panen" : "Populasi",
        enableSorting: false,
        cell: ({ row: { original: r } }) =>
          jenis === "PERTANIAN" ? (
            <span className="num text-xs">
              {r.luas_ha ? `${formatAngka(r.luas_ha, 1)} ha` : "—"}
              {r.estimasi_panen_ton
                ? ` · ${formatAngka(r.estimasi_panen_ton, 0)} ton`
                : ""}
            </span>
          ) : (
            <span className="num text-xs">
              {r.populasi ? `${formatAngka(r.populasi)} jiwa` : "—"}
            </span>
          ),
      },
      {
        id: "koordinat",
        header: "Koordinat",
        enableSorting: false,
        cell: ({ row: { original: r } }) => (
          <span className="inline-flex items-center gap-1 font-mono text-[11px] text-muted-foreground tabular-nums">
            <MapPin className="size-3" />
            {r.lat.toFixed(4)}, {r.lng.toFixed(4)}
          </span>
        ),
      },
      {
        id: "status",
        header: "Status",
        enableSorting: false,
        cell: ({ row: { original: r } }) => (
          <StatusBadge tone={r.aktif ? "normal" : "netral"}>
            {r.aktif ? "Aktif" : "Nonaktif"}
          </StatusBadge>
        ),
      },
      ...(isSuperadmin
        ? [
            {
              id: "aksi",
              meta: { sematKanan: true },
              header: () => <span className="sr-only">Aksi</span>,
              cell: ({ row: { original: r } }) => (
                <div className="flex items-center justify-end gap-1">
                  <button
                    type="button"
                    aria-label={`Ubah ${r.kode}`}
                    onClick={(e) => {
                      e.stopPropagation();
                      setEditRow(r);
                    }}
                    className="grid size-8 place-items-center rounded-md text-muted-foreground hover:bg-secondary hover:text-foreground"
                  >
                    <Pencil className="size-3.5" />
                  </button>
                  <button
                    type="button"
                    aria-label={`Hapus ${r.kode}`}
                    onClick={(e) => {
                      e.stopPropagation();
                      hapus.mutate({ entitas, kode: r.kode });
                    }}
                    className="grid size-8 place-items-center rounded-md text-muted-foreground hover:bg-rose-50 hover:text-rose-600"
                  >
                    <Trash2 className="size-3.5" />
                  </button>
                </div>
              ),
            } satisfies ColumnDef<LokasiSpasialRow>,
          ]
        : []),
    ],
    [entitas, hapus, isSuperadmin, jenis],
  );

  const actions: BulkAction<LokasiSpasialRow>[] = [
    {
      id: "ekspor",
      label: "Ekspor CSV",
      icon: Download,
      run: (rows) =>
        exportCsv(
          rows,
          [
            { header: "Kode", value: (r) => r.kode },
            { header: "Jenis", value: (r) => r.jenis },
            { header: "Nama", value: (r) => r.nama },
            { header: "Kabupaten", value: (r) => r.kabupaten_nama },
            { header: "Komoditas", value: (r) => r.komoditas_utama },
            { header: "Lat", value: (r) => r.lat },
            { header: "Lng", value: (r) => r.lng },
          ],
          `master-${entitas}`,
        ),
    },
  ];

  return (
    <>
      <DataTable
        isiPenuh
        data={data?.data}
        isLoading={isLoading}
        columns={kolom}
        getRowId={(r) => r.kode}
        entityLabel={jenis === "PERTANIAN" ? "sentra pertanian" : "perkampungan"}
        bulkActions={actions}
        searchPlaceholder={
          jenis === "PERTANIAN"
            ? "Cari sentra pertanian atau komoditas…"
            : "Cari perkampungan atau gampong…"
        }
        filterBar={
          <FilterSelect
            label="Kabupaten/kota"
            value={kabupatenKode}
            onChange={(v) => setFilter("kabupaten", v)}
            options={(wilayah.data ?? []).map((w) => ({
              value: w.kode,
              label: w.nama,
            }))}
            disabled={wilayah.isLoading}
          />
        }
        server={tabel(data?.total ?? 0, isPlaceholderData)}
        empty={{
          title:
            jenis === "PERTANIAN"
              ? "Belum ada sentra pertanian"
              : "Belum ada data perkampungan",
          description:
            "Tambahkan titik lokasi langsung melalui tombol Tambah sebagai Superadmin.",
        }}
      />
      <LangsungDialog
        target={editRow ? { entitas, awal: editRow } : null}
        open={Boolean(editRow)}
        onOpenChange={(o) => !o && setEditRow(null)}
      />
    </>
  );
}
