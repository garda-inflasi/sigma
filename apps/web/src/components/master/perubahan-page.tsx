"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { useMemo, useState } from "react";
import { DataTable, SelTeks } from "@/components/data-table/data-table";
import { HalamanPas } from "@/components/garda/halaman-pas";
import { PageHeader } from "@/components/garda/page-header";
import { Segmented } from "@/components/garda/segmented";
import { StatusBadge } from "@/components/garda/status-badge";
import { Button } from "@/components/ui/button";
import {
  ENTITAS_LABEL,
  JENIS_LABEL,
  ringkasDiff,
  statusTampil,
  type EntitasPerubahan,
  type PerubahanMaster,
  type PerubahanParams,
  type StatusPerubahan,
} from "@/lib/api/master-perubahan";
import { formatTanggal } from "@/lib/format";
import { usePerubahanHalaman } from "@/lib/query/hooks";
import { FilterSelect } from "./filter-select";
import { MasterTabs } from "./master-tabs";
import { PerubahanDetailSheet } from "./perubahan-detail-sheet";
import { useTabelServer } from "./tabel-server";

type Tab = "menunggu" | "riwayat";

const kolom: ColumnDef<PerubahanMaster>[] = [
  {
    id: "data",
    header: "Data",
    enableSorting: false,
    accessorFn: (p) => p.entitasNama ?? p.entitasId,
    cell: ({ row: { original: p } }) => (
      <span className="block">
        <SelTeks className="font-medium">
          {p.entitasNama ?? p.entitasId}
        </SelTeks>
        <SelTeks className="num text-xs text-muted-foreground">
          {p.entitasNama
            ? `${ENTITAS_LABEL[p.entitas]} · ${p.entitasId}`
            : ENTITAS_LABEL[p.entitas]}
        </SelTeks>
      </span>
    ),
  },
  {
    id: "jenis",
    header: "Jenis",
    enableSorting: false,
    accessorFn: (p) => JENIS_LABEL[p.jenis],
    cell: ({ getValue }) => <span>{getValue<string>()}</span>,
  },
  {
    id: "ringkasan",
    header: "Perubahan",
    enableSorting: false,
    accessorFn: (p) => ringkasDiff(p),
    cell: ({ getValue }) => (
      <SelTeks className="max-w-[24rem] text-muted-foreground">
        {getValue<string>()}
      </SelTeks>
    ),
  },
  {
    id: "pengaju",
    header: "Pengaju",
    enableSorting: false,
    accessorFn: (p) => p.diajukanOleh?.nama ?? "—",
    cell: ({ row: { original: p } }) => (
      <span className="block">
        <SelTeks className="max-w-[12rem]">
          {p.diajukanOleh?.nama ?? "—"}
        </SelTeks>
        <span className="num block text-xs whitespace-nowrap text-muted-foreground">
          {formatTanggal(p.dibuatPada, "d MMM yyyy HH.mm")}
        </span>
      </span>
    ),
  },
  {
    id: "status",
    header: "Status",
    enableSorting: false,
    accessorFn: (p) => statusTampil(p).label,
    cell: ({ row: { original: p } }) => {
      const s = statusTampil(p);
      return <StatusBadge tone={s.tone}>{s.label}</StatusBadge>;
    },
  },
];

/**
 * Antrean persetujuan perubahan master. "Menunggu": usulan yang belum diputuskan, terlama dahulu. "Riwayat":
 * seluruh usulan terbaru dahulu (tanpa filter status ikut yang masih menunggu); saring Disetujui/Ditolak untuk
 * keputusan saja. Daftar dipaginasi server (`X-Total-Count`); server tidak mendukung pencarian teks.
 */
export function PerubahanPage() {
  const { state, setFilter, tabel } = useTabelServer();
  const [detail, setDetail] = useState<string | null>(null);
  const tab: Tab = state.filter.tab === "riwayat" ? "riwayat" : "menunggu";
  const entitas = state.filter.entitas as EntitasPerubahan | undefined;
  const params = useMemo<PerubahanParams>(
    () => ({
      status:
        tab === "menunggu"
          ? "MENUNGGU"
          : (state.filter.status as StatusPerubahan | undefined),
      entitas,
      page: state.page,
      limit: state.limit,
    }),
    [tab, state.filter.status, entitas, state.page, state.limit],
  );
  const { data, isLoading, isPlaceholderData } = usePerubahanHalaman(params);
  const gantiTab = (v: Tab) => {
    setFilter("tab", v);
    setFilter("status", undefined);
  };

  return (
    <HalamanPas>
      <PageHeader
        title="Master data"
        description="Setujui atau tolak usulan dari pengaju lain."
        tabs={<MasterTabs aktif="perubahan" />}
      />
      <DataTable
        isiPenuh
        data={data?.rows}
        isLoading={isLoading}
        columns={kolom}
        getRowId={(r) => r.id}
        entityLabel="usulan"
        tanpaCari
        onRowClick={(r) => setDetail(r.id)}
        server={tabel(data?.total ?? 0, isPlaceholderData)}
        empty={
          tab === "menunggu"
            ? {
                title: "Tidak ada usulan yang menunggu",
                description:
                  "Semua usulan perubahan sudah diputuskan. Keputusan sebelumnya ada di riwayat.",
                action: (
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => gantiTab("riwayat")}
                  >
                    Lihat riwayat
                  </Button>
                ),
              }
            : undefined
        }
        toolbar={
          <div className="flex w-full flex-wrap items-center gap-2 lg:w-auto">
            <Segmented<Tab>
              layoutId="perubahan-tab"
              value={tab}
              onChange={gantiTab}
              options={[
                { value: "menunggu", label: "Menunggu" },
                { value: "riwayat", label: "Riwayat" },
              ]}
            />
            <FilterSelect
              label="Entitas"
              value={entitas}
              onChange={(v) => setFilter("entitas", v)}
              options={(Object.keys(ENTITAS_LABEL) as EntitasPerubahan[]).map(
                (e) => ({ value: e, label: ENTITAS_LABEL[e] }),
              )}
            />
            {tab === "riwayat" && (
              <FilterSelect
                label="Status"
                value={state.filter.status}
                onChange={(v) => setFilter("status", v)}
                options={[
                  { value: "DISETUJUI", label: "Disetujui" },
                  { value: "DITOLAK", label: "Ditolak atau dibatalkan" },
                ]}
              />
            )}
          </div>
        }
      />
      <PerubahanDetailSheet id={detail} onClose={() => setDetail(null)} />
    </HalamanPas>
  );
}
