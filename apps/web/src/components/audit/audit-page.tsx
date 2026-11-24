"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Download, Flag, Loader2 } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useMemo, useState } from "react";
import { toast } from "sonner";
import { DataTable, SelTeks } from "@/components/data-table/data-table";
import {
  HalamanPas,
  RingkasanKeputusan,
  type ItemRingkas,
} from "@/components/garda/halaman-pas";
import { PageHeader } from "@/components/garda/page-header";
import { StatusBadge } from "@/components/garda/status-badge";
import { Button } from "@/components/ui/button";
import { filterAuditDariQuery, type AuditFilter } from "@/lib/api/audit";
import { exportCsv } from "@/lib/csv";
import type { AuditRow } from "@/lib/data/types";
import { formatTanggal } from "@/lib/format";
import { useAudit, useEksporAudit } from "@/lib/query/hooks";
import { AuditDetailSheet } from "./audit-detail-sheet";
import { AuditFilterBar } from "./audit-filter-bar";
import { AuditIntegritasCard } from "./audit-integritas-card";

const HASIL_BADGE = {
  BERHASIL: <StatusBadge tone="normal">Berhasil</StatusBadge>,
  GAGAL: <StatusBadge tone="defisit">Gagal</StatusBadge>,
  DITOLAK: <StatusBadge tone="defisit">Ditolak</StatusBadge>,
} as const;

export function AuditPage() {
  // Tautan dari halaman lain (mis. riwayat peran pengguna) membuka audit yang sudah disaring lewat kueri URL.
  const params = useSearchParams();
  const [filter, setFilter] = useState<AuditFilter>(() =>
    filterAuditDariQuery(params),
  );
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
          <div className="min-w-0">
            <SelTeks className="font-medium">{a.aktor}</SelTeks>
            <SelTeks className="text-xs text-muted-foreground">
              {a.peran}
            </SelTeks>
          </div>
        ),
      },
      {
        accessorKey: "aksi",
        header: "Aksi",
        cell: ({ getValue }) => (
          <span className="num rounded-md bg-secondary px-1.5 py-0.5 text-[11px]">
            {getValue<string>()}
          </span>
        ),
      },
      {
        id: "entitas",
        accessorFn: (a) => `${a.entitas} ${a.entitasId}`,
        header: "Entitas",
        cell: ({ row: { original: a } }) => (
          <span
            title={`${a.entitas} ${a.entitasId}`}
            className="block max-w-[16rem] truncate"
          >
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

  // Audit memakai paginasi cursor: angka hanya mencakup catatan yang sudah dimuat, dan itu disebut di tiap sel.
  const dimuat = rows ?? [];
  const cakupan = hasNextPage ? "yang dimuat" : "semua hasil";
  const selHasil = (
    label: string,
    hasil: "GAGAL" | "DITOLAK",
    arti: string,
  ): ItemRingkas => {
    const jumlah = dimuat.filter((a) => a.hasil === hasil).length;
    const aktif = filter.hasil === hasil;
    return {
      label,
      nilai: isLoading ? "–" : jumlah.toLocaleString("id-ID"),
      keterangan: aktif ? "Filter aktif" : cakupan,
      tone: jumlah > 0 ? "defisit" : "netral",
      arti,
      aktif,
      onClick: () =>
        setFilter((f) => ({ ...f, hasil: aktif ? undefined : hasil })),
    };
  };
  const ringkasan: ItemRingkas[] = [
    {
      label: "Dimuat",
      nilai: isLoading
        ? "–"
        : `${dimuat.length.toLocaleString("id-ID")}${hasNextPage ? "+" : ""}`,
      keterangan: hasNextPage ? "Masih ada lagi" : "Semua catatan",
    },
    selHasil(
      "Gagal",
      "GAGAL",
      "Aksi yang gagal dijalankan. Klik untuk menyaring.",
    ),
    selHasil(
      "Ditolak",
      "DITOLAK",
      "Aksi yang ditolak karena izin atau aturan. Klik untuk menyaring.",
    ),
  ];

  return (
    <HalamanPas>
      <PageHeader
        title="Audit trail"
        description="Jejak tetap atas setiap aksi: validasi, publikasi, persetujuan, perubahan ambang, dan akses."
        actions={
          <Button
            type="button"
            variant="outline"
            onClick={() => ekspor.mutate(filter)}
            disabled={ekspor.isPending}
          >
            {ekspor.isPending ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              <Download className="size-4" />
            )}
            Ekspor CSV sesuai filter
          </Button>
        }
      />
      <div className="grid shrink-0 gap-2 lg:grid-cols-[minmax(0,5fr)_minmax(0,6fr)]">
        <AuditIntegritasCard onLihatBaris={setDetailId} />
        <RingkasanKeputusan
          items={ringkasan}
          label="Ringkasan catatan audit"
          className="grid-cols-3"
        />
      </div>
      <AuditDetailSheet
        id={detailId}
        onClose={() => setDetailId(undefined)}
        onFilter={(patch) => setFilter((f) => ({ ...f, ...patch }))}
      />
      <DataTable
        isiPenuh
        filterBar={<AuditFilterBar filter={filter} onChange={setFilter} />}
        toolbar={
          hasNextPage ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => fetchNextPage()}
              disabled={isFetchingNextPage}
            >
              {isFetchingNextPage && (
                <Loader2 className="size-4 animate-spin" />
              )}{" "}
              Muat catatan lebih lama
            </Button>
          ) : undefined
        }
        empty={{
          title: "Tidak ada catatan yang cocok",
          description: "Longgarkan filter atau perluas rentang tanggal.",
        }}
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
    </HalamanPas>
  );
}
