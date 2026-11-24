"use client";

import { MapPin } from "lucide-react";
import { RegionMap } from "@/components/map/region-map";
import { Input } from "@/components/ui/input";
import { useMarkerWilayah } from "@/lib/query/hooks";

/**
 * Pemilih koordinat interaktif: klik titik langsung di peta Aceh atau ketik latitude/longitude secara presisi.
 */
export function KoordinatPicker({
  lat,
  lng,
  onChange,
}: {
  lat: number;
  lng: number;
  onChange: (lat: number, lng: number) => void;
}) {
  const { data: markers } = useMarkerWilayah();
  const valid = Number.isFinite(lat) && Number.isFinite(lng);

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between text-xs text-muted-foreground">
        <span className="inline-flex items-center gap-1 font-medium text-foreground">
          <MapPin className="size-3.5 text-primary" />
          Pilih titik langsung di peta Aceh
        </span>
        <span className="font-mono text-[11px] tabular-nums">
          {valid ? `${lat.toFixed(5)}, ${lng.toFixed(5)}` : "Belum dipilih"}
        </span>
      </div>
      <RegionMap
        markers={markers ?? []}
        titikCari={valid ? { lat, lng } : undefined}
        onKlikPeta={(newLat, newLng) =>
          onChange(
            Math.round(newLat * 100000) / 100000,
            Math.round(newLng * 100000) / 100000,
          )
        }
        fokusPilihan={false}
        className="h-48 w-full rounded-lg border border-border"
      />
      <div className="grid grid-cols-2 gap-2">
        <div>
          <label className="mb-1 block text-[11px] text-muted-foreground">
            Latitude (Lintang)
          </label>
          <Input
            type="number"
            step="0.0001"
            value={Number.isFinite(lat) ? lat : ""}
            onChange={(e) => {
              const v = Number(e.target.value);
              if (Number.isFinite(v)) onChange(v, lng);
            }}
          />
        </div>
        <div>
          <label className="mb-1 block text-[11px] text-muted-foreground">
            Longitude (Bujur)
          </label>
          <Input
            type="number"
            step="0.0001"
            value={Number.isFinite(lng) ? lng : ""}
            onChange={(e) => {
              const v = Number(e.target.value);
              if (Number.isFinite(v)) onChange(lat, v);
            }}
          />
        </div>
      </div>
    </div>
  );
}
