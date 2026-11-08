"use client";

import {
  LABEL_KEKUATAN,
  SKOR_KEKUATAN,
  kekuatanSandi,
  type KekuatanSandi,
} from "@/lib/auth/password-strength";
import { toneClass, type Tone } from "@/lib/design/tokens";
import { cn } from "@/lib/utils";

const NADA: Record<KekuatanSandi, Tone> = {
  lemah: "defisit",
  sedang: "waspada",
  kuat: "normal",
};

const SEGMEN = [1, 2, 3] as const;

/** Meter kekuatan kata sandi: tiga segmen dan label teks. Tidak dirender selagi isian kosong. */
export function MeterKekuatanSandi({
  sandi,
  className,
}: {
  sandi: string;
  className?: string;
}) {
  const kekuatan = kekuatanSandi(sandi);
  if (!kekuatan) return null;
  const skor = SKOR_KEKUATAN[kekuatan];
  const label = LABEL_KEKUATAN[kekuatan];
  const nada = toneClass[NADA[kekuatan]];

  return (
    <div
      role="meter"
      aria-label="Kekuatan kata sandi"
      aria-valuemin={0}
      aria-valuemax={3}
      aria-valuenow={skor}
      aria-valuetext={label}
      className={cn("flex items-center gap-3", className)}
    >
      <div aria-hidden className="flex flex-1 gap-1.5">
        {SEGMEN.map((i) => (
          <span
            key={i}
            className={cn(
              "h-1.5 flex-1 rounded-full transition-colors duration-300",
              i <= skor ? nada.bg : "bg-mist-200 dark:bg-white/10",
            )}
          />
        ))}
      </div>
      <span className={cn("w-14 text-xs font-medium", nada.text)}>{label}</span>
    </div>
  );
}
