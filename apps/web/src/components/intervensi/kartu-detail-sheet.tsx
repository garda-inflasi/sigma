"use client";

import { Loader2, ShieldAlert } from "lucide-react";
import { ActivityTimeline, InspectorSection, MetaGrid, MetaItem, type TimelineEntry } from "@/components/garda/inspector";
import { StatusBadge } from "@/components/garda/status-badge";
import { KomoditasIcon } from "@/components/garda/komoditas-icon";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import {
  deskripsiRiwayat,
  persetujuanDiDepan,
  ringkasRantai,
  type Alur,
} from "@/lib/api/intervensi";
import { namaWilayah } from "@/lib/data/generators";
import { komoditasByKode } from "@/lib/data/reference";
import type { Tahap } from "@/lib/data/siklus";
import type { KartuIntervensi } from "@/lib/data/types";
import { siklusMeta } from "@/lib/design/tokens";
import { formatRelatif, formatTanggal } from "@/lib/format";
import { labelKode } from "@/lib/kode";
import { useRiwayatKartu } from "@/lib/query/hooks";

/** Linimasa riwayat kartu dari GET /v1/intervensi/{id}/riwayat (pemindahan, persetujuan, penugasan, eskalasi). */
function Riwayat({
  id,
  labelTahap,
}: {
  id: string;
  labelTahap: (kode?: string) => string;
}) {
  const { data, isLoading, isError, error } = useRiwayatKartu(id);
  if (isLoading)
    return (
      <Loader2
        className="mx-auto my-4 size-5 animate-spin text-muted-foreground"
        aria-label="Memuat riwayat"
      />
    );
  if (isError)
    return (
      <p
        role="alert"
        className="rounded-lg bg-defisit-soft/60 px-3 py-2 text-xs text-defisit-ink dark:text-rose-300"
      >
        Riwayat belum dapat dimuat
        {error instanceof Error ? `: ${error.message}` : ""}.
      </p>
    );
  if (!data || data.length === 0)
    return (
      <p className="text-xs text-muted-foreground">
        Belum ada peristiwa tercatat.
      </p>
    );
  const entries: TimelineEntry[] = data.map((r) => ({
    id: r.id,
    title: deskripsiRiwayat(r, labelTahap),
    meta: (
      <>
        {r.olehNama ?? "Sistem"} ·{" "}
        <time dateTime={r.waktu} title={formatTanggal(r.waktu, "d MMM yyyy HH:mm")}>
          {formatRelatif(r.waktu)}
        </time>
      </>
    ),
    note: r.alasan || undefined,
  }));
  return <ActivityTimeline entries={entries} label="Riwayat kartu" />;
}

/**
 * Lembar detail kartu: ringkasan, persetujuan yang akan dilalui (dari alur persetujuan), dan riwayat.
 * `kartu` null = tertutup.
 */
export function KartuDetailSheet({
  kartu,
  tahapSiklus,
  semuaTahap,
  alur,
  onClose,
}: {
  kartu: KartuIntervensi | null;
  /** Tahap siklus kartu, terurut. */
  tahapSiklus: Tahap[];
  semuaTahap: Tahap[];
  alur: Alur[] | undefined;
  onClose: () => void;
}) {
  const labelTahap = (kode?: string) =>
    semuaTahap.find((t) => t.id === kode)?.judul ?? kode ?? "";
  const depan =
    kartu && alur ? persetujuanDiDepan(alur, tahapSiklus, kartu.tahap) : [];
  const tone = kartu
    ? kartu.prioritas === "TINGGI"
      ? "defisit"
      : kartu.prioritas === "SEDANG"
        ? "waspada"
        : "netral"
    : "netral";

  return (
    <Sheet open={kartu !== null} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="w-full gap-0 overflow-y-auto sm:max-w-[480px]" data-siklus={kartu?.siklus}>
        <SheetHeader className="gap-1 border-b border-border p-5 pr-12">
          <SheetTitle className="text-base">{kartu?.judul ?? "Detail intervensi"}</SheetTitle>
          <SheetDescription>
            {kartu
              ? `${labelKode(kartu)} · ${siklusMeta[kartu.siklus].label} · ${labelTahap(kartu.tahap)}`
              : "Rincian dan riwayat kartu."}
          </SheetDescription>
        </SheetHeader>
        {kartu && (
          <div className="text-[13px]">
            <InspectorSection title="Rincian">
              <div className="flex flex-wrap gap-2">
                <StatusBadge tone={tone}>Prioritas {kartu.prioritas.toLowerCase()}</StatusBadge>
                {kartu.dieskalasi && (
                  <span className="inline-flex h-[22px] items-center gap-1 rounded-md bg-defisit-soft px-2 text-xs font-medium text-defisit-ink dark:bg-defisit/15 dark:text-rose-300">
                    <ShieldAlert className="size-3" /> Dieskalasi ke Dewan Pengarah
                  </span>
                )}
                {kartu.selesaiPada && <StatusBadge tone="normal">Selesai {formatTanggal(kartu.selesaiPada)}</StatusBadge>}
              </div>

              {kartu.deskripsi && <p className="text-muted-foreground">{kartu.deskripsi}</p>}

              <MetaGrid>
                <MetaItem label="Komoditas">
                  <span className="inline-flex items-center gap-1.5">
                    <KomoditasIcon kode={kartu.komoditasKode} className="size-3.5" />
                    {komoditasByKode[kartu.komoditasKode]?.nama ?? kartu.komoditasKode}
                  </span>
                </MetaItem>
                <MetaItem label="Wilayah">{namaWilayah(kartu.kabupatenKode)}</MetaItem>
                <MetaItem label="Tenggat">{formatTanggal(kartu.tenggat)}</MetaItem>
                <MetaItem label="Penanggung jawab">
                  {kartu.penanggungJawab.length ? kartu.penanggungJawab.map((p) => p.nama).join(", ") : "Belum ada"}
                </MetaItem>
                {kartu.ewsAlertId && <MetaItem label="Alert EWS">{kartu.ewsAlertKode ?? kartu.ewsAlertId}</MetaItem>}
                {kartu.dibuatPada && <MetaItem label="Dibuat">{formatTanggal(kartu.dibuatPada)}</MetaItem>}
              </MetaGrid>
            </InspectorSection>

            <InspectorSection title="Progres checklist">
              <div className="flex items-center gap-3">
                <div
                  role="progressbar"
                  aria-label="Progres checklist"
                  aria-valuemin={0}
                  aria-valuemax={kartu.progres.total}
                  aria-valuenow={kartu.progres.selesai}
                  className="h-1 flex-1 overflow-hidden rounded-full bg-secondary"
                >
                  <div
                    className="h-full rounded-full bg-data-600 transition-[width] duration-500"
                    style={{ width: `${kartu.progres.total ? (kartu.progres.selesai / kartu.progres.total) * 100 : 0}%` }}
                  />
                </div>
                <span className="num text-xs text-muted-foreground">{kartu.progres.selesai}/{kartu.progres.total}</span>
              </div>
            </InspectorSection>

            {depan.length > 0 && (
              <InspectorSection title="Persetujuan yang akan dilalui">
                <ul className="space-y-2" aria-label="Persetujuan di depan">
                  {depan.map(({ tahap, alur: a }) => (
                    <li key={a.kode} className="rounded-md border border-border bg-secondary/50 px-3 py-2 text-xs">
                      <p className="font-medium">{tahap.judul}</p>
                      <p className="text-muted-foreground">{ringkasRantai(a)}</p>
                      <p className="mt-0.5 text-muted-foreground">Izin: {a.langkah.map((l) => l.izin).join(", ")}</p>
                    </li>
                  ))}
                </ul>
              </InspectorSection>
            )}

            <InspectorSection title="Riwayat">
              <Riwayat id={kartu.id} labelTahap={labelTahap} />
            </InspectorSection>
          </div>
        )}
      </SheetContent>
    </Sheet>
  );
}
