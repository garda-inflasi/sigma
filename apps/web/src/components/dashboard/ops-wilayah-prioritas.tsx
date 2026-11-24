"use client";

import Link from "next/link";
import { useMemo } from "react";
import { EmptyState } from "@/components/garda/empty-state";
import { PanelPas } from "@/components/garda/halaman-pas";
import { PanelGalat } from "@/components/garda/panel-galat";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import { formatPersen } from "@/lib/format";
import { META_STATUS } from "@/lib/peta/aceh";
import { useMarkerWilayah } from "@/lib/query/hooks";

/** Wilayah berstatus defisit atau waspada, yang paling tertekan di atas; memakai data penanda yang sama dengan peta. */
export function OpsWilayahPrioritas({ className }: { className?: string }) {
  const { data, isLoading, error, refetch } = useMarkerWilayah();
  const urut = useMemo(
    () =>
      (data ?? [])
        .filter(
          (m) =>
            m.punyaPasar && (m.status === "defisit" || m.status === "waspada"),
        )
        .sort(
          (a, b) =>
            META_STATUS[a.status].urutan - META_STATUS[b.status].urutan ||
            b.delta - a.delta,
        ),
    [data],
  );

  return (
    <PanelPas
      judul="Wilayah prioritas"
      deskripsi="Urut dari yang paling tertekan"
      aksi={
        urut.length > 0 && (
          <StatusBadge tone="defisit" dot={false} className="num">
            {urut.length}
          </StatusBadge>
        )
      }
      tanpaPadding
      className={className}
      isiClassName="max-h-[50dvh] overflow-y-auto lg:max-h-none"
    >
      {isLoading ? (
        <div className="space-y-2 p-3.5" aria-busy="true">
          {Array.from({ length: 6 }).map((_, i) => (
            <Shimmer key={i} className="h-10 w-full" />
          ))}
        </div>
      ) : !data ? (
        <PanelGalat
          judul="Wilayah prioritas gagal dimuat"
          error={error}
          onCoba={() => void refetch()}
        />
      ) : urut.length === 0 ? (
        <EmptyState
          title="Tidak ada wilayah tertekan"
          description="Semua wilayah berpasar berstatus normal atau surplus."
          className="py-10"
        />
      ) : (
        <ul className="divide-y divide-border">
          {urut.map((m) => (
            <li key={m.kode}>
              <Link
                href={`/peta?wilayah=${m.kode}`}
                className="flex items-center gap-2.5 px-3.5 py-2 transition-colors hover:bg-secondary/60"
              >
                <span
                  aria-hidden
                  className="size-2 shrink-0 rounded-full"
                  style={{ backgroundColor: META_STATUS[m.status].warna }}
                />
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-[13px] font-medium">
                    {m.nama}
                  </span>
                  <span className="block truncate text-xs text-muted-foreground">
                    {META_STATUS[m.status].label} · {m.komoditas}
                  </span>
                </span>
                <span className="num text-[13px] font-semibold">
                  {formatPersen(m.delta, { signed: true })}
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </PanelPas>
  );
}
