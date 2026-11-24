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
    <div
      role="status"
      aria-label="Memuat profil"
      className="flex min-w-0 flex-col gap-3"
    >
      <div className="glass flex shrink-0 items-center justify-between gap-3 rounded-xl px-4 py-4 sm:px-5">
        <div className="flex min-w-0 flex-1 items-center gap-3.5">
          <Shimmer className="size-12 shrink-0 rounded-xl" />
          <div className="flex min-w-0 flex-1 flex-col gap-2">
            <Shimmer className="h-5 w-52 max-w-full" />
            <Shimmer className="h-3.5 w-44 max-w-full" />
          </div>
        </div>
        <Shimmer className="hidden h-6 w-28 rounded-full sm:block" />
      </div>

      <div className="grid min-w-0 items-start gap-3 lg:grid-cols-[minmax(0,1.05fr)_minmax(0,0.95fr)]">
        <div className="glass rounded-xl p-5">
          <Shimmer className="h-5 w-36" />
          <Shimmer className="mt-2 h-4 w-72 max-w-full" />
          <div className="mt-5 grid grid-cols-1 gap-3.5 sm:grid-cols-2">
            <Shimmer className="h-14 rounded-lg" />
            <Shimmer className="h-14 rounded-lg" />
            <Shimmer className="h-14 rounded-lg sm:col-span-2" />
          </div>
          <div className="mt-5 flex justify-end gap-2 border-t border-border pt-3.5">
            <Shimmer className="h-9 w-36 rounded-md" />
            <Shimmer className="h-9 w-36 rounded-md" />
          </div>
        </div>
        <div className="glass rounded-xl p-5">
          <Shimmer className="h-5 w-48" />
          <Shimmer className="mt-2 h-4 w-80 max-w-full" />
          <div className="mt-5 grid grid-cols-1 gap-3 sm:grid-cols-2">
            <Shimmer className="h-16 rounded-lg" />
            <Shimmer className="h-16 rounded-lg" />
            <Shimmer className="h-16 rounded-lg" />
            <Shimmer className="h-16 rounded-lg" />
          </div>
        </div>
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
