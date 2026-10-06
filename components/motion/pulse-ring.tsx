"use client";

import { motion, useReducedMotion } from "motion/react";
import type { ReactNode } from "react";
import { pulseRing } from "@/lib/motion/presets";
import { cn } from "@/lib/utils";
import type { Tone } from "@/lib/design/tokens";

const ringColor: Record<Tone, string> = {
  data: "bg-data-500",
  normal: "bg-normal",
  waspada: "bg-waspada",
  defisit: "bg-defisit",
  surplus: "bg-surplus",
  kritis: "bg-kritis",
  netral: "bg-mist-400",
};

interface PulseRingProps {
  tone?: Tone;
  rings?: number;
  active?: boolean;
  className?: string;
  /** Konten tengah (ikon). Tanpa children → titik kecil. */
  children?: ReactNode;
}

/** Cincin denyut bertingkat untuk sinyal EWS. Diam saat reduced-motion. */
export function PulseRing({ tone = "kritis", rings = 2, active = true, className, children }: PulseRingProps) {
  const reduce = useReducedMotion();
  return (
    <span className={cn("relative inline-grid place-items-center", className)}>
      {active && !reduce &&
        Array.from({ length: rings }).map((_, i) => {
          const p = pulseRing(i * 0.65);
          return (
            <motion.span
              key={i}
              aria-hidden
              className={cn("absolute inset-0 rounded-full", ringColor[tone])}
              initial={p.initial}
              animate={p.animate}
              transition={p.transition}
            />
          );
        })}
      {children ?? <span className={cn("relative size-2 rounded-full", ringColor[tone])} />}
    </span>
  );
}
