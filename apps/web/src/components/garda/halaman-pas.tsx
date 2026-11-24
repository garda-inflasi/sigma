import Link from "next/link";
import type { ReactNode } from "react";
import { toneClass, type Tone } from "@/lib/design/tokens";
import { cn } from "@/lib/utils";

/**
 * Kerangka halaman konsol: di desktop (`lg`) mengisi tinggi `<main>` sehingga halaman tidak perlu digulir dan tiap
 * panel menggulir isinya sendiri; di bawah `lg` mengalir normal mengikuti gulir dokumen.
 */
export function HalamanPas({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "flex w-full min-w-0 flex-col gap-3 lg:h-full lg:min-h-0",
        className,
      )}
    >
      {children}
    </div>
  );
}

/** Kolom panel bertumpuk di dalam `IsiPas`: tiap panel mengikuti isinya, dan menyusut bergulir bila kolom penuh. */
export const KOLOM_PAS =
  "flex w-full min-w-0 flex-col gap-3 lg:max-h-full lg:min-h-0";

/**
 * Area sisa di bawah header dan ringkasan, sekaligus batas tinggi isinya. `kolom` (kelas `lg:grid-cols-*`) menyusun
 * panel berdampingan: baris setinggi area ini, tetapi tiap panel berhenti mengikuti isinya. Panel visual yang memang
 * harus mengisi baris (peta, grafik utama) memakai `lg:h-full`.
 */
export function IsiPas({
  children,
  className,
  kolom,
}: {
  children: ReactNode;
  className?: string;
  kolom?: string;
}) {
  return (
    <div
      className={cn(
        "flex min-w-0 flex-col gap-3 lg:min-h-0 lg:flex-1",
        kolom &&
          "lg:grid lg:grid-rows-[minmax(0,1fr)] lg:items-start lg:*:max-h-full",
        kolom,
        className,
      )}
    >
      {children}
    </div>
  );
}

/**
 * Panel dengan judul tetap. Tingginya mengikuti isi dan isinya bergulir sendiri saat melebihi ruang di desktop.
 * `penuh` hanya untuk visualisasi yang harus mengisi area (diagram, grafik). `tanpaPadding` untuk tabel atau peta
 * yang mengisi panel sampai ke tepinya.
 */
export function PanelPas({
  judul,
  deskripsi,
  aksi,
  children,
  className,
  isiClassName,
  tanpaPadding,
  penuh,
  ...props
}: {
  judul?: ReactNode;
  deskripsi?: ReactNode;
  aksi?: ReactNode;
  children: ReactNode;
  className?: string;
  isiClassName?: string;
  tanpaPadding?: boolean;
  penuh?: boolean;
} & Omit<React.ComponentProps<"section">, "children" | "className">) {
  return (
    <section
      className={cn(
        "glass flex w-full min-w-0 flex-col overflow-hidden rounded-lg lg:max-h-full lg:min-h-0 lg:self-start",
        penuh && "lg:h-full lg:flex-1 lg:self-stretch",
        className,
      )}
      {...props}
    >
      {(judul || aksi) && (
        <div className="flex shrink-0 flex-wrap items-center justify-between gap-x-3 gap-y-1 border-b border-border px-3.5 py-2.5">
          <div className="min-w-0 flex-1 basis-40">
            {judul && (
              <h2 className="truncate text-display-sm font-semibold text-navy-900 dark:text-white">
                {judul}
              </h2>
            )}
            {deskripsi && (
              <p className="truncate text-xs text-muted-foreground">
                {deskripsi}
              </p>
            )}
          </div>
          {aksi && (
            <div className="flex shrink-0 flex-wrap items-center gap-2">
              {aksi}
            </div>
          )}
        </div>
      )}
      <div
        className={cn(
          "scrollbar-thin min-w-0 flex-1 lg:min-h-0 lg:overflow-y-auto",
          !tanpaPadding && "p-3.5",
          isiClassName,
        )}
      >
        {children}
      </div>
    </section>
  );
}

export interface ItemRingkas {
  label: string;
  nilai: ReactNode;
  /** Penjelasan singkat: apa artinya atau apa yang perlu dilakukan. */
  keterangan?: ReactNode;
  tone?: Tone;
  /** Penjelasan istilah saat kursor berada di kartu. */
  arti?: string;
  href?: string;
  onClick?: () => void;
  aktif?: boolean;
}

/** Baris ringkasan keputusan: kondisi saat ini, yang butuh tindakan, dan pintasan ke sana dalam satu pandangan. */
export function RingkasanKeputusan({
  items,
  label = "Ringkasan",
  className,
}: {
  items: ItemRingkas[];
  label?: string;
  className?: string;
}) {
  return (
    <ul
      aria-label={label}
      className={cn(
        "grid shrink-0 grid-cols-2 gap-2 lg:auto-cols-fr lg:grid-flow-col lg:grid-cols-none",
        className,
      )}
    >
      {items.map((it) => (
        <li key={it.label} className="min-w-0">
          <SelRingkas item={it} />
        </li>
      ))}
    </ul>
  );
}

function SelRingkas({ item }: { item: ItemRingkas }) {
  const isi = (
    <>
      <span className="flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
        {item.tone && (
          <span
            aria-hidden
            className={cn(
              "size-1.5 shrink-0 rounded-full",
              toneClass[item.tone].dot,
            )}
          />
        )}
        <span className="truncate">{item.label}</span>
      </span>
      <span className="num mt-0.5 block truncate text-lg leading-6 font-semibold text-foreground">
        {item.nilai}
      </span>
      {item.keterangan && (
        <span className="block truncate text-[11px] leading-4 text-muted-foreground">
          {item.keterangan}
        </span>
      )}
    </>
  );
  const kelas = cn(
    "glass block h-full w-full min-w-0 rounded-lg px-3 py-2 text-left",
    (item.href || item.onClick) &&
      "transition-colors hover:bg-secondary focus-visible:ring-[3px] focus-visible:ring-ring/30 focus-visible:outline-none",
    item.aktif && "border-navy-800 dark:border-white",
  );
  if (item.href)
    return (
      <Link href={item.href} title={item.arti} className={kelas}>
        {isi}
      </Link>
    );
  if (item.onClick)
    return (
      <button
        type="button"
        title={item.arti}
        aria-pressed={item.aktif}
        onClick={item.onClick}
        className={kelas}
      >
        {isi}
      </button>
    );
  return (
    <div title={item.arti} className={kelas}>
      {isi}
    </div>
  );
}
