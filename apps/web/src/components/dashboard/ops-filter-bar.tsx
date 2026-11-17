"use client";

import { Calendar, Filter, RotateCcw } from "lucide-react";
import { KABUPATEN, KOMODITAS } from "@/lib/data/reference";
import { useFilterOperasional } from "./use-filter-operasional";

const INPUT_CLASS =
  "h-9 w-full rounded-lg border border-input bg-white/80 px-2.5 text-sm text-foreground outline-none transition-shadow focus:border-data-400 focus:ring-2 focus:ring-data-400/20 dark:bg-white/5";

/**
 * Bilah filter operasional: wilayah kabupaten/kota, komoditas pangan pokok, dan rentang tanggal
 * (maksimum 90 hari), tersinkronisasi dengan kueri URL (?kabupaten_kode=&komoditas_kode=&dari=&sampai=).
 */
export function OpsFilterBar() {
  const { mentah, galatRentang, adaFilter, setFilter, resetFilter } = useFilterOperasional();
  const hariIni = new Date().toISOString().slice(0, 10);

  return (
    <section
      aria-label="Filter dashboard operasional"
      className="glass rounded-xl p-4"
    >
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-12 lg:items-end">
        <div className="lg:col-span-3">
          <label
            htmlFor="ops-filter-kabupaten"
            className="mb-1 flex items-center gap-1.5 text-xs font-medium text-muted-foreground"
          >
            <Filter className="size-3.5" /> Wilayah Kabupaten/Kota
          </label>
          <select
            id="ops-filter-kabupaten"
            value={mentah.kabupatenKode ?? ""}
            onChange={(e) => setFilter({ kabupatenKode: e.target.value || undefined })}
            className={INPUT_CLASS}
          >
            <option value="">Seluruh Aceh</option>
            {KABUPATEN.map((k) => (
              <option key={k.kode} value={k.kode}>
                {k.nama}
              </option>
            ))}
          </select>
        </div>

        <div className="lg:col-span-3">
          <label
            htmlFor="ops-filter-komoditas"
            className="mb-1 block text-xs font-medium text-muted-foreground"
          >
            Komoditas Pangan
          </label>
          <select
            id="ops-filter-komoditas"
            value={mentah.komoditasKode ?? ""}
            onChange={(e) => setFilter({ komoditasKode: e.target.value || undefined })}
            className={INPUT_CLASS}
          >
            <option value="">Semua komoditas</option>
            {KOMODITAS.map((k) => (
              <option key={k.kode} value={k.kode}>
                {k.nama}
              </option>
            ))}
          </select>
        </div>

        <div className="lg:col-span-2">
          <label
            htmlFor="ops-filter-dari"
            className="mb-1 flex items-center gap-1.5 text-xs font-medium text-muted-foreground"
          >
            <Calendar className="size-3.5" /> Dari tanggal
          </label>
          <input
            id="ops-filter-dari"
            type="date"
            max={mentah.sampai || hariIni}
            value={mentah.dari ?? ""}
            onChange={(e) => setFilter({ dari: e.target.value || undefined })}
            className={INPUT_CLASS}
          />
        </div>

        <div className="lg:col-span-2">
          <label
            htmlFor="ops-filter-sampai"
            className="mb-1 block text-xs font-medium text-muted-foreground"
          >
            Sampai tanggal
          </label>
          <input
            id="ops-filter-sampai"
            type="date"
            min={mentah.dari || undefined}
            max={hariIni}
            value={mentah.sampai ?? ""}
            onChange={(e) => setFilter({ sampai: e.target.value || undefined })}
            className={INPUT_CLASS}
          />
        </div>

        <div className="flex items-center justify-end lg:col-span-2">
          <button
            type="button"
            onClick={resetFilter}
            disabled={!adaFilter}
            className="inline-flex h-9 w-full items-center justify-center gap-1.5 rounded-lg border border-border bg-white/60 px-3 text-xs font-medium text-foreground transition-colors hover:bg-white disabled:cursor-not-allowed disabled:opacity-45 dark:bg-white/5 dark:hover:bg-white/10"
          >
            <RotateCcw className="size-3.5" />
            Reset filter
          </button>
        </div>
      </div>

      {galatRentang && (
        <p role="alert" className="mt-2.5 text-xs font-medium text-defisit-ink">
          {galatRentang}
        </p>
      )}
    </section>
  );
}
