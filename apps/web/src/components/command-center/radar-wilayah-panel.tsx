"use client";

import { memo, useMemo, useState } from "react";
import { CloudRain, MapPin, Search, X } from "lucide-react";
import type { CuacaWilayah, Heatmap } from "@/lib/api/dashboard";
import { StatusBadge } from "@/components/garda/status-badge";
import { META_STATUS, formatDelta, type MarkerWilayah } from "@/lib/peta/aceh";
import { cn } from "@/lib/utils";

export interface RadarWilayahPanelProps {
  markers?: MarkerWilayah[];
  heatmap?: Heatmap;
  cuaca?: CuacaWilayah[];
  selectedKab?: string;
  onSelectKab: (kode?: string) => void;
}

export const RadarWilayahPanel = memo(function RadarWilayahPanel({
  markers,
  heatmap,
  cuaca,
  selectedKab,
  onSelectKab,
}: RadarWilayahPanelProps) {
  const [cari, setCari] = useState("");

  const wilayahUrut = useMemo(() => {
    const q = cari.trim().toLowerCase();
    return [...(markers ?? [])]
      .filter((m) => !q || m.nama.toLowerCase().includes(q) || m.komoditas.toLowerCase().includes(q))
      .sort(
        (a, b) =>
          META_STATUS[a.status].urutan - META_STATUS[b.status].urutan ||
          Math.abs(b.delta) - Math.abs(a.delta),
      );
  }, [markers, cari]);

  const topPenggerak = useMemo(() => {
    const sel = heatmap?.sel ?? [];
    const disaring = selectedKab
      ? sel.filter((s) => s.kabupatenKode === selectedKab)
      : sel;
    return [...disaring]
      .sort((a, b) => Math.abs(b.delta) - Math.abs(a.delta))
      .slice(0, 4)
      .map((s) => {
        const komNama =
          heatmap?.komoditas.find((k) => k.kode === s.komoditasKode)?.nama ??
          s.komoditasKode;
        const kabNama =
          heatmap?.kabupaten.find((k) => k.kode === s.kabupatenKode)?.nama ??
          s.kabupatenKode;
        return { ...s, komNama, kabNama };
      });
  }, [heatmap, selectedKab]);

  const anomaliCuaca = useMemo(
    () => (cuaca ?? []).filter((c) => c.anomali).slice(0, 3),
    [cuaca],
  );

  return (
    <aside
      aria-label="Radar wilayah dan penggerak harga"
      className="cc-bento glass flex min-h-0 flex-col overflow-hidden rounded-xl"
    >
      <div className="flex shrink-0 items-center justify-between gap-2 border-b border-border px-3.5 py-2.5">
        <div className="min-w-0">
          <h2 className="truncate text-xs font-semibold uppercase tracking-wide text-foreground">
            Radar Wilayah Aceh
          </h2>
          <p className="truncate text-[11px] text-muted-foreground">
            Klik kabupaten/kota untuk mengunci peta &amp; tren
          </p>
        </div>
        {selectedKab && (
          <button
            type="button"
            onClick={() => onSelectKab(undefined)}
            className="inline-flex h-6 shrink-0 items-center gap-1 rounded-md border border-border bg-card px-2 text-[11px] font-medium hover:bg-secondary"
          >
            <X className="size-3" /> Reset
          </button>
        )}
      </div>

      <div className="shrink-0 border-b border-border px-3 py-2">
        <label className="relative block">
          <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <input
            type="search"
            value={cari}
            onChange={(e) => setCari(e.target.value)}
            placeholder="Filter wilayah atau komoditas…"
            className="h-7 w-full rounded-md border border-border bg-background pr-2.5 pl-7 text-xs text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring/40"
          />
        </label>
      </div>

      <div
        data-lenis-prevent
        className="scrollbar-thin min-h-0 flex-1 divide-y divide-border overflow-y-auto"
      >
        {wilayahUrut.length === 0 ? (
          <p className="p-4 text-center text-xs text-muted-foreground">
            Belum ada data wilayah terpantau.
          </p>
        ) : (
          wilayahUrut.map((m) => {
            const aktif = selectedKab === m.kode;
            const meta = META_STATUS[m.status] ?? META_STATUS.normal;
            return (
              <button
                key={m.kode}
                type="button"
                onClick={() => onSelectKab(aktif ? undefined : m.kode)}
                aria-pressed={aktif}
                className={cn(
                  "flex w-full items-center justify-between gap-2 px-3.5 py-2 text-left transition-colors hover:bg-secondary/60",
                  aktif && "bg-secondary",
                )}
              >
                <div className="min-w-0">
                  <div className="flex items-center gap-1.5">
                    <span
                      aria-hidden
                      className="size-2 shrink-0 rounded-full"
                      style={{ backgroundColor: meta.warna }}
                    />
                    <span className="truncate text-xs font-medium text-foreground">
                      {m.nama}
                    </span>
                  </div>
                  <p className="truncate pl-3.5 text-[11px] text-muted-foreground">
                    {m.komoditas || "Stabil"}
                  </p>
                </div>
                <span
                  className={cn(
                    "num shrink-0 rounded px-1.5 py-0.5 text-[11px] font-semibold",
                    m.status === "defisit"
                      ? "bg-rose-500/12 text-rose-600 dark:text-rose-300"
                      : m.status === "waspada"
                        ? "bg-amber-500/15 text-amber-700 dark:text-amber-300"
                        : m.status === "surplus"
                          ? "bg-sky-500/12 text-sky-700 dark:text-sky-300"
                          : "bg-emerald-500/12 text-emerald-700 dark:text-emerald-300",
                  )}
                >
                  {formatDelta(m.delta)}
                </span>
              </button>
            );
          })
        )}
      </div>

      {/* Ringkasan penggerak harga & cuaca anomali */}
      <div className="shrink-0 space-y-2 border-t border-border bg-muted/25 px-3.5 py-2.5">
        {topPenggerak.length > 0 && (
          <div>
            <p className="mb-1 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
              Penggerak Terkuat
            </p>
            <div className="grid grid-cols-2 gap-1.5">
              {topPenggerak.map((p) => (
                <div
                  key={`${p.kabupatenKode}:${p.komoditasKode}`}
                  className="rounded-md border border-border bg-card/80 px-2 py-1 text-[11px]"
                >
                  <p className="truncate font-medium text-foreground">
                    {p.komNama}
                  </p>
                  <div className="flex items-center justify-between gap-1 text-[10px] text-muted-foreground">
                    <span className="truncate">{p.kabNama}</span>
                    <span
                      className={cn(
                        "num font-semibold",
                        p.delta > 5 ? "text-rose-600 dark:text-rose-300" : "text-emerald-600",
                      )}
                    >
                      {formatDelta(p.delta)}
                    </span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {anomaliCuaca.length > 0 && (
          <div className="flex flex-wrap items-center gap-1.5 text-[11px]">
            <CloudRain className="size-3.5 shrink-0 text-sky-500" />
            <span className="font-medium text-foreground">Anomali Hujan:</span>
            {anomaliCuaca.map((c) => (
              <StatusBadge key={c.kode} tone="waspada">
                <MapPin className="size-2.5" />
                {c.nama} ({c.curahHujanMm.toFixed(0)}mm)
              </StatusBadge>
            ))}
          </div>
        )}
      </div>
    </aside>
  );
});
