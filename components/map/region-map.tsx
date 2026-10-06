"use client";

import { AnimatePresence, motion } from "motion/react";
import dynamic from "next/dynamic";
import { useState } from "react";
import { Shimmer } from "@/components/garda/shimmer";
import { formatPersen } from "@/lib/format";
import { popIn } from "@/lib/motion/presets";
import { cn } from "@/lib/utils";
import { MarkerPin, type MarkerWilayah } from "./marker-pin";

const MapboxRegionMap = dynamic(() => import("./region-map-mapbox"), { ssr: false, loading: () => <Shimmer className="size-full rounded-none" /> });

const TOKEN = process.env.NEXT_PUBLIC_MAPBOX_TOKEN;
const LEGENDA = [
  { s: "defisit", label: "Tekanan/defisit", cls: "bg-defisit" },
  { s: "waspada", label: "Waspada", cls: "bg-waspada" },
  { s: "normal", label: "Normal", cls: "bg-normal" },
  { s: "surplus", label: "Surplus", cls: "bg-surplus" },
];

export function RegionMap({ markers, isLoading, selected, onSelect, className }: { markers?: MarkerWilayah[]; isLoading?: boolean; selected?: string; onSelect?: (kode: string) => void; className?: string }) {
  return (
    <div className={cn("relative overflow-hidden rounded-xl bg-mist-100 dark:bg-navy-950/60", className)}>
      {isLoading || !markers ? (
        <Shimmer className="size-full rounded-none" />
      ) : TOKEN ? (
        <MapboxRegionMap token={TOKEN} markers={markers} selected={selected} onSelect={onSelect} />
      ) : (
        <SkemaAceh markers={markers} selected={selected} onSelect={onSelect} />
      )}
      <div className="glass-subtle absolute bottom-3 left-3 flex flex-wrap gap-x-3 gap-y-1 rounded-xl px-3 py-2 text-[11px] text-muted-foreground">
        {LEGENDA.map((l) => (
          <span key={l.s} className="inline-flex items-center gap-1.5"><span className={cn("size-2 rounded-full", l.cls)} />{l.label}</span>
        ))}
      </div>
    </div>
  );
}

/* ── Fallback tanpa token Mapbox: skema Aceh (tidak berskala) ── */
const B = { minLng: 94.9, maxLng: 98.5, minLat: 1.9, maxLat: 6.1 };
const px = (lng: number) => ((lng - B.minLng) / (B.maxLng - B.minLng)) * 100;
const py = (lat: number) => ((B.maxLat - lat) / (B.maxLat - B.minLat)) * 100;
const DARAT: [number, number][] = [
  [95.2, 5.6], [95.5, 5.62], [95.9, 5.45], [96.3, 5.27], [96.8, 5.25], [97.2, 5.18], [97.55, 5.1], [97.85, 4.85], [98.1, 4.55],
  [98.25, 4.2], [98.15, 3.9], [97.95, 3.55], [98.0, 3.1], [98.1, 2.75], [97.95, 2.45], [97.75, 2.2], [97.5, 2.35], [97.3, 2.9],
  [97.15, 3.25], [96.85, 3.62], [96.5, 3.85], [96.1, 4.12], [95.85, 4.35], [95.6, 4.62], [95.42, 4.95], [95.3, 5.25],
];
const poly = (pts: [number, number][]) => pts.map(([lng, lat]) => `${px(lng).toFixed(2)},${py(lat).toFixed(2)}`).join(" ");

function SkemaAceh({ markers, selected, onSelect }: { markers: MarkerWilayah[]; selected?: string; onSelect?: (kode: string) => void }) {
  const [aktif, setAktif] = useState<MarkerWilayah | null>(null);
  return (
    <div className="relative mx-auto aspect-[36/42] h-full max-w-full">
      <svg viewBox="0 0 100 100" preserveAspectRatio="none" className="absolute inset-0 size-full" aria-hidden>
        <polygon points={poly(DARAT)} className="fill-white/80 stroke-navy-300 dark:fill-white/5 dark:stroke-white/15" strokeWidth={0.35} strokeLinejoin="round" />
        <ellipse cx={px(96.1)} cy={py(2.62)} rx={3.6} ry={1.5} transform={`rotate(-28 ${px(96.1)} ${py(2.62)})`} className="fill-white/80 stroke-navy-300 dark:fill-white/5 dark:stroke-white/15" strokeWidth={0.35} />
        <ellipse cx={px(95.32)} cy={py(5.86)} rx={0.9} ry={0.6} className="fill-white/80 stroke-navy-300 dark:fill-white/5 dark:stroke-white/15" strokeWidth={0.35} />
      </svg>
      {markers.map((m, i) => (
        <div key={m.kode} className="absolute -translate-x-1/2 -translate-y-1/2" style={{ left: `${px(m.lng)}%`, top: `${py(m.lat)}%` }}>
          <MarkerPin m={m} index={i} selected={selected === m.kode} onClick={() => { setAktif(m); onSelect?.(m.kode); }} />
        </div>
      ))}
      <AnimatePresence>
        {aktif && (
          <motion.div key={aktif.kode} variants={popIn} initial="hidden" animate="show" exit="exit" className="glass-strong absolute top-3 right-3 w-52 rounded-xl p-3">
            <p className="font-display text-sm font-semibold">{aktif.nama}</p>
            <p className="mt-0.5 text-xs text-muted-foreground">Pergerakan terbesar: {aktif.komoditas}</p>
            <p className="num mt-1 text-lg font-semibold">{formatPersen(aktif.delta, { signed: true })}</p>
            <button onClick={() => setAktif(null)} className="mt-1 text-xs text-data-700 hover:underline dark:text-data-300">Tutup</button>
          </motion.div>
        )}
      </AnimatePresence>
      <p className="absolute top-3 left-3 text-[11px] text-muted-foreground">Skema wilayah (tidak berskala). Tambahkan token Mapbox untuk peta interaktif.</p>
    </div>
  );
}
