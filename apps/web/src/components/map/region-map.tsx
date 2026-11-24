"use client";

import dynamic from "next/dynamic";
import { useMemo, useState } from "react";
import { Shimmer } from "@/components/garda/shimmer";
import type { GeometriWilayah } from "@/lib/api/spasial";
import type { GarisUkurPeta } from "./region-map-leaflet";
import {
  META_STATUS,
  formatDelta,
  type MarkerWilayah,
  type StatusWilayah,
  type TitikPeta,
} from "@/lib/peta/aceh";
import { cn } from "@/lib/utils";

const PetaLeaflet = dynamic(() => import("./region-map-leaflet"), {
  ssr: false,
  loading: () => <Shimmer className="size-full rounded-none" />,
});

const URUTAN_STATUS: StatusWilayah[] = [
  "defisit",
  "waspada",
  "normal",
  "surplus",
];

/**
 * Peta interaktif Provinsi Aceh: poligon status per kabupaten/kota, pin ibu kota, serta pasar dan cold storage
 * opsional. `onKlikPeta` menerima koordinat `(lat, lng)` titik yang diklik.
 */
export function RegionMap({
  markers,
  geometri,
  titikLain,
  titikCari,
  garisUkur,
  isLoading,
  selected,
  onSelect,
  onKlikPeta,
  onKlikTitik,
  aksiPopup,
  fokusPilihan,
  className,
}: {
  markers?: MarkerWilayah[];
  geometri?: GeometriWilayah[];
  titikLain?: TitikPeta[];
  titikCari?: { lat: number; lng: number };
  garisUkur?: GarisUkurPeta | null;
  isLoading?: boolean;
  selected?: string;
  onSelect?: (kode: string) => void;
  onKlikPeta?: (lat: number, lng: number) => void;
  onKlikTitik?: (titik: TitikPeta) => void;
  aksiPopup?: { label: string; onClick: (kode: string) => void };
  fokusPilihan?: boolean;
  className?: string;
}) {
  const [tampilPasar, setTampilPasar] = useState(true);
  const [tampilCold, setTampilCold] = useState(true);
  const [tampilPertanian, setTampilPertanian] = useState(true);
  const [tampilPerkampungan, setTampilPerkampungan] = useState(true);
  const jumlahPasar = titikLain?.filter((t) => t.jenis === "pasar").length ?? 0;
  const jumlahCold =
    titikLain?.filter((t) => t.jenis === "cold-storage").length ?? 0;
  const jumlahPertanian =
    titikLain?.filter((t) => t.jenis === "pertanian").length ?? 0;
  const jumlahPerkampungan =
    titikLain?.filter((t) => t.jenis === "perkampungan").length ?? 0;
  const titikTampil = useMemo(
    () =>
      titikLain?.filter((t) => {
        if (t.jenis === "pasar") return tampilPasar;
        if (t.jenis === "cold-storage") return tampilCold;
        if (t.jenis === "pertanian") return tampilPertanian;
        return tampilPerkampungan;
      }),
    [titikLain, tampilPasar, tampilCold, tampilPertanian, tampilPerkampungan],
  );

  return (
    <div
      role="region"
      aria-label="Peta Provinsi Aceh"
      data-lenis-prevent
      className={cn(
        "relative isolate overflow-hidden rounded-lg border border-border bg-secondary",
        onKlikPeta && "[&_.leaflet-container]:cursor-crosshair",
        className,
      )}
    >
      {isLoading || !markers ? (
        <Shimmer className="size-full rounded-none" />
      ) : (
        <div className="absolute inset-0 z-0">
          <PetaLeaflet
            markers={markers}
            geometri={geometri}
            titikLain={titikTampil}
            titikCari={titikCari}
            garisUkur={garisUkur}
            selected={selected}
            onSelect={onSelect}
            onKlikPeta={onKlikPeta}
            onKlikTitik={onKlikTitik}
            aksiPopup={aksiPopup}
            fokusPilihan={fokusPilihan}
          />
        </div>
      )}

      <div className="pointer-events-none absolute top-2 left-2 z-10 flex max-w-[calc(100%-3.75rem)] flex-col items-start gap-1.5">
        <ul
          aria-label="Legenda status wilayah"
          className="pointer-events-auto flex flex-wrap gap-x-2.5 gap-y-1 rounded-md border border-border bg-card/95 px-2 py-1 text-[11px] text-muted-foreground shadow-ambient-xs"
        >
          {URUTAN_STATUS.map((s) => (
            <li
              key={s}
              title={META_STATUS[s].arti}
              className="inline-flex cursor-help items-center gap-1"
            >
              <span
                aria-hidden
                className="size-2 rounded-full"
                style={{ backgroundColor: META_STATUS[s].warna }}
              />
              {META_STATUS[s].label}
            </li>
          ))}
        </ul>
        {(jumlahPasar > 0 ||
          jumlahCold > 0 ||
          jumlahPertanian > 0 ||
          jumlahPerkampungan > 0) && (
          <div
            className="pointer-events-auto flex flex-wrap gap-1"
            role="group"
            aria-label="Lapisan peta"
          >
            {jumlahPasar > 0 && (
              <TombolLapisan
                aktif={tampilPasar}
                onClick={() => setTampilPasar((v) => !v)}
                kelasPin="garda-pin-pasar"
              >
                Pasar <span className="num">{jumlahPasar}</span>
              </TombolLapisan>
            )}
            {jumlahCold > 0 && (
              <TombolLapisan
                aktif={tampilCold}
                onClick={() => setTampilCold((v) => !v)}
                kelasPin="garda-pin-cold"
              >
                Cold storage <span className="num">{jumlahCold}</span>
              </TombolLapisan>
            )}
            {jumlahPertanian > 0 && (
              <TombolLapisan
                aktif={tampilPertanian}
                onClick={() => setTampilPertanian((v) => !v)}
                kelasPin="garda-pin-pertanian"
              >
                Pertanian <span className="num">{jumlahPertanian}</span>
              </TombolLapisan>
            )}
            {jumlahPerkampungan > 0 && (
              <TombolLapisan
                aktif={tampilPerkampungan}
                onClick={() => setTampilPerkampungan((v) => !v)}
                kelasPin="garda-pin-perkampungan"
              >
                Perkampungan <span className="num">{jumlahPerkampungan}</span>
              </TombolLapisan>
            )}
          </div>
        )}
      </div>

      {/* Jalur keyboard dan pembaca layar ke tiap wilayah; muncul saat mendapat fokus. */}
      {markers && (
        <ul aria-label="Daftar wilayah di peta">
          {markers.map((m) => (
            <li key={m.kode}>
              <button
                type="button"
                aria-label={`${m.nama}: ${m.komoditas}`}
                onClick={() => onSelect?.(m.kode)}
                className="sr-only focus:not-sr-only focus:absolute focus:bottom-8 focus:left-2 focus:z-20 focus:rounded-md focus:border focus:border-border focus:bg-card focus:px-2.5 focus:py-1.5 focus:text-xs focus:font-medium focus:shadow-ambient"
              >
                {m.nama} · {META_STATUS[m.status]?.label ?? m.status} ·{" "}
                {formatDelta(m.delta)}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function TombolLapisan({
  aktif,
  onClick,
  kelasPin,
  children,
}: {
  aktif: boolean;
  onClick: () => void;
  kelasPin: string;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      aria-pressed={aktif}
      onClick={onClick}
      className={cn(
        "inline-flex h-6 items-center gap-1.5 rounded-md border border-border bg-card/95 px-2 text-[11px] font-medium shadow-ambient-xs transition-colors hover:bg-secondary",
        aktif ? "text-foreground" : "text-muted-foreground opacity-70",
      )}
    >
      <span
        aria-hidden
        className={cn(kelasPin, "scale-75", !aktif && "grayscale")}
      />
      {children}
    </button>
  );
}
