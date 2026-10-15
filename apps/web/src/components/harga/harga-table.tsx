"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Camera, CheckCheck, Download, MapPinOff, RotateCcw, Send, XCircle } from "lucide-react";
import { useMemo } from "react";
import type { BulkAction } from "@/components/data-table/bulk-action-bar";
import { KomoditasIcon } from "@/components/garda/komoditas-icon";
import { STATUS_ENTRI, StatusBadge } from "@/components/garda/status-badge";
import { useSession } from "@/lib/auth/session-context";
import { exportCsv } from "@/lib/csv";
import type { EntriHarga, StatusEntri } from "@/lib/data/types";
import { formatPersen, formatRelatif, formatRupiah } from "@/lib/format";
import { useBulkAction } from "@/lib/query/hooks";
import { qk } from "@/lib/query/keys";
import { cn } from "@/lib/utils";

export function useHargaColumns(opts: { ringkas?: boolean } = {}) {
  return useMemo<ColumnDef<EntriHarga>[]>(() => {
    const cols: ColumnDef<EntriHarga>[] = [
      {
        id: "komoditas",
        accessorKey: "komoditasNama",
        header: "Komoditas",
        cell: ({ row: { original: r } }) => (
          <div className="flex items-center gap-2.5">
            <span className="grid size-8 place-items-center rounded-xl bg-navy-50 text-navy-700 dark:bg-white/10 dark:text-navy-200"><KomoditasIcon kode={r.komoditasKode} /></span>
            <div className="min-w-0">
              <p className="truncate font-medium">{r.komoditasNama}</p>
              <p className="text-xs text-muted-foreground">{r.tipe === "PRODUSEN" ? "Harga produsen" : "Harga konsumen"}, {r.kualitas.toLowerCase()}</p>
            </div>
          </div>
        ),
      },
      {
        id: "pasar",
        accessorFn: (r) => `${r.pasarNama} ${r.wilayah}`,
        header: "Pasar",
        cell: ({ row: { original: r } }) => (
          <div className="min-w-0">
            <p className="truncate">{r.pasarNama}</p>
            <p className="truncate text-xs text-muted-foreground">{r.wilayah}</p>
          </div>
        ),
      },
      {
        accessorKey: "harga",
        header: "Harga",
        cell: ({ row: { original: r } }) => (
          <span className="num font-semibold whitespace-nowrap">{formatRupiah(r.harga)}<span className="font-normal text-muted-foreground">/{r.satuan}</span></span>
        ),
      },
      {
        accessorKey: "deviasiPct",
        header: "Deviasi",
        cell: ({ row: { original: r } }) => {
          const a = Math.abs(r.deviasiPct);
          return (
            <span className={cn("num inline-flex h-6 items-center rounded-full px-2 text-xs font-medium", a > 25 ? "bg-defisit-soft text-defisit-ink" : a > 10 ? "bg-waspada-soft text-waspada-ink" : "text-muted-foreground")} title="Terhadap median 7 hari pasar yang sama">
              {formatPersen(r.deviasiPct, { signed: true })}
            </span>
          );
        },
      },
      {
        id: "bukti",
        header: "Bukti",
        enableSorting: false,
        cell: ({ row: { original: r } }) => (
          <span className="inline-flex items-center gap-1.5 text-muted-foreground">
            <Camera className={cn("size-4", r.adaFoto ? "text-data-600" : "opacity-25")} aria-label={r.adaFoto ? "Ada foto" : "Tanpa foto"} />
            {!r.dalamGeofence && <MapPinOff className="size-4 text-waspada" aria-label="Di luar geofence" />}
          </span>
        ),
      },
      { accessorKey: "petugasNama", header: "Petugas", cell: ({ getValue }) => <span className="whitespace-nowrap">{getValue<string>()}</span> },
      { accessorKey: "dicatatPada", header: "Dicatat", cell: ({ getValue }) => <span className="whitespace-nowrap text-muted-foreground">{formatRelatif(getValue<string>())}</span> },
      {
        accessorKey: "status",
        header: "Status",
        filterFn: "equals",
        cell: ({ getValue }) => {
          const s = STATUS_ENTRI[getValue<StatusEntri>()];
          return <StatusBadge tone={s.tone}>{s.label}</StatusBadge>;
        },
      },
    ];
    return opts.ringkas ? cols.filter((c) => !["bukti", "petugasNama"].includes((c.id ?? (c as { accessorKey?: string }).accessorKey) as string)) : cols;
  }, [opts.ringkas]);
}

/** Aksi massal entri harga — mengikuti pemisahan tugas (SoD). */
export function useHargaBulkActions(): BulkAction<EntriHarga>[] {
  const user = useSession();
  const mut = useBulkAction<EntriHarga>({
    sumber: "harga",
    queryKey: qk.harga.list(),
    patch: (r, aksi) => ({ ...r, status: ({ validasi: "DIVALIDASI", tolak: "DITOLAK", publikasi: "DIPUBLIKASI", kembalikan: "DRAF" } as Record<string, StatusEntri>)[aksi] ?? r.status }),
  });
  const ids = (rows: EntriHarga[]) => rows.map((r) => r.id);
  return [
    {
      id: "validasi", label: "Validasi", icon: CheckCheck, perm: "harga.validasi", tone: "data",
      enabled: (rows) => rows.every((r) => r.status === "DIAJUKAN" && r.petugasId !== user.id),
      run: (rows) => mut.mutateAsync({ aksi: "validasi", ids: ids(rows), label: "Entri divalidasi" }),
    },
    {
      id: "publikasi", label: "Publikasikan", icon: Send, perm: "harga.publikasi",
      enabled: (rows) => rows.every((r) => r.status === "DIVALIDASI"),
      run: (rows) => mut.mutateAsync({ aksi: "publikasi", ids: ids(rows), label: "Entri dipublikasikan" }),
    },
    {
      id: "tolak", label: "Tolak", icon: XCircle, perm: "harga.validasi", tone: "danger",
      enabled: (rows) => rows.every((r) => r.status === "DIAJUKAN"),
      confirm: { title: "Tolak entri terpilih?", description: (n) => `${n} entri akan dikembalikan ke petugas dengan status ditolak. Petugas menerima notifikasi untuk survei ulang.`, actionLabel: "Tolak entri" },
      run: (rows) => mut.mutateAsync({ aksi: "tolak", ids: ids(rows), label: "Entri ditolak" }),
    },
    {
      id: "kembalikan", label: "Minta survei ulang", icon: RotateCcw, perm: "harga.validasi",
      run: (rows) => mut.mutateAsync({ aksi: "kembalikan", ids: ids(rows), label: "Survei ulang diminta" }),
    },
    {
      id: "ekspor", label: "Ekspor CSV", icon: Download, perm: "harga.ekspor",
      run: (rows) =>
        exportCsv(rows, [
          { header: "ID", value: (r) => r.id }, { header: "Komoditas", value: (r) => r.komoditasNama }, { header: "Tipe", value: (r) => r.tipe },
          { header: "Pasar", value: (r) => r.pasarNama }, { header: "Wilayah", value: (r) => r.wilayah }, { header: "Harga", value: (r) => r.harga },
          { header: "Satuan", value: (r) => r.satuan }, { header: "Deviasi (%)", value: (r) => r.deviasiPct }, { header: "Petugas", value: (r) => r.petugasNama },
          { header: "Dicatat", value: (r) => r.dicatatPada }, { header: "Status", value: (r) => r.status },
        ], "entri-harga"),
    },
  ];
}

export const FACET_STATUS_ENTRI = {
  columnId: "status",
  label: "Status",
  options: Object.entries(STATUS_ENTRI).map(([value, v]) => ({ value, label: v.label })),
};
