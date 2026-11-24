"use client";

import dynamic from "next/dynamic";
import { memo, useMemo } from "react";
import { MapPin, X } from "lucide-react";
import { RegionMap } from "@/components/map/region-map";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { GeometriWilayah } from "@/lib/api/spasial";
import type { TitikDesa } from "@/lib/api/spasial-jarak";
import { KOMODITAS, kabupatenByKode } from "@/lib/data/reference";
import type { MasterPasarRow, FasilitasColdStorage } from "@/lib/data/types";
import { META_STATUS, formatDelta, type MarkerWilayah, type TitikPeta } from "@/lib/peta/aceh";
import type { TitikTrenHarga } from "./tren-mini-chart";

const TrenMiniChart = dynamic(() => import("./tren-mini-chart"), {
  ssr: false,
  loading: () => <Shimmer className="h-36 w-full rounded-lg" />,
});

export interface PetaSentralPanelProps {
  markers?: MarkerWilayah[];
  geometri?: GeometriWilayah[];
  pasar?: MasterPasarRow[];
  coldFasilitas?: FasilitasColdStorage[];
  titikDesa?: TitikDesa[];
  isLoadingMap: boolean;
  selectedKab?: string;
  onSelectKab: (kode?: string) => void;
  komoditasKode: string;
  onKomoditasChange: (kode: string) => void;
  trenData?: TitikTrenHarga[];
  trenLoading: boolean;
}

export const PetaSentralPanel = memo(function PetaSentralPanel({
  markers,
  geometri,
  pasar,
  coldFasilitas,
  titikDesa,
  isLoadingMap,
  selectedKab,
  onSelectKab,
  komoditasKode,
  onKomoditasChange,
  trenData,
  trenLoading,
}: PetaSentralPanelProps) {
  const titikLain = useMemo<TitikPeta[]>(
    () => [
      ...(pasar ?? [])
        .filter((p) => p.aktif)
        .map((p) => ({
          id: `pasar-${p.id}`,
          jenis: "pasar" as const,
          nama: p.nama,
          keterangan: `Pasar · ${p.kabupaten}`,
          lat: p.lat,
          lng: p.lng,
        })),
      ...(coldFasilitas ?? []).map((f) => ({
        id: `cold-${f.id}`,
        jenis: "cold-storage" as const,
        nama: f.nama,
        keterangan: `Cold storage · ${f.wilayah}`,
        lat: f.lat,
        lng: f.lng,
      })),
      ...(titikDesa ?? []).map((d) => ({
        id: `desa-${d.kode}`,
        jenis: (d.jenis === "PERTANIAN" ? "pertanian" : "perkampungan") as
          | "pertanian"
          | "perkampungan",
        nama: d.nama,
        keterangan:
          d.jenis === "PERTANIAN"
            ? `Sentra Pertanian · ${d.komoditas_utama}`
            : `Perkampungan · ${d.kabupaten_nama}`,
        lat: d.lat,
        lng: d.lng,
      })),
    ],
    [pasar, coldFasilitas, titikDesa],
  );

  const markerAktif = useMemo(
    () => markers?.find((m) => m.kode === selectedKab),
    [markers, selectedKab],
  );
  const kabTren = selectedKab ?? "1171";
  const namaKabTren = kabupatenByKode[kabTren]?.nama ?? "Banda Aceh";
  const komoditasInfo = KOMODITAS.find((k) => k.kode === komoditasKode);

  return (
    <section
      aria-label="Peta sebaran Aceh dan tren harga"
      className="cc-bento flex min-h-0 flex-col gap-2.5"
    >
      <div className="glass relative min-h-[280px] flex-1 overflow-hidden rounded-xl lg:min-h-0">
        <RegionMap
          markers={markers}
          geometri={geometri}
          titikLain={titikLain}
          isLoading={isLoadingMap}
          selected={selectedKab}
          onSelect={(kode) => onSelectKab(selectedKab === kode ? undefined : kode)}
          fokusPilihan
          className="h-full w-full rounded-xl border-0"
        />

        {/* Kartu overlay wilayah terpilih */}
        {markerAktif && (
          <div className="pointer-events-auto absolute right-2.5 bottom-2.5 z-10 flex max-w-xs items-center gap-2.5 rounded-lg border border-border bg-card/95 px-3 py-2 shadow-ambient">
            <MapPin className="size-4 shrink-0 text-data-600" />
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-1.5">
                <p className="truncate text-xs font-semibold text-foreground">
                  {markerAktif.nama}
                </p>
                <StatusBadge
                  tone={
                    markerAktif.status === "defisit"
                      ? "defisit"
                      : markerAktif.status === "waspada"
                        ? "waspada"
                        : markerAktif.status === "surplus"
                          ? "surplus"
                          : "normal"
                  }
                >
                  {META_STATUS[markerAktif.status]?.label ?? markerAktif.status}{" "}
                  {formatDelta(markerAktif.delta)}
                </StatusBadge>
              </div>
              <p className="truncate text-[11px] text-muted-foreground">
                Pemicu: {markerAktif.komoditas || "Stabil"}
              </p>
            </div>
            <button
              type="button"
              onClick={() => onSelectKab(undefined)}
              aria-label="Lepas pilihan wilayah"
              className="grid size-6 shrink-0 place-items-center rounded hover:bg-secondary"
            >
              <X className="size-3.5" />
            </button>
          </div>
        )}
      </div>

      {/* Tren harga 45 hari ringkas */}
      <div className="glass shrink-0 rounded-xl px-3.5 py-2.5">
        <div className="mb-1.5 flex flex-wrap items-center justify-between gap-2">
          <div className="min-w-0">
            <h2 className="truncate text-xs font-semibold uppercase tracking-wide text-foreground">
              Tren Harga 45 Hari · {namaKabTren}
            </h2>
            <p className="truncate text-[11px] text-muted-foreground">
              Perbandingan konsumen vs produsen
            </p>
          </div>
          <Select value={komoditasKode} onValueChange={onKomoditasChange}>
            <SelectTrigger className="h-7 w-44 text-xs" aria-label="Pilih komoditas tren">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {KOMODITAS.map((k) => (
                <SelectItem key={k.kode} value={k.kode}>
                  {k.nama}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <TrenMiniChart
          data={trenData}
          isLoading={trenLoading}
          satuan={komoditasInfo?.satuan ?? "kg"}
        />
      </div>
    </section>
  );
});
