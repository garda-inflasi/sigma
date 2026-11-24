"use client";

import { ArrowRightLeft, Navigation, RotateCcw } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { StatusBadge } from "@/components/garda/status-badge";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { GarisUkurPeta } from "@/components/map/region-map-leaflet";
import {
  estimasiJalanKm,
  estimasiMenitTempuh,
  haversineKm,
  risikoFoodLossDariMenit,
  type TitikPeta,
} from "@/lib/peta/aceh";

const JENIS_LABEL: Record<TitikPeta["jenis"], string> = {
  pertanian: "Sentra Pertanian",
  perkampungan: "Perkampungan",
  pasar: "Pasar",
  "cold-storage": "Cold Storage",
};

/**
 * Kalkulator jarak spasial antar titik (Sentra Pertanian / Perkampungan <-> Pasar / Cold Storage)
 * yang menggambar garis ukur di peta dan menampilkan estimasi km jalan, waktu tempuh, & risiko food loss.
 */
export function KalkulatorJarakPanel({
  titikLain,
  garisUkur,
  onChangeGaris,
}: {
  titikLain: TitikPeta[];
  garisUkur: GarisUkurPeta | null;
  onChangeGaris: (g: GarisUkurPeta | null) => void;
}) {
  const [asalId, setAsalId] = useState<string>("");
  const [tujuanId, setTujuanId] = useState<string>("");

  const sumberAsal = useMemo(
    () =>
      titikLain.filter(
        (t) => t.jenis === "pertanian" || t.jenis === "perkampungan",
      ),
    [titikLain],
  );
  const pilihanAsal = sumberAsal.length > 0 ? sumberAsal : titikLain;
  const asal = titikLain.find((t) => t.id === asalId);
  const tujuan = titikLain.find((t) => t.id === tujuanId);

  // Hitung fasilitas pasar & cold storage terdekat dari titik asal
  const fasilitasTerdekat = useMemo(() => {
    if (!asal) return { pasar: [], cold: [] };
    const ukur = (t: TitikPeta) => {
      const lurus = haversineKm(asal.lat, asal.lng, t.lat, t.lng);
      const jalan = estimasiJalanKm(lurus);
      return {
        ...t,
        lurusKm: Math.round(lurus * 10) / 10,
        jalanKm: jalan,
        menit: estimasiMenitTempuh(jalan),
      };
    };
    const pasar = titikLain
      .filter((t) => t.jenis === "pasar")
      .map(ukur)
      .sort((a, b) => a.lurusKm - b.lurusKm)
      .slice(0, 3);
    const cold = titikLain
      .filter((t) => t.jenis === "cold-storage")
      .map(ukur)
      .sort((a, b) => a.lurusKm - b.lurusKm)
      .slice(0, 3);
    return { pasar, cold };
  }, [asal, titikLain]);

  const hasilLangsung = useMemo(() => {
    if (!asal || !tujuan) return null;
    const lurusKm = Math.round(haversineKm(asal.lat, asal.lng, tujuan.lat, tujuan.lng) * 10) / 10;
    const jalanKm = estimasiJalanKm(lurusKm);
    const menit = estimasiMenitTempuh(jalanKm);
    const risiko = risikoFoodLossDariMenit(menit);
    return { lurusKm, jalanKm, menit, risiko };
  }, [asal, tujuan]);

  useEffect(() => {
    if (!asal || !tujuan || !hasilLangsung) {
      if (garisUkur) onChangeGaris(null);
      return;
    }
    const label = `${hasilLangsung.lurusKm} km lurus (~${hasilLangsung.jalanKm} km jalan · ${hasilLangsung.menit} mnt)`;
    if (
      garisUkur?.asal[0] === asal.lat &&
      garisUkur?.asal[1] === asal.lng &&
      garisUkur?.tujuan[0] === tujuan.lat &&
      garisUkur?.tujuan[1] === tujuan.lng &&
      garisUkur?.label === label
    ) {
      return;
    }
    onChangeGaris({
      asal: [asal.lat, asal.lng],
      tujuan: [tujuan.lat, tujuan.lng],
      label,
    });
  }, [asal, tujuan, hasilLangsung, garisUkur, onChangeGaris]);

  return (
    <div className="space-y-2.5 border-t border-border bg-card/70 px-3 py-2.5">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="inline-flex items-center gap-1.5 text-xs font-medium text-foreground">
          <Navigation className="size-3.5 text-sky-600" />
          Ukur jarak sentra pertanian / perkampungan ke pasar &amp; cold storage
        </span>
        {(asalId || tujuanId) && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-6 px-2 text-[11px]"
            onClick={() => {
              setAsalId("");
              setTujuanId("");
              onChangeGaris(null);
            }}
          >
            <RotateCcw className="size-3 mr-1" />
            Reset
          </Button>
        )}
      </div>

      <div className="grid grid-cols-1 gap-2 sm:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] sm:items-center">
        <Select value={asalId} onValueChange={setAsalId}>
          <SelectTrigger className="h-8 text-xs">
            <SelectValue placeholder="Pilih titik asal (Pertanian / Perkampungan)…" />
          </SelectTrigger>
          <SelectContent>
            {pilihanAsal.map((t) => (
              <SelectItem key={t.id} value={t.id}>
                [{JENIS_LABEL[t.jenis]}] {t.nama}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <ArrowRightLeft className="hidden size-3.5 text-muted-foreground sm:block" />

        <Select value={tujuanId} onValueChange={setTujuanId}>
          <SelectTrigger className="h-8 text-xs">
            <SelectValue placeholder="Pilih tujuan (Pasar / Cold storage)…" />
          </SelectTrigger>
          <SelectContent>
            {titikLain
              .filter((t) => t.id !== asalId)
              .map((t) => (
                <SelectItem key={t.id} value={t.id}>
                  [{JENIS_LABEL[t.jenis]}] {t.nama}
                </SelectItem>
              ))}
          </SelectContent>
        </Select>
      </div>

      {hasilLangsung && asal && tujuan && (
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1 rounded-md border border-sky-200 bg-sky-50/70 px-2.5 py-1.5 text-xs dark:border-sky-900 dark:bg-sky-950/30">
          <span>
            Jarak lurus:{" "}
            <strong className="num">{hasilLangsung.lurusKm} km</strong>
          </span>
          <span>
            Estimasi jalan darat:{" "}
            <strong className="num">{hasilLangsung.jalanKm} km</strong> (~
            <span className="num">{hasilLangsung.menit} menit</span>)
          </span>
          <StatusBadge
            tone={
              hasilLangsung.risiko === "TINGGI"
                ? "defisit"
                : hasilLangsung.risiko === "SEDANG"
                  ? "waspada"
                  : "normal"
            }
          >
            Risiko food loss: {hasilLangsung.risiko}
          </StatusBadge>
        </div>
      )}

      {asal && (fasilitasTerdekat.pasar.length > 0 || fasilitasTerdekat.cold.length > 0) && (
        <div className="flex flex-wrap items-center gap-1.5 text-[11px] text-muted-foreground">
          <span className="font-medium text-foreground">Terdekat dari {asal.nama}:</span>
          {fasilitasTerdekat.pasar.slice(0, 2).map((p) => (
            <button
              key={p.id}
              type="button"
              onClick={() => setTujuanId(p.id)}
              className="rounded border border-border bg-background px-1.5 py-0.5 hover:bg-secondary transition-colors"
            >
              Pasar {p.nama.replace(/^Pasar\s+/i, "")} ({p.jalanKm} km)
            </button>
          ))}
          {fasilitasTerdekat.cold.slice(0, 2).map((c) => (
            <button
              key={c.id}
              type="button"
              onClick={() => setTujuanId(c.id)}
              className="rounded border border-purple-200 bg-purple-50/60 px-1.5 py-0.5 text-purple-800 hover:bg-purple-100 transition-colors dark:border-purple-900 dark:bg-purple-950/30 dark:text-purple-200"
            >
              {c.nama} ({c.jalanKm} km)
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
