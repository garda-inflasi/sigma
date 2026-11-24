"use client";

import { AlertTriangle, RotateCw } from "lucide-react";
import { ApiError } from "@/lib/api/client";
import { cn } from "@/lib/utils";
import { SpringButton } from "./spring-button";

/**
 * Keterangan galat muat data untuk pengguna: pesan server (berbahasa Indonesia) untuk galat HTTP, petunjuk sambungan
 * untuk galat lain (mis. jaringan putus, yang pesan aslinya teknis dan berbahasa Inggris).
 */
export function pesanGalatMuat(error: unknown): string {
  return error instanceof ApiError
    ? error.message
    : "Periksa sambungan internet, lalu coba lagi.";
}

/**
 * Keadaan galat di dalam widget saat query gagal (berbeda dari WidgetErrorBoundary, yang menangkap galat render).
 * Berisi apa yang gagal, keterangannya, dan tombol memuat ulang bila `onCoba` diberikan.
 */
export function PanelGalat({
  judul,
  error,
  onCoba,
  className,
}: {
  /** Apa yang gagal dimuat, mis. "Peta panas harga gagal dimuat". */
  judul: string;
  error?: unknown;
  onCoba?: () => void;
  className?: string;
}) {
  return (
    <div
      role="alert"
      className={cn(
        "flex flex-col items-center justify-center gap-2 rounded-lg bg-defisit/5 px-6 py-8 text-center",
        className,
      )}
    >
      <span className="grid size-10 place-items-center rounded-lg bg-defisit-soft text-defisit-ink dark:bg-defisit/15 dark:text-rose-300">
        <AlertTriangle className="size-5" strokeWidth={1.75} aria-hidden />
      </span>
      <p className="text-sm font-medium text-foreground">{judul}</p>
      <p className="max-w-sm text-[13px] text-muted-foreground">
        {pesanGalatMuat(error)}
      </p>
      {onCoba && (
        <SpringButton
          onClick={onCoba}
          size="sm"
          variant="soft"
          className="mt-1 gap-1.5"
        >
          <RotateCw className="size-3.5" aria-hidden />
          Coba lagi
        </SpringButton>
      )}
    </div>
  );
}
