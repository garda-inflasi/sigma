"use client";

import { motion } from "motion/react";
import { MapPin, Thermometer } from "lucide-react";
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
}

/** Ringkasan satu fasilitas: status, utilisasi, kapasitas terisi, dan suhu terhadap target. */
export function FacilityCard({ f, active = false, onSelect }: Props) {
  const status = STATUS[f.status];
  const rencana = f.status === "RENCANA";
  const persen = utilisasiPersen(f);
  const nada = nadaUtilisasi(persen);
  const deviasi = deviasiSuhu(f);
  const menyimpang = deviasi !== null && deviasi > AMBANG_DEVIASI_SUHU_C;
  // Fasilitas rencana belum punya stok, jadi tidak ada yang dapat disaring.
  const dapatDipilih = Boolean(onSelect) && !rencana;

  return (
    <motion.article
      variants={fadeUp}
      className={cn(
        "glass relative flex h-full min-w-0 flex-col rounded-xl p-5 transition-shadow duration-300",
        dapatDipilih && "hover:shadow-lift",
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
          className="absolute inset-0 z-10 rounded-xl"
        />
      )}

      <StatusBadge tone={status.tone} className="self-start">
        {status.label}
      </StatusBadge>

      {/* Nama boleh dua baris agar tidak terpotong (judul penuh lewat title). Tinggi blok
          dikunci untuk dua baris + wilayah supaya angka utilisasi sejajar antar-kartu. */}
      <div className="mt-3 min-h-16">
        <h3
          title={f.nama}
          className="line-clamp-2 font-display text-[15px] leading-snug font-semibold text-navy-900 dark:text-white"
        >
          {f.nama}
        </h3>
        <p className="mt-1 flex items-center gap-1 text-xs text-muted-foreground">
          <MapPin className="size-3 shrink-0" aria-hidden />
          <span className="truncate">{f.wilayah}</span>
        </p>
      </div>

      {rencana ? (
        <p className="mt-5 text-sm leading-relaxed text-muted-foreground">
          Studi kelayakan oleh Unit Pembiayaan &amp; Investasi. Kapasitas
          rencana{" "}
          <span className="num text-foreground">
            {formatAngka(f.kapasitasTon)} ton
          </span>
          .
        </p>
      ) : (
        <>
          <div className="mt-5 flex items-end justify-between gap-2">
            <p className="text-3xl leading-none font-semibold text-navy-900 dark:text-white">
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
            className="mt-2.5 h-2 overflow-hidden rounded-full bg-mist-200 dark:bg-white/10"
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
              "mt-4 flex items-center gap-1.5 text-sm",
              menyimpang ? "text-waspada-ink" : "text-muted-foreground",
            )}
            title={menyimpang ? "Suhu menyimpang dari target" : undefined}
          >
            <Thermometer className="size-4 shrink-0" aria-hidden />
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
