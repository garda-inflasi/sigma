"use client";

import { motion } from "motion/react";
import { PulseRing } from "@/components/motion/pulse-ring";
import { toneClass, type Tone } from "@/lib/design/tokens";
import { markerBounce } from "@/lib/motion/presets";
import { cn } from "@/lib/utils";

export interface MarkerWilayah {
  kode: string;
  nama: string;
  lat: number;
  lng: number;
  delta: number;
  komoditas: string;
  status: "defisit" | "waspada" | "surplus" | "normal";
  punyaPasar: boolean;
}

/** Pin wilayah: jatuh-memantul dengan stagger; wilayah defisit berdenyut. */
export function MarkerPin({ m, index, selected, onClick }: { m: MarkerWilayah; index: number; selected?: boolean; onClick?: () => void }) {
  const tone = m.status as Tone;
  return (
    <motion.button
      custom={index}
      variants={markerBounce}
      initial="hidden"
      animate="show"
      whileHover={{ scale: 1.18 }}
      onClick={onClick}
      aria-label={`${m.nama}: ${m.komoditas}`}
      className="group relative grid place-items-center"
    >
      <PulseRing tone={tone} rings={m.status === "defisit" ? 2 : 1} active={m.status === "defisit" || m.status === "waspada"} className={cn(m.punyaPasar ? "size-4" : "size-3")}>
        <span className={cn("relative block rounded-full ring-[3px] ring-white shadow-ambient dark:ring-navy-900", toneClass[tone].bg, m.punyaPasar ? "size-4" : "size-3 opacity-80", selected && "ring-navy-800 dark:ring-white")} />
      </PulseRing>
      <span className="pointer-events-none absolute bottom-full mb-2 hidden whitespace-nowrap rounded-lg bg-navy-900 px-2 py-1 text-[11px] text-white shadow-ambient group-hover:block group-focus-visible:block">
        {m.nama}
      </span>
    </motion.button>
  );
}
