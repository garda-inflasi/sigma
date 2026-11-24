"use client";

import { AnimatePresence, motion } from "motion/react";
import {
  Building2,
  ChevronDown,
  KeyRound,
  Layers,
  MapPin,
  Search,
  Shield,
  type LucideIcon,
} from "lucide-react";
import { useId, useMemo, useState, type ReactNode } from "react";
import { PanelTitle } from "@/components/garda/page-header";
import { Input } from "@/components/ui/input";
import { ease } from "@/lib/motion/presets";
import { kelompokkanIzin } from "@/lib/profil/izin";
import { cakupanWilayah, infoPeran, labelLapis } from "@/lib/profil/penugasan";
import { cn } from "@/lib/utils";

function Butir({
  ikon: Ikon,
  label,
  nilai,
  keterangan,
}: {
  ikon: LucideIcon;
  label: string;
  nilai: ReactNode;
  keterangan?: string;
}) {
  return (
    <div className="min-w-0 rounded-lg border border-border/80 bg-secondary/45 px-3 py-2.5 dark:bg-white/[0.03]">
      <dt className="flex items-center gap-1.5 text-[11px] font-medium text-muted-foreground">
        <Ikon className="size-3.5 shrink-0 text-data-600 dark:text-data-400" aria-hidden />
        <span>{label}</span>
      </dt>
      <dd className="mt-1 text-[13px] font-semibold text-navy-900 dark:text-white">
        {nilai}
      </dd>
      {keterangan && (
        <dd className="mt-0.5 text-xs leading-relaxed text-muted-foreground">
          {keterangan}
        </dd>
      )}
    </div>
  );
}

/** Daftar hak akses efektif, dikelompokkan menurut katalog izin dengan pencarian opsional. */
function DaftarHakAkses({ izin }: { izin: readonly string[] }) {
  const [cari, setCari] = useState("");
  const kelompok = useMemo(() => {
    const q = cari.trim().toLowerCase();
    const semua = kelompokkanIzin(izin);
    if (!q) return semua;
    return semua
      .map((k) => ({
        ...k,
        izin: k.izin.filter(
          (it) =>
            it.label.toLowerCase().includes(q) ||
            it.id.toLowerCase().includes(q) ||
            k.nama.toLowerCase().includes(q),
        ),
      }))
      .filter((k) => k.izin.length > 0);
  }, [izin, cari]);

  return (
    <div className="space-y-3.5 pt-3 pb-1">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <p className="text-xs text-muted-foreground">
          Hak akses mengikuti peran Anda ditambah izin langsung dari administrator.
        </p>
        {izin.length > 6 && (
          <div className="relative w-full sm:w-52">
            <Search
              aria-hidden
              className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground"
            />
            <Input
              type="search"
              value={cari}
              onChange={(e) => setCari(e.target.value)}
              placeholder="Saring hak akses…"
              aria-label="Saring hak akses"
              className="h-8 pl-8 text-xs"
            />
          </div>
        )}
      </div>

      {kelompok.length === 0 ? (
        <p className="rounded-lg border border-dashed border-border p-3 text-center text-xs text-muted-foreground">
          Tidak ada hak akses yang cocok dengan pencarian.
        </p>
      ) : (
        <div className="space-y-3">
          {kelompok.map((k) => (
            <div
              key={k.nama}
              className="rounded-lg border border-border/70 bg-secondary/30 p-2.5 dark:bg-white/[0.02]"
            >
              <p className="text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                {k.nama}
              </p>
              <ul
                aria-label={`Hak akses ${k.nama}`}
                className="mt-2 flex flex-wrap gap-1.5"
              >
                {k.izin.map((item) => (
                  <li
                    key={item.id}
                    title={item.id}
                    className="rounded-md bg-data-50 px-2 py-1 text-xs font-medium text-data-800 ring-1 ring-data-200/80 dark:bg-data-500/10 dark:text-data-200 dark:ring-data-500/30"
                  >
                    {item.label}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      )}
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
    <section className="glass scrollbar-thin min-w-0 rounded-xl p-4 sm:p-5 lg:max-h-full lg:min-h-0 lg:overflow-y-auto">
      <PanelTitle
        title="Penugasan & wilayah"
        description="Ditetapkan administrator. Hubungi administrator bila perlu diubah."
      />

      <dl className="mt-4 grid grid-cols-1 gap-2.5 sm:grid-cols-2">
        <Butir ikon={Building2} label="Lembaga" nilai={lembaga || "-"} />
        <Butir
          ikon={Shield}
          label="Peran utama"
          nilai={dataPeran.label}
          keterangan={dataPeran.tugas}
        />
        <Butir ikon={Layers} label="Lapis akses" nilai={labelLapis(lapis)} />
        <Butir
          ikon={MapPin}
          label="Cakupan wilayah"
          nilai={wilayah.wilayah}
          keterangan={[wilayah.tingkat, wilayah.batasPeran]
            .filter(Boolean)
            .join(" · ")}
        />
      </dl>

      <div className="mt-4 border-t border-border pt-2">
        {jumlah === 0 ? (
          <p className="py-2 text-sm text-muted-foreground">
            Belum ada hak akses yang diberikan.
          </p>
        ) : (
          <>
            <button
              type="button"
              aria-expanded={terbuka}
              aria-controls={idPanel}
              onClick={() => setTerbuka((v) => !v)}
              className="flex min-h-10 w-full items-center justify-between gap-3 rounded-lg px-2 py-1.5 text-left text-sm font-medium text-navy-800 transition-colors hover:bg-secondary/70 hover:text-data-700 focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none dark:text-white dark:hover:text-data-300"
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
                  transition={{ duration: 0.22, ease: ease.garda }}
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
