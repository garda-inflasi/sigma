"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Download, Loader2 } from "lucide-react";
import { useMemo, useState } from "react";
import type { BulkAction } from "@/components/data-table/bulk-action-bar";
import { DataTable } from "@/components/data-table/data-table";
import { SpringButton } from "@/components/garda/spring-button";
import { toast } from "sonner";
import type { KecamatanRow } from "@/lib/api/master";
import { ambilSemuaHalaman } from "@/lib/api/master-api";
import { pesanGalatMaster } from "@/lib/api/master-perubahan";
import { api } from "@/lib/api/endpoints";
import { exportCsv } from "@/lib/csv";
import { useKecamatanHalaman, useMasterWilayah } from "@/lib/query/hooks";
import { FilterSelect } from "./filter-select";
import { useTabelServer } from "./tabel-server";

const kolomCsv = [
  { header: "Kode", value: (x: KecamatanRow) => x.kode },
  { header: "Nama", value: (x: KecamatanRow) => x.nama },
  { header: "Kabupaten/kota", value: (x: KecamatanRow) => x.kabupatenNama },
  { header: "Kode kabupaten", value: (x: KecamatanRow) => x.kabupatenKode },
  { header: "Kode Kemendagri", value: (x: KecamatanRow) => x.kodeDagri },
];

/**
 * Kecamatan se-Aceh (sekitar 290 baris): pencarian, filter kabupaten/kota, dan paginasi dijalankan server
 * (`X-Total-Count` untuk total), jadi tidak ada seluruh baris yang dimuat. Ekspor CSV: baris terpilih (satu halaman)
 * lewat bilah aksi, atau seluruh hasil pencarian/filter saat ini lewat tombol "Ekspor semua hasil" (halaman
 * diambil bertahap).
 */
export function KecamatanPanel() {
  const { state, setFilter, tabel } = useTabelServer();
  const kabupaten = state.filter.kabupaten;
  const { data, isLoading, isPlaceholderData } = useKecamatanHalaman({
    kabupatenKode: kabupaten,
    q: state.search,
    page: state.page,
    limit: state.limit,
  });
  const wilayah = useMasterWilayah();
  const [mengekspor, setMengekspor] = useState(false);

  const kolom = useMemo<ColumnDef<KecamatanRow>[]>(
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
        header: "Nama",
        enableSorting: false,
        cell: ({ getValue }) => (
          <span className="font-medium">{getValue<string>()}</span>
        ),
      },
      {
        accessorKey: "kabupatenNama",
        header: "Kabupaten/kota",
        enableSorting: false,
        cell: ({ getValue }) => (
          <span className="text-muted-foreground">{getValue<string>()}</span>
        ),
      },
      {
        accessorKey: "kodeDagri",
        header: "Kode Kemendagri",
        enableSorting: false,
        cell: ({ getValue }) => (
          <span className="num text-xs">
            {getValue<string | null>() ?? "—"}
          </span>
        ),
      },
    ],
    [],
  );

  const actions: BulkAction<KecamatanRow>[] = [
    {
      id: "ekspor",
      label: "Ekspor CSV",
      icon: Download,
      run: (r) => exportCsv(r, kolomCsv, "master-kecamatan"),
    },
  ];

  async function eksporSemua() {
    setMengekspor(true);
    try {
      const semua = await ambilSemuaHalaman((page, limit) =>
        api.master.kecamatan({
          kabupatenKode: kabupaten,
          q: state.search,
          page,
          limit,
        }),
      );
      exportCsv(semua, kolomCsv, "master-kecamatan");
    } catch (err) {
      toast.error("Ekspor kecamatan gagal", {
        description: pesanGalatMaster(err),
      });
    } finally {
      setMengekspor(false);
    }
  }

  return (
    <DataTable
      data={data?.rows}
      isLoading={isLoading}
      columns={kolom}
      getRowId={(r) => r.id}
      entityLabel="kecamatan"
      bulkActions={actions}
      searchPlaceholder="Cari kecamatan"
      server={tabel(data?.total ?? 0, isPlaceholderData)}
      toolbar={
        <SpringButton
          variant="soft"
          size="md"
          onClick={() => void eksporSemua()}
          disabled={mengekspor}
        >
          {mengekspor ? <Loader2 className="animate-spin" /> : <Download />}{" "}
          Ekspor semua hasil
        </SpringButton>
      }
      filterBar={
        <FilterSelect
          label="Kabupaten/kota"
          value={kabupaten}
          onChange={(v) => setFilter("kabupaten", v)}
          options={(wilayah.data ?? []).map((w) => ({
            value: w.kode,
            label: w.nama,
          }))}
          disabled={wilayah.isLoading}
        />
      }
    />
  );
}
