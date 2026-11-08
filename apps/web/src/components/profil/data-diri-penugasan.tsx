"use client";

import { AnimatePresence, motion } from "motion/react";
import { ChevronDown, KeyRound } from "lucide-react";
import { useId, useState, type ReactNode } from "react";
import { PanelTitle } from "@/components/garda/page-header";
import { kelompokkanIzin } from "@/lib/profil/izin";
import { cakupanWilayah, infoPeran, labelLapis } from "@/lib/profil/penugasan";
import { cn } from "@/lib/utils";

function Butir({
  label,
  nilai,
  keterangan,
}: {
  label: string;
  nilai: ReactNode;
  keterangan?: string;
}) {
  return (
    <div className="min-w-0">
      <dt className="text-xs font-medium text-muted-foreground">{label}</dt>
      <dd className="mt-1 text-sm font-medium text-navy-900 dark:text-white">
        {nilai}
      </dd>
      {keterangan && (
        <dd className="mt-0.5 text-xs text-muted-foreground">{keterangan}</dd>
      )}
    </div>
  );
}

/** Daftar hak akses efektif, dikelompokkan menurut katalog izin. */
function DaftarHakAkses({ izin }: { izin: readonly string[] }) {
  return (
    <div className="space-y-4 pt-3 pb-1">
      <p className="text-xs text-muted-foreground">
        Hak akses mengikuti peran Anda ditambah izin langsung dari
        administrator.
      </p>
      {kelompokkanIzin(izin).map((kelompok) => (
        <div key={kelompok.nama}>
          <p className="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
            {kelompok.nama}
          </p>
          <ul
            aria-label={`Hak akses ${kelompok.nama}`}
            className="mt-2 flex flex-wrap gap-1.5"
          >
            {kelompok.izin.map((item) => (
              <li
                key={item.id}
                title={item.id}
                className="rounded-full bg-data-50 px-2.5 py-1 text-xs text-data-800 ring-1 ring-data-200/70 dark:bg-data-500/10 dark:text-data-200 dark:ring-data-500/30"
              >
                {item.label}
              </li>
            ))}
          </ul>
        </div>
      ))}
    </div>
  );
}

/**
 * "Penugasan & wilayah": lembaga, peran, lapis, cakupan wilayah, dan hak akses efektif. Semuanya hanya-baca;
 * administrator yang menetapkannya. Hak akses ada di bagian yang dibuka-tutup agar halaman tetap ringkas.
 */
export function PenugasanWilayah({
  lembaga,
  peran,
  lapis,
  kabupatenKode,
  namaKabupaten,
  izin,
}: {
  lembaga: string;
  /** Kode peran. */
  peran: string;
  lapis: string;
  /** Kosong berarti cakupan provinsi. */
  kabupatenKode?: string;
  namaKabupaten?: string;
  /** Izin efektif sesi (klaim token; matriks bawaan peran bila tidak ada). */
  izin: readonly string[];
}) {
  const [terbuka, setTerbuka] = useState(false);
  const idPanel = useId();
  const dataPeran = infoPeran(peran);
  const wilayah = cakupanWilayah(kabupatenKode, namaKabupaten, peran);
  const jumlah = new Set(izin).size;

  return (
    <section className="glass rounded-xl p-5 sm:p-6">
      <PanelTitle
        title="Penugasan & wilayah"
        description="Ditetapkan administrator. Hubungi administrator bila perlu diubah."
      />

      <dl className="mt-5 grid gap-x-6 gap-y-4 sm:grid-cols-2">
        <Butir label="Lembaga" nilai={lembaga || "-"} />
        <Butir
          label="Peran utama"
          nilai={dataPeran.label}
          keterangan={dataPeran.tugas}
        />
        <Butir label="Lapis akses" nilai={labelLapis(lapis)} />
        <Butir
          label="Cakupan wilayah"
          nilai={wilayah.wilayah}
          keterangan={[wilayah.tingkat, wilayah.batasPeran]
            .filter(Boolean)
            .join(" · ")}
        />
      </dl>

      <div className="mt-5 border-t border-border pt-2">
        {jumlah === 0 ? (
          <p className="py-3 text-sm text-muted-foreground">
            Belum ada hak akses yang diberikan.
          </p>
        ) : (
          <>
            <button
              type="button"
              aria-expanded={terbuka}
              aria-controls={idPanel}
              onClick={() => setTerbuka((v) => !v)}
              className="flex min-h-11 w-full items-center justify-between gap-3 rounded-lg px-1 text-left text-sm font-medium text-navy-800 transition-colors hover:text-data-700 focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none dark:text-white dark:hover:text-data-300"
            >
              <span className="flex items-center gap-2">
                <KeyRound
                  aria-hidden
                  className="size-4 text-muted-foreground"
                />
                {`Lihat hak akses (${jumlah})`}
              </span>
              <ChevronDown
                aria-hidden
                className={cn(
                  "size-4 text-muted-foreground transition-transform duration-200",
                  terbuka && "rotate-180",
                )}
              />
            </button>
            <AnimatePresence initial={false}>
              {terbuka && (
                <motion.div
                  id={idPanel}
                  initial={{ height: 0, opacity: 0 }}
                  animate={{ height: "auto", opacity: 1 }}
                  exit={{ height: 0, opacity: 0 }}
                  transition={{ duration: 0.2 }}
                  className="overflow-hidden"
                >
                  <DaftarHakAkses izin={izin} />
                </motion.div>
              )}
            </AnimatePresence>
          </>
        )}
      </div>
    </section>
  );
}
