"use client";

import { motion } from "motion/react";
import { ChevronRight, MapPin, Thermometer, TriangleAlert } from "lucide-react";
import { StatusBadge } from "@/components/garda/status-badge";
import { CountUp } from "@/components/motion/count-up";
import {
  AMBANG_DEVIASI_SUHU_C,
  deviasiSuhu,
  labelUtilisasi,
  nadaUtilisasi,
  utilisasiPersen,
} from "@/lib/api/cold-storage";
import type { FasilitasColdStorage } from "@/lib/data/types";
import { toneClass, type Tone } from "@/lib/design/tokens";
import { formatAngka } from "@/lib/format";
import { ease, fadeUp } from "@/lib/motion/presets";
import { cn } from "@/lib/utils";

const STATUS: Record<
  FasilitasColdStorage["status"],
  { tone: Tone; label: string }
> = {
  OPERASIONAL: { tone: "normal", label: "Operasional" },
  PERAWATAN: { tone: "waspada", label: "Perawatan" },
  RENCANA: { tone: "netral", label: "Rencana" },
};

interface Props {
  f: FasilitasColdStorage;
  /** Fasilitas ini sedang dipakai sebagai filter tabel stok. */
  active?: boolean;
  /** Bila diisi, kartu dapat diklik untuk menyaring tabel stok pada fasilitas ini. */
  onSelect?: () => void;
  /** Bila diisi, tombol "Detail" membuka riwayat suhu, lot, dan transaksi fasilitas. */
  onDetail?: () => void;
}

/** Backend menghitung alarm dari batas fasilitas; data mock memakai selisih terhadap target. */
export function suhuMenyimpang(f: FasilitasColdStorage): boolean {
  const deviasi = deviasiSuhu(f);
  return f.alarmSuhu ?? (deviasi !== null && deviasi > AMBANG_DEVIASI_SUHU_C);
}

/** Ringkasan satu fasilitas: status, utilisasi, kapasitas terisi, dan suhu terhadap target. */
export function FacilityCard({ f, active = false, onSelect, onDetail }: Props) {
  const status = STATUS[f.status];
  const rencana = f.status === "RENCANA";
  const persen = utilisasiPersen(f);
  const nada = nadaUtilisasi(persen);
  const menyimpang = suhuMenyimpang(f);
  // Fasilitas rencana belum punya stok, jadi tidak ada yang dapat disaring.
  const dapatDipilih = Boolean(onSelect) && !rencana;

  return (
    <motion.article
      variants={fadeUp}
      className={cn(
        "relative flex h-full min-w-0 flex-col rounded-lg border border-border bg-card p-3.5 transition-colors",
        dapatDipilih && "hover:bg-secondary/60",
        active && "ring-2 ring-data-500/70",
      )}
    >
      {dapatDipilih && (
        <button
          type="button"
          onClick={onSelect}
          aria-pressed={active}
          aria-label={
            active
              ? `Hapus filter fasilitas ${f.nama}`
              : `Tampilkan stok ${f.nama}`
          }
          className="absolute inset-0 z-10 rounded-lg"
        />
      )}

      <div className="flex items-center justify-between gap-2">
        <div className="flex flex-wrap items-center gap-1.5">
          <StatusBadge tone={status.tone}>{status.label}</StatusBadge>
          {menyimpang && (
            <StatusBadge tone="defisit">
              <TriangleAlert className="mr-1 size-3" aria-hidden />
              Suhu di luar batas
            </StatusBadge>
          )}
        </div>
        {onDetail && !rencana && (
          <button
            type="button"
            onClick={onDetail}
            aria-label={`Detail ${f.nama}`}
            className="relative z-20 inline-flex items-center gap-0.5 rounded-md px-1.5 py-0.5 text-xs font-medium text-data-700 hover:bg-secondary dark:text-data-300"
          >
            Detail <ChevronRight className="size-3.5" aria-hidden />
          </button>
        )}
      </div>

      {/* Nama boleh dua baris; tinggi blok dikunci supaya angka utilisasi sejajar antar-kartu. */}
      <div className="mt-2 min-h-[3.75rem]">
        <h3
          title={f.nama}
          className="line-clamp-2 font-display text-sm leading-snug font-semibold text-navy-900 dark:text-white"
        >
          {f.nama}
        </h3>
        <p className="mt-0.5 flex items-center gap-1 text-xs text-muted-foreground">
          <MapPin className="size-3 shrink-0" aria-hidden />
          <span className="truncate">{f.wilayah}</span>
        </p>
      </div>

      {rencana ? (
        <p className="mt-2 text-[13px] leading-relaxed text-muted-foreground">
          Studi kelayakan oleh Unit Pembiayaan &amp; Investasi. Kapasitas
          rencana{" "}
          <span className="num text-foreground">
            {formatAngka(f.kapasitasTon)} ton
          </span>
          .
        </p>
      ) : (
        <>
          <div className="mt-2 flex items-end justify-between gap-2">
            <p className="text-2xl leading-none font-semibold text-navy-900 dark:text-white">
              <CountUp value={persen} format={(n) => `${Math.round(n)}%`} />
            </p>
            <p className="num text-xs whitespace-nowrap text-muted-foreground">
              {formatAngka(f.terisiTon)} / {formatAngka(f.kapasitasTon)} ton
            </p>
          </div>

          <div
            role="progressbar"
            aria-label={`Utilisasi ${f.nama}`}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={Math.round(persen)}
            className="mt-2 h-1.5 overflow-hidden rounded-full bg-mist-200 dark:bg-white/10"
          >
            <motion.div
              className={cn("h-full rounded-full", toneClass[nada].bg)}
              initial={{ width: 0 }}
              animate={{ width: `${Math.min(persen, 100)}%` }}
              transition={{ duration: 1, ease: ease.garda }}
            />
          </div>
          <p className={cn("mt-1.5 text-xs font-medium", toneClass[nada].text)}>
            {labelUtilisasi(persen)}
          </p>

          <p
            className={cn(
              "mt-1.5 flex items-center gap-1.5 text-xs",
              menyimpang ? "text-waspada-ink" : "text-muted-foreground",
            )}
            title={menyimpang ? "Suhu menyimpang dari target" : undefined}
          >
            <Thermometer className="size-3.5 shrink-0" aria-hidden />
            {f.suhuAktualC === null ? (
              <span>Belum ada data suhu</span>
            ) : (
              <span>
                <span className="num">{formatAngka(f.suhuAktualC, 1)}°C</span>{" "}
                (target{" "}
                <span className="num">
                  {formatAngka(
                    f.suhuTargetC,
                    Number.isInteger(f.suhuTargetC) ? 0 : 1,
                  )}
                  °C
                </span>
                )
              </span>
            )}
          </p>
        </>
      )}
    </motion.article>
  );
}
