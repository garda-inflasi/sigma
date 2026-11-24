"use client";

import { Calendar, Filter, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { kelasKontrol } from "@/components/ui/kontrol";
import { KABUPATEN, KOMODITAS } from "@/lib/data/reference";
import { cn } from "@/lib/utils";
import { useFilterOperasional } from "./use-filter-operasional";

const INPUT_CLASS = cn(
  kelasKontrol,
  "h-9 w-full min-w-0 px-2.5 text-sm text-foreground md:text-[13px]",
);
// Di desktop bilah menjadi satu baris: label tetap ada untuk pembaca layar, nilainya sudah menjelaskan isi kontrol.
const LABEL_CLASS =
  "mb-1 flex items-center gap-1.5 text-xs font-medium text-muted-foreground lg:sr-only";

/**
 * Bilah filter operasional: wilayah kabupaten/kota, komoditas pangan pokok, dan rentang tanggal
 * (maksimum 90 hari), tersinkronisasi dengan kueri URL (?kabupaten_kode=&komoditas_kode=&dari=&sampai=).
 */
export function OpsFilterBar() {
  const { mentah, galatRentang, adaFilter, setFilter, resetFilter } =
    useFilterOperasional();
  const hariIni = new Date().toISOString().slice(0, 10);

  return (
    <section
      aria-label="Filter dashboard operasional"
      className="glass shrink-0 rounded-lg p-2"
    >
      <div className="grid grid-cols-2 gap-2 lg:flex lg:items-center">
        <div className="min-w-0 lg:w-52 xl:w-60">
          <label htmlFor="ops-filter-kabupaten" className={LABEL_CLASS}>
            <Filter className="size-3.5 shrink-0" />
            <span className="truncate">Wilayah Kabupaten/Kota</span>
          </label>
          <select
            id="ops-filter-kabupaten"
            value={mentah.kabupatenKode ?? ""}
            onChange={(e) =>
              setFilter({ kabupatenKode: e.target.value || undefined })
            }
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

        <div className="min-w-0 lg:w-48 xl:w-60">
          <label htmlFor="ops-filter-komoditas" className={LABEL_CLASS}>
            Komoditas Pangan
          </label>
          <select
            id="ops-filter-komoditas"
            value={mentah.komoditasKode ?? ""}
            onChange={(e) =>
              setFilter({ komoditasKode: e.target.value || undefined })
            }
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

        <span
          aria-hidden
          className="hidden shrink-0 items-center gap-1.5 pl-1 text-xs font-medium text-muted-foreground lg:flex"
        >
          <Calendar className="size-3.5" /> Periode
        </span>

        <div className="min-w-0 lg:w-36">
          <label htmlFor="ops-filter-dari" className={LABEL_CLASS}>
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

        <span
          aria-hidden
          className="hidden shrink-0 text-xs text-muted-foreground lg:block"
        >
          s.d.
        </span>

        <div className="min-w-0 lg:w-36">
          <label htmlFor="ops-filter-sampai" className={LABEL_CLASS}>
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

        <div
          className={cn(
            "col-span-2 flex items-center justify-end lg:ml-auto",
            !adaFilter && "max-lg:hidden",
          )}
        >
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-9 text-[13px]"
            onClick={resetFilter}
            disabled={!adaFilter}
          >
            <RotateCcw className="size-3.5" />
            Reset filter
          </Button>
        </div>
      </div>

      {galatRentang && (
        <p role="alert" className="mt-2 text-xs font-medium text-defisit-ink">
          {galatRentang}
        </p>
      )}
    </section>
  );
}
