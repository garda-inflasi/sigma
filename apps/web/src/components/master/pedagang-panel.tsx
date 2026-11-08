"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Download, Loader2 } from "lucide-react";
import { useMemo, useState } from "react";
import { toast } from "sonner";
import type { BulkAction } from "@/components/data-table/bulk-action-bar";
import { DataTable } from "@/components/data-table/data-table";
import { SpringButton } from "@/components/garda/spring-button";
import { StatusBadge } from "@/components/garda/status-badge";
import { api } from "@/lib/api/endpoints";
import type { PedagangParams, PedagangRow } from "@/lib/api/master";
import { ambilSemuaHalaman } from "@/lib/api/master-api";
import { pesanGalatMaster } from "@/lib/api/master-perubahan";
import { exportCsv } from "@/lib/csv";
import { formatAngka } from "@/lib/format";
import { useMasterPasar, usePedagangHalaman } from "@/lib/query/hooks";
import { FilterSelect } from "./filter-select";
import { STATUS_PASAR } from "./status";
import { useTabelServer } from "./tabel-server";

const kolomCsv = [
  { header: "Kode", value: (x: PedagangRow) => x.kode },
  { header: "Nama", value: (x: PedagangRow) => x.nama },
  { header: "Pasar", value: (x: PedagangRow) => x.pasarNama },
  { header: "ID pasar", value: (x: PedagangRow) => x.pasarId },
  {
    header: "Status",
    value: (x: PedagangRow) => (x.aktif ? "Aktif" : "Nonaktif"),
  },
  { header: "Jumlah entri", value: (x: PedagangRow) => x.jumlahEntri },
];

/**
 * Pedagang per pasar, dengan pencarian, filter pasar dan status, serta paginasi di server. Ekspor CSV seperti
 * tab kecamatan: baris terpilih, atau seluruh hasil lewat "Ekspor semua hasil".
 */
export function PedagangPanel() {
  const { state, setFilter, tabel } = useTabelServer();
  const params = useMemo<PedagangParams>(
    () => ({
      pasarId: state.filter.pasar,
      aktif:
        state.filter.status === undefined
          ? undefined
          : state.filter.status === "AKTIF",
      q: state.search,
    }),
    [state.filter.pasar, state.filter.status, state.search],
  );
  const { data, isLoading, isPlaceholderData } = usePedagangHalaman({
    ...params,
    page: state.page,
    limit: state.limit,
  });
  const pasar = useMasterPasar();
  const [mengekspor, setMengekspor] = useState(false);

  const kolom = useMemo<ColumnDef<PedagangRow>[]>(
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
        cell: ({ getValue }) =>
          getValue<string | null>() ?? (
            <span className="text-muted-foreground">Tanpa nama</span>
          ),
      },
      {
        accessorKey: "pasarNama",
        header: "Pasar",
        enableSorting: false,
        cell: ({ getValue }) => (
          <span className="text-muted-foreground">{getValue<string>()}</span>
        ),
      },
      {
        accessorKey: "aktif",
        header: "Status",
        enableSorting: false,
        cell: ({ getValue }) => {
          const s = STATUS_PASAR[getValue<boolean>() ? "AKTIF" : "NONAKTIF"];
          return <StatusBadge tone={s.tone}>{s.label}</StatusBadge>;
        },
      },
      {
        accessorKey: "jumlahEntri",
        header: "Entri harga",
        enableSorting: false,
        cell: ({ getValue }) => (
          <span className="num">{formatAngka(getValue<number>())}</span>
        ),
      },
    ],
    [],
  );

  const actions: BulkAction<PedagangRow>[] = [
    {
      id: "ekspor",
      label: "Ekspor CSV",
      icon: Download,
      run: (r) => exportCsv(r, kolomCsv, "master-pedagang"),
    },
  ];

  async function eksporSemua() {
    setMengekspor(true);
    try {
      const semua = await ambilSemuaHalaman((page, limit) =>
        api.master.pedagang({ ...params, page, limit }),
      );
      exportCsv(semua, kolomCsv, "master-pedagang");
    } catch (err) {
      toast.error("Ekspor pedagang gagal", {
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
      entityLabel="pedagang"
      bulkActions={actions}
      searchPlaceholder="Cari pedagang"
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
        <div className="flex flex-wrap gap-2">
          <FilterSelect
            label="Pasar"
            value={state.filter.pasar}
            onChange={(v) => setFilter("pasar", v)}
            options={(pasar.data ?? []).map((p) => ({
              value: p.kode ?? p.id,
              label: p.nama,
            }))}
            disabled={pasar.isLoading}
          />
          <FilterSelect
            label="Status"
            value={state.filter.status}
            onChange={(v) => setFilter("status", v)}
            options={Object.entries(STATUS_PASAR).map(([value, s]) => ({
              value,
              label: s.label,
            }))}
          />
        </div>
      }
    />
  );
}
