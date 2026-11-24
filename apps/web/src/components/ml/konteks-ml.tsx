"use client";

import { BrainCircuit } from "lucide-react";
import type { LampiranMl } from "@/lib/api/ml";
import { formatPersen } from "@/lib/format";
import {
  alasanBelumAndal,
  dapatDiandalkan,
  keadaanBlok,
  LABEL_BELUM_ANDAL,
  labelTargetMl,
  namaWilayahMl,
  ringkasBlokModel,
} from "@/lib/ml/label";
import {
  useLampiranMl,
  useMlAktif,
  useSiklusMlTerpilih,
} from "@/lib/query/ml-hooks";
import { cn } from "@/lib/utils";
import { BadgeSiklusMl, PeringatanSiklusMl } from "./siklus-ml";

/** Kalimat konteks blok `model` (halaman KPI). null bila blok tidak tampil. */
export function konteksModel(l: LampiranMl | undefined): string | null {
  const k = keadaanBlok(l, l?.model);
  if (k.jenis !== "tampil") return null;
  const r = ringkasBlokModel(k.blok);
  return [r.klasifikasi, r.regresi ? `Proyeksi harga: ${r.regresi}.` : null]
    .filter(Boolean)
    .join(" ");
}

/** Kalimat konteks blok `inflasi` dan `makro` (halaman policy brief). null bila blok inflasi tidak tampil. */
export function konteksInflasi(l: LampiranMl | undefined): string | null {
  const k = keadaanBlok(l, l?.inflasi);
  if (k.jenis !== "tampil") return null;
  const b = k.blok;
  const target = labelTargetMl(b.target);
  const status = dapatDiandalkan(b.kelayakan)
    ? `${b.wilayah.length} wilayah bersinyal.`
    : `${LABEL_BELUM_ANDAL} (${alasanBelumAndal(b.kelayakan, b.alasanKelayakan)}).`;
  const tertinggi = [...b.wilayah]
    .filter((w) => w.aktual.yoy !== null)
    .sort((a, c) => (c.aktual.yoy ?? 0) - (a.aktual.yoy ?? 0))[0];
  const aktual = tertinggi
    ? `Perubahan indeks aktual y-o-y tertinggi: ${namaWilayahMl(tertinggi.wilayahKode)} ${formatPersen(tertinggi.aktual.yoy ?? 0, { signed: true, digits: 2 })}.`
    : null;
  const m = keadaanBlok(l, l?.makro);
  const makro =
    m.jenis === "tampil"
      ? `${m.blok.items.length} indikator makro tersedia.`
      : "Indikator makro belum tersedia.";
  return [`${target}: ${status}`, aktual, makro].filter(Boolean).join(" ");
}

/**
 * Strip konteks hasil training satu baris di bawah ringkasan halaman: badge siklus yang dipakai dan kalimat jujur
 * dari blok yang tersedia. Tidak tampil di mode Staging atau bila hasil training tidak tersedia.
 */
export function KonteksMl({
  path,
  ringkas,
  className,
}: {
  /** GET endpoint lama halaman ini (lampiran `ml`-nya yang dibaca). */
  path: string;
  ringkas: (l: LampiranMl | undefined) => string | null;
  className?: string;
}) {
  const aktif = useMlAktif();
  const [cycleId] = useSiklusMlTerpilih();
  const lampiran = useLampiranMl(path, { cycleId, enabled: aktif });
  // Blok tidak diperiksa di sini: hanya `cycle_id` pilihan yang tidak dapat dipakai.
  const k = keadaanBlok(lampiran.data, undefined);
  if (!aktif) return null;
  if (k.jenis === "siklus")
    return (
      <PeringatanSiklusMl
        pesan={k.pesan}
        className={cn("shrink-0", className)}
      />
    );
  const teks = ringkas(lampiran.data);
  if (!teks) return null;
  return (
    <div
      role="note"
      className={cn(
        "flex shrink-0 flex-wrap items-center gap-x-2.5 gap-y-1 rounded-md border border-border bg-card px-2.5 py-1.5 text-xs lg:flex-nowrap",
        className,
      )}
    >
      <BadgeSiklusMl siklus={lampiran.data?.siklus} className="h-6" />
      <BrainCircuit
        className="size-3.5 shrink-0 text-muted-foreground"
        aria-hidden
      />
      <p title={teks} className="min-w-0 text-muted-foreground lg:truncate">
        {teks}
      </p>
    </div>
  );
}
