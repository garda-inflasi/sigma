"use client";

import { Lock } from "lucide-react";
import { motion } from "motion/react";
import { useMemo, useState } from "react";
import { PanelTitle } from "@/components/garda/page-header";
import { PanelGalat } from "@/components/garda/panel-galat";
import { Segmented } from "@/components/garda/segmented";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import { ApiError } from "@/lib/api/client";
import { capaianPct } from "@/lib/api/dashboard";
import type { PetugasAktivitas } from "@/lib/api/dashboard-operasional";
import { formatAngka, formatTanggal } from "@/lib/format";
import { ease } from "@/lib/motion/presets";
import { useAktivitasLapangan, useAktivitasPetugas } from "@/lib/query/hooks";
import type { Tone } from "@/lib/design/tokens";
import { cn } from "@/lib/utils";

type Tampilan = "wilayah" | "petugas";

/** Petugas yang dirender sebelum "Tampilkan semua": daftar bisa ratusan, jadi tidak dirender seluruhnya sekaligus. */
export const BATAS_PETUGAS = 20;

const warnaBar = (pct: number) =>
  pct >= 80 ? "bg-normal" : pct >= 50 ? "bg-data-500" : "bg-waspada";

/** Bar capaian entri terhadap target; `pct` sudah dibatasi 0-100. */
function BarCapaian({
  pct,
  label,
  delay,
}: {
  pct: number;
  label: string;
  delay: number;
}) {
  return (
    <div
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={Math.round(pct)}
      className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-mist-200 dark:bg-white/10"
    >
      <motion.div
        className={cn("h-full rounded-full", warnaBar(pct))}
        initial={{ width: 0 }}
        animate={{ width: `${pct}%` }}
        transition={{ duration: 0.9, ease: ease.garda, delay }}
      />
    </div>
  );
}

function DaftarMemuat() {
  return (
    <ul className="mt-5 space-y-3.5" aria-busy="true">
      {Array.from({ length: 6 }).map((_, i) => (
        <li key={i}>
          <Shimmer className="h-9 w-full" />
        </li>
      ))}
    </ul>
  );
}

const KOSONG = "Belum ada petugas lapangan aktif.";

/** Capaian per kabupaten/kota (GET /aktivitas-lapangan); yang tertinggi di atas. */
function PerWilayah() {
  const { data, isLoading, isError, error, refetch } = useAktivitasLapangan();
  const perKab = useMemo(
    () =>
      (data ?? [])
        .map((a) => ({ ...a, pct: capaianPct(a) }))
        .sort((a, b) => b.pct - a.pct),
    [data],
  );

  if (isLoading) return <DaftarMemuat />;
  // Galat muat ulang berkala tidak menghapus data yang sudah tampil; panel galat hanya bila belum ada data sama sekali.
  if (isError && !data)
    return (
      <PanelGalat
        judul="Aktivitas per wilayah gagal dimuat"
        error={error}
        onCoba={() => void refetch()}
        className="mt-5"
      />
    );
  if (perKab.length === 0)
    return <p className="mt-5 text-sm text-muted-foreground">{KOSONG}</p>;

  return (
    <ul className="mt-5 space-y-3.5">
      {perKab.map((k, i) => (
        <li key={k.kode}>
          <div className="flex items-baseline justify-between gap-2 text-sm">
            <span className="truncate">{k.nama}</span>
            <span className="num shrink-0 text-xs text-muted-foreground">
              <span className="font-semibold text-foreground">{k.entri}</span>/
              {k.target}
            </span>
          </div>
          <BarCapaian
            pct={k.pct}
            label={`Capaian ${k.nama}`}
            delay={0.1 + i * 0.05}
          />
        </li>
      ))}
    </ul>
  );
}

/** Penanda kecil di bawah nama petugas (menunggu validasi, ditolak, tugas terlambat). */
function Tanda({
  tone,
  jumlah,
  title,
  children,
}: {
  tone: Tone;
  jumlah: number;
  title?: string;
  /** Kata setelah angka, mis. "menunggu validasi". */
  children: string;
}) {
  return (
    <StatusBadge tone={tone} dot={false} className="h-5 px-2 text-[11px]">
      <span title={title}>
        <span className="num">{formatAngka(jumlah)}</span> {children}
      </span>
    </StatusBadge>
  );
}

/** Tooltip jumlah entri: kapan entri terakhir petugas (tanggal yang tak terbaca tidak boleh merusak daftar). */
function keteranganEntriTerakhir(iso: string | null): string | undefined {
  if (!iso) return "Belum pernah mencatat entri";
  const waktu = new Date(iso);
  return Number.isNaN(waktu.getTime())
    ? undefined
    : `Entri terakhir ${formatTanggal(waktu, "d MMM yyyy HH.mm")}`;
}

function BarisPetugas({
  p,
  pct,
  jendelaHari,
  urutan,
}: {
  p: PetugasAktivitas;
  pct: number;
  jendelaHari: number;
  urutan: number;
}) {
  return (
    <li>
      <div className="flex items-baseline justify-between gap-2 text-sm">
        <span className="truncate" title={p.nama}>
          {p.nama}
        </span>
        <span
          className="num shrink-0 text-xs text-muted-foreground"
          title={keteranganEntriTerakhir(p.entriTerakhirPada)}
        >
          <span className="font-semibold text-foreground">
            {p.entriHariIni}
          </span>
          /{p.targetHarian}
        </span>
      </div>
      <BarCapaian
        pct={pct}
        label={`Capaian ${p.nama}`}
        delay={0.1 + Math.min(urutan, 8) * 0.05}
      />
      <div className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-[11px] text-muted-foreground">
        <span className="truncate">{p.kabupatenNama}</span>
        {p.menungguValidasi > 0 && (
          <Tanda tone="waspada" jumlah={p.menungguValidasi}>
            menunggu validasi
          </Tanda>
        )}
        {p.tugasTerlambat > 0 && (
          <Tanda tone="defisit" jumlah={p.tugasTerlambat}>
            tugas terlambat
          </Tanda>
        )}
        {p.ditolakJendela > 0 && (
          <Tanda
            tone="netral"
            jumlah={p.ditolakJendela}
            title={`Entri ditolak dalam ${jendelaHari} hari terakhir`}
          >
            ditolak
          </Tanda>
        )}
      </div>
    </li>
  );
}

/**
 * Capaian per petugas (GET /aktivitas-petugas), diminta hanya saat tampilan ini dibuka. Yang capaiannya terendah
 * di atas supaya terlihat siapa yang perlu dihubungi; hanya `BATAS_PETUGAS` pertama yang dirender sampai
 * "Tampilkan semua", dan daftar yang panjang bergulir di dalam panel.
 */
function PerPetugas() {
  const { data, isLoading, isError, error, refetch } = useAktivitasPetugas();
  const [semua, setSemua] = useState(false);
  const urut = useMemo(
    () =>
      (data?.petugas ?? [])
        .map((p) => ({
          p,
          pct: capaianPct({ entri: p.entriHariIni, target: p.targetHarian }),
        }))
        // Urutan sortir stabil: yang sama capaiannya tetap menurut kabupaten dan nama (urutan dari server).
        .sort((a, b) => a.pct - b.pct),
    [data],
  );

  if (isLoading) return <DaftarMemuat />;
  if (isError && !data) {
    // 403: peran ini tidak boleh melihat nama dan ID petugas. Bukan galat: keterangan saja, tampilan per wilayah tetap ada.
    if (error instanceof ApiError && error.status === 403) {
      return (
        <div
          role="status"
          className="mt-5 flex items-start gap-2.5 rounded-xl bg-mist-100 p-3 text-sm text-muted-foreground dark:bg-white/5"
        >
          <Lock className="mt-0.5 size-4 shrink-0" aria-hidden />
          <p>
            Peran Anda tidak boleh melihat rincian per petugas. Capaian per
            wilayah tetap dapat dilihat.
          </p>
        </div>
      );
    }
    return (
      <PanelGalat
        judul="Aktivitas per petugas gagal dimuat"
        error={error}
        onCoba={() => void refetch()}
        className="mt-5"
      />
    );
  }
  if (urut.length === 0)
    return <p className="mt-5 text-sm text-muted-foreground">{KOSONG}</p>;

  const tampil = semua ? urut : urut.slice(0, BATAS_PETUGAS);
  const jendelaHari = data?.jendelaHari ?? 0;

  return (
    <>
      <div
        role="region"
        aria-label="Daftar petugas lapangan"
        tabIndex={0}
        className="scrollbar-thin mt-5 max-h-[26rem] overflow-y-auto pr-1"
      >
        <ul className="space-y-3.5">
          {tampil.map(({ p, pct }, i) => (
            <BarisPetugas
              key={p.id}
              p={p}
              pct={pct}
              jendelaHari={jendelaHari}
              urutan={i}
            />
          ))}
        </ul>
      </div>
      {urut.length > BATAS_PETUGAS && (
        <button
          type="button"
          onClick={() => setSemua((v) => !v)}
          className="mt-3 text-xs font-medium text-data-700 hover:underline dark:text-data-300"
        >
          {semua
            ? "Ringkas daftar"
            : `Tampilkan semua ${formatAngka(urut.length)} petugas`}
        </button>
      )}
    </>
  );
}

const DESKRIPSI: Record<Tampilan, string> = {
  wilayah: "Capaian entri hari ini terhadap target",
  petugas: "Capaian entri hari ini per petugas, terendah di atas",
};

/**
 * Aktivitas tim lapangan: capaian entri hari ini terhadap target, per wilayah (bawaan) atau per petugas. Tampilan per
 * petugas memuat nama petugas dari API dan hanya diminta saat dipilih; peran yang tidak boleh melihatnya (403)
 * mendapat keterangan, dan tampilan per wilayah tetap berfungsi.
 */
export function FieldActivity() {
  const [tampilan, setTampilan] = useState<Tampilan>("wilayah");

  return (
    <section className="glass h-full rounded-xl p-5 sm:p-6">
      <PanelTitle
        title="Aktivitas tim lapangan"
        description={DESKRIPSI[tampilan]}
      />
      <Segmented
        layoutId="field-activity-tampilan"
        size="sm"
        className="mt-3"
        value={tampilan}
        onChange={setTampilan}
        options={[
          { value: "wilayah", label: "Per wilayah" },
          { value: "petugas", label: "Per petugas" },
        ]}
      />
      {tampilan === "wilayah" ? <PerWilayah /> : <PerPetugas />}
    </section>
  );
}
