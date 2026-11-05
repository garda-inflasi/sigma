"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { useMemo, useState } from "react";
import { DataTable } from "@/components/data-table/data-table";
import { PanelTitle } from "@/components/garda/page-header";
import { StatusBadge } from "@/components/garda/status-badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ringkasNeraca, type NeracaWilayah } from "@/lib/api/spasial";
import { KOMODITAS } from "@/lib/data/reference";
import { formatAngka, formatPersen, formatTanggal } from "@/lib/format";
import { useNeracaWilayah } from "@/lib/query/hooks";
import { cn } from "@/lib/utils";

const SEMUA = "__semua";

const STATUS = {
  surplus: {
    tone: "surplus",
    label: "Surplus",
    teks: "text-surplus-ink dark:text-blue-300",
  },
  defisit: {
    tone: "defisit",
    label: "Defisit",
    teks: "text-defisit-ink dark:text-rose-300",
  },
  seimbang: {
    tone: "netral",
    label: "Seimbang",
    teks: "text-muted-foreground",
  },
} as const;

/**
 * Neraca pasokan per wilayah (periode terbaru): ton positif = surplus, negatif = defisit.
 * Klik baris memilih wilayah di peta.
 */
export function NeracaPanel({
  pilih,
  onPilih,
}: {
  pilih?: string;
  onPilih: (kode: string) => void;
}) {
  const [komoditas, setKomoditas] = useState("CBM-KRT");
  const { data, isLoading, isError, error } = useNeracaWilayah(
    komoditas === SEMUA ? "" : komoditas,
  );

  const ringkas = useMemo(() => ringkasNeraca(data ?? []), [data]);
  const periode = data?.[0]?.periode;
  const demo = data?.some((n) => n.demo) ?? false;

  const kolom = useMemo<ColumnDef<NeracaWilayah>[]>(
    () => [
      {
        accessorKey: "wilayah",
        header: "Wilayah",
        cell: ({ getValue }) => (
          <span className="whitespace-nowrap font-medium">
            {getValue<string>()}
          </span>
        ),
      },
      { accessorKey: "komoditas", header: "Komoditas" },
      {
        accessorKey: "surplusDefisitTon",
        header: "Surplus/defisit",
        cell: ({ row: { original: n } }) => {
          const s = STATUS[n.status];
          return (
            <span className="flex items-center gap-2 whitespace-nowrap">
              <span className={cn("num font-semibold", s.teks)}>
                {n.surplusDefisitTon > 0
                  ? "+"
                  : n.surplusDefisitTon < 0
                    ? "−"
                    : ""}
                {formatAngka(Math.abs(n.surplusDefisitTon), 1)} ton
              </span>
              <StatusBadge tone={s.tone} className="h-5 text-[10px]">
                {s.label}
              </StatusBadge>
            </span>
          );
        },
      },
      {
        accessorKey: "deltaHargaPct",
        header: "Δ harga",
        cell: ({ getValue }) => (
          <span className="num">
            {formatPersen(getValue<number>(), { signed: true })}
          </span>
        ),
      },
      {
        accessorKey: "sumber",
        header: "Sumber",
        cell: ({ row: { original: n } }) =>
          n.demo ? (
            <StatusBadge tone="waspada" dot={false} className="h-5 text-[10px]">
              Data demo
            </StatusBadge>
          ) : (
            <span className="text-xs text-muted-foreground">{n.sumber}</span>
          ),
      },
    ],
    [],
  );

  return (
    <section
      className="glass space-y-3 rounded-xl p-5"
      aria-label="Neraca pasokan"
    >
      <PanelTitle
        title="Neraca pasokan"
        description={
          <>
            Surplus (+) dan defisit (−) pasokan per wilayah dalam ton
            {periode ? `, periode ${formatTanggal(periode, "MMMM yyyy")}` : ""}
            {demo ? "; data demo" : ""}.
          </>
        }
      />
      {isError ? (
        <p role="alert" className="text-sm text-defisit-ink dark:text-rose-300">
          Gagal memuat neraca pasokan
          {error instanceof Error ? `: ${error.message}` : "."}
        </p>
      ) : (
        <DataTable
          data={data}
          isLoading={isLoading}
          columns={kolom}
          getRowId={(n) => n.id}
          entityLabel="wilayah"
          pageSize={10}
          searchPlaceholder="Cari wilayah"
          initialSorting={[{ id: "surplusDefisitTon", desc: false }]}
          onRowClick={(n) => onPilih(n.kabupatenKode)}
          rowTone={(n) => (n.kabupatenKode === pilih ? "data" : undefined)}
          empty={{
            title: "Belum ada neraca",
            description:
              "Belum ada data neraca pasokan untuk komoditas ini pada periode terbaru.",
          }}
          toolbar={
            <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
              <Select value={komoditas} onValueChange={setKomoditas}>
                <SelectTrigger
                  aria-label="Komoditas neraca"
                  className="h-10 w-52"
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={SEMUA}>Semua komoditas</SelectItem>
                  {KOMODITAS.map((k) => (
                    <SelectItem key={k.kode} value={k.kode}>
                      {k.nama}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {data && data.length > 0 && (
                <p className="text-xs text-muted-foreground" aria-live="polite">
                  <span className="num font-semibold text-surplus-ink dark:text-blue-300">
                    Surplus {formatAngka(ringkas.surplusTon, 1)} ton
                  </span>{" "}
                  di {ringkas.jumlahSurplus} wilayah ·{" "}
                  <span className="num font-semibold text-defisit-ink dark:text-rose-300">
                    Defisit {formatAngka(ringkas.defisitTon, 1)} ton
                  </span>{" "}
                  di {ringkas.jumlahDefisit} wilayah
                </p>
              )}
            </div>
          }
        />
      )}
    </section>
  );
}
