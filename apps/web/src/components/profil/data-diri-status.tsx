"use client";

import { CircleAlert, Loader2, RotateCw } from "lucide-react";
import { Shimmer } from "@/components/garda/shimmer";
import { SpringButton } from "@/components/garda/spring-button";

/**
 * Kerangka tab Edit Profil saat memuat. Susunan dan tinggi blok mengikuti isi sesungguhnya (ringkasan,
 * form, penugasan) supaya tab tidak melompat ketika data tiba.
 */
export function DataDiriMemuat() {
  return (
    <div role="status" aria-label="Memuat profil" className="space-y-5">
      <div className="glass-strong flex flex-col items-center gap-4 rounded-xl p-5 min-h-[230px] sm:min-h-[140px] sm:flex-row sm:gap-5 sm:p-6">
        <Shimmer className="size-20 shrink-0 rounded-2xl" />
        <div className="flex w-full flex-1 flex-col items-center gap-2.5 sm:items-start">
          <Shimmer className="h-6 w-48 max-w-full" />
          <Shimmer className="h-6 w-40 rounded-full" />
          <Shimmer className="h-4 w-32" />
        </div>
      </div>

      <div className="glass rounded-xl p-5 min-h-[557px] sm:p-6 md:min-h-[382px]">
        <Shimmer className="h-6 w-36" />
        <Shimmer className="mt-2 h-4 w-72 max-w-full" />
        <div className="mt-6 grid gap-4 md:grid-cols-2">
          <Shimmer className="h-[68px]" />
          <Shimmer className="h-[68px]" />
          <Shimmer className="h-[68px] md:col-span-2" />
        </div>
        <div className="mt-6 flex justify-end gap-2">
          <Shimmer className="h-11 w-44" />
          <Shimmer className="h-11 w-40" />
        </div>
      </div>

      <div className="glass rounded-xl p-5 min-h-[480px] sm:p-6 sm:min-h-[356px]">
        <Shimmer className="h-6 w-48" />
        <Shimmer className="mt-2 h-4 w-80 max-w-full" />
        <div className="mt-6 grid gap-4 sm:grid-cols-2">
          <Shimmer className="h-12" />
          <Shimmer className="h-12" />
          <Shimmer className="h-12" />
          <Shimmer className="h-12" />
        </div>
        <Shimmer className="mt-5 h-11 w-full" />
      </div>
    </div>
  );
}

/** Profil gagal dimuat: pesan dan tombol untuk mencoba lagi. */
export function DataDiriGalat({
  pesan,
  mengulang,
  onUlangi,
}: {
  pesan?: string;
  /** True selama pemuatan ulang berjalan. */
  mengulang: boolean;
  onUlangi: () => void;
}) {
  return (
    <div
      role="alert"
      className="glass flex flex-col items-center gap-3 rounded-xl px-6 py-12 text-center"
    >
      <span className="grid size-12 place-items-center rounded-full bg-defisit-soft/80 text-defisit-ink dark:bg-defisit/15 dark:text-rose-300">
        <CircleAlert className="size-6" aria-hidden />
      </span>
      <div className="space-y-1">
        <p className="font-display text-lg font-semibold text-navy-900 dark:text-white">
          Profil tidak dapat dimuat
        </p>
        {pesan && <p className="text-sm text-muted-foreground">{pesan}</p>}
      </div>
      <SpringButton
        type="button"
        variant="outline"
        className="mt-1 h-11 px-5"
        disabled={mengulang}
        onClick={onUlangi}
      >
        {mengulang ? <Loader2 className="animate-spin" /> : <RotateCw />}
        Coba lagi
      </SpringButton>
    </div>
  );
}
