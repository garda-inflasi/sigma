"use client";

import { Calendar, Filter, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { DateField } from "@/components/ui/date-field";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { KABUPATEN, KOMODITAS } from "@/lib/data/reference";
import { cn } from "@/lib/utils";
import { useFilterOperasional } from "./use-filter-operasional";

const SEMUA = "__semua";
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
          <Select
            value={mentah.kabupatenKode ?? SEMUA}
            onValueChange={(v) =>
              setFilter({ kabupatenKode: v === SEMUA ? undefined : v })
            }
          >
            <SelectTrigger id="ops-filter-kabupaten" className="text-[13px] md:text-[13px]">
              <SelectValue placeholder="Seluruh Aceh" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={SEMUA}>Seluruh Aceh</SelectItem>
              {KABUPATEN.map((k) => (
                <SelectItem key={k.kode} value={k.kode}>
                  {k.nama}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="min-w-0 lg:w-48 xl:w-60">
          <label htmlFor="ops-filter-komoditas" className={LABEL_CLASS}>
            Komoditas Pangan
          </label>
          <Select
            value={mentah.komoditasKode ?? SEMUA}
            onValueChange={(v) =>
              setFilter({ komoditasKode: v === SEMUA ? undefined : v })
            }
          >
            <SelectTrigger id="ops-filter-komoditas" className="text-[13px] md:text-[13px]">
              <SelectValue placeholder="Semua komoditas" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={SEMUA}>Semua komoditas</SelectItem>
              {KOMODITAS.map((k) => (
                <SelectItem key={k.kode} value={k.kode}>
                  {k.nama}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
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
          <DateField
            id="ops-filter-dari"
            max={mentah.sampai || hariIni}
            value={mentah.dari ?? ""}
            onValueChange={(v) => setFilter({ dari: v || undefined })}
            kelasInput="px-2.5 pr-9 text-sm text-foreground md:text-[13px]"
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
          <DateField
            id="ops-filter-sampai"
            min={mentah.dari || undefined}
            max={hariIni}
            value={mentah.sampai ?? ""}
            onValueChange={(v) => setFilter({ sampai: v || undefined })}
            kelasInput="px-2.5 pr-9 text-sm text-foreground md:text-[13px]"
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
