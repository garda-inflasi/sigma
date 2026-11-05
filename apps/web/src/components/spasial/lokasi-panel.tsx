"use client";

import { Crosshair, Loader2, MapPin } from "lucide-react";
import { useState } from "react";
import {
  keteranganLokasi,
  pesanGalatGeolokasi,
  type LokasiWilayah,
} from "@/lib/api/spasial";

export interface TitikLokasi {
  lat: number;
  lng: number;
}

/**
 * "Lokasi saya": membaca posisi dari geolokasi peramban lalu menanyakan kabupaten/kota-nya
 * ke API. Hasil pencarian (juga dari klik peta) ditampilkan di sini; pemilihan wilayah di
 * peta dilakukan pemanggil lewat `onCari`.
 */
export function LokasiPanel({
  titik,
  hasil,
  galat,
  mencari,
  onCari,
}: {
  /** Titik terakhir yang dicari (dari geolokasi atau klik peta). */
  titik?: TitikLokasi;
  hasil?: LokasiWilayah;
  galat?: string;
  mencari: boolean;
  onCari: (t: TitikLokasi) => void;
}) {
  const [membaca, setMembaca] = useState(false);
  // Galat geolokasi hanya berlaku sampai ada hasil pencarian baru (mis. dari klik peta).
  const [galatGeo, setGalatGeo] = useState<{ pesan: string; saatHasil?: LokasiWilayah }>();

  const cariPosisiSaya = () => {
    setGalatGeo(undefined);
    if (typeof navigator === "undefined" || !("geolocation" in navigator)) {
      setGalatGeo({ pesan: "Peramban ini tidak mendukung geolokasi. Klik peta untuk memilih titik.", saatHasil: hasil });
      return;
    }
    setMembaca(true);
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        setMembaca(false);
        onCari({ lat: pos.coords.latitude, lng: pos.coords.longitude });
      },
      (err) => {
        setMembaca(false);
        setGalatGeo({ pesan: pesanGalatGeolokasi(err.code), saatHasil: hasil });
      },
      { enableHighAccuracy: false, timeout: 10_000, maximumAge: 60_000 },
    );
  };

  const sibuk = membaca || mencari;
  const pesanGalat = (galatGeo && galatGeo.saatHasil === hasil ? galatGeo.pesan : undefined) ?? galat;

  return (
    <div
      className="mb-3 flex flex-wrap items-center gap-x-4 gap-y-2"
      aria-label="Lokasi saya"
    >
      <button
        type="button"
        onClick={cariPosisiSaya}
        disabled={sibuk}
        className="inline-flex h-10 items-center gap-2 rounded-xl border border-input bg-white/80 px-4 text-sm font-medium hover:bg-white disabled:opacity-50 dark:bg-white/[0.06]"
      >
        {sibuk ? (
          <Loader2 className="size-4 animate-spin" />
        ) : (
          <Crosshair className="size-4" />
        )}{" "}
        Lokasi saya
      </button>
      <div className="min-w-0 flex-1 text-sm" role="status" aria-live="polite">
        {pesanGalat ? (
          <p className="text-defisit-ink dark:text-rose-300">{pesanGalat}</p>
        ) : hasil ? (
          <p className="flex flex-wrap items-center gap-x-2">
            <MapPin
              className="size-4 shrink-0 text-muted-foreground"
              aria-hidden
            />
            <span className="font-semibold">{hasil.nama}</span>
            <span className="text-muted-foreground">
              {keteranganLokasi(hasil)}
            </span>
            {titik && (
              <span className="num text-xs text-muted-foreground">
                ({titik.lat.toFixed(4)}, {titik.lng.toFixed(4)})
              </span>
            )}
          </p>
        ) : (
          <p className="text-muted-foreground">
            Temukan kabupaten/kota Anda lewat geolokasi, atau klik titik di
            peta.
          </p>
        )}
      </div>
    </div>
  );
}
