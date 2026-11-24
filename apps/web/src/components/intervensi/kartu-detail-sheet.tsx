"use client";

import { ShieldAlert } from "lucide-react";
import { EmptyState } from "@/components/garda/empty-state";
import {
  ActivityTimeline,
  InspectorRingkasan,
  InspectorSection,
  MetaGrid,
  MetaItem,
  type TimelineEntry,
} from "@/components/garda/inspector";
import { PanelGalat } from "@/components/garda/panel-galat";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import { KomoditasIcon } from "@/components/garda/komoditas-icon";
import {
  Sheet,
  SheetBody,
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
import { labelTenggat } from "./tenggat";

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
      <div role="status" aria-label="Memuat riwayat" className="space-y-3">
        <Shimmer className="h-10 w-full" />
        <Shimmer className="h-10 w-full" />
      </div>
    );
  if (isError)
    return <PanelGalat judul="Riwayat belum dapat dimuat" error={error} />;
  if (!data || data.length === 0)
    return (
      <EmptyState title="Belum ada peristiwa tercatat." className="py-6" />
    );
  const entries: TimelineEntry[] = data.map((r) => ({
    id: r.id,
    title: deskripsiRiwayat(r, labelTahap),
    meta: (
      <>
        {r.olehNama ?? "Sistem"} ·{" "}
        <time
          dateTime={r.waktu}
          title={formatTanggal(r.waktu, "d MMM yyyy HH:mm")}
        >
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
      <SheetContent className="sm:max-w-lg" data-siklus={kartu?.siklus}>
        <SheetHeader>
          <SheetTitle>{kartu?.judul ?? "Detail intervensi"}</SheetTitle>
          <SheetDescription>
            {kartu
              ? `${labelKode(kartu)} · ${siklusMeta[kartu.siklus].label} · ${labelTahap(kartu.tahap)}`
              : "Rincian dan riwayat kartu."}
          </SheetDescription>
        </SheetHeader>
        {kartu && (
          <SheetBody className="text-[13px]">
            <InspectorRingkasan
              status={
                <>
                  <StatusBadge tone={tone}>
                    Prioritas {kartu.prioritas.toLowerCase()}
                  </StatusBadge>
                  {kartu.dieskalasi && (
                    <span className="inline-flex h-[22px] items-center gap-1 rounded-md bg-defisit-soft px-2 text-xs font-medium text-defisit-ink dark:bg-defisit/15 dark:text-rose-300">
                      <ShieldAlert className="size-3" /> Dieskalasi ke Dewan
                      Pengarah
                    </span>
                  )}
                  {kartu.selesaiPada && (
                    <StatusBadge tone="normal">
                      Selesai {formatTanggal(kartu.selesaiPada)}
                    </StatusBadge>
                  )}
                </>
              }
              nilai={labelTenggat(kartu).text}
              keterangan={`Tahap saat ini: ${labelTahap(kartu.tahap)}. Checklist ${kartu.progres.selesai} dari ${kartu.progres.total} selesai.`}
            />

            <InspectorSection title="Rincian" className="border-t-0">
              {kartu.deskripsi && (
                <p className="text-muted-foreground">{kartu.deskripsi}</p>
              )}

              <MetaGrid>
                <MetaItem label="Komoditas">
                  <span className="inline-flex items-center gap-1.5">
                    <KomoditasIcon
                      kode={kartu.komoditasKode}
                      className="size-3.5"
                    />
                    {komoditasByKode[kartu.komoditasKode]?.nama ??
                      kartu.komoditasKode}
                  </span>
                </MetaItem>
                <MetaItem label="Wilayah">
                  {namaWilayah(kartu.kabupatenKode)}
                </MetaItem>
                <MetaItem label="Tenggat">
                  <span className="num">{formatTanggal(kartu.tenggat)}</span>
                </MetaItem>
                <MetaItem label="Penanggung jawab">
                  {kartu.penanggungJawab.length
                    ? kartu.penanggungJawab.map((p) => p.nama).join(", ")
                    : "Belum ada"}
                </MetaItem>
                {kartu.ewsAlertId && (
                  <MetaItem label="Alert EWS">
                    <span className="num">
                      {kartu.ewsAlertKode ?? kartu.ewsAlertId}
                    </span>
                  </MetaItem>
                )}
                {kartu.dibuatPada && (
                  <MetaItem label="Dibuat">
                    <span className="num">
                      {formatTanggal(kartu.dibuatPada)}
                    </span>
                  </MetaItem>
                )}
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
                    style={{
                      width: `${kartu.progres.total ? (kartu.progres.selesai / kartu.progres.total) * 100 : 0}%`,
                    }}
                  />
                </div>
                <span className="num text-xs text-muted-foreground">
                  {kartu.progres.selesai}/{kartu.progres.total}
                </span>
              </div>
            </InspectorSection>

            {depan.length > 0 && (
              <InspectorSection title="Persetujuan yang akan dilalui">
                <ul className="space-y-2" aria-label="Persetujuan di depan">
                  {depan.map(({ tahap, alur: a }) => (
                    <li
                      key={a.kode}
                      className="rounded-md border border-border bg-secondary px-3 py-2 text-xs"
                    >
                      <p className="font-medium">{tahap.judul}</p>
                      <p
                        className="text-muted-foreground"
                        title={`Izin: ${a.langkah.map((l) => l.izin).join(", ")}`}
                      >
                        {ringkasRantai(a)}
                      </p>
                    </li>
                  ))}
                </ul>
              </InspectorSection>
            )}

            <InspectorSection title="Riwayat">
              <Riwayat id={kartu.id} labelTahap={labelTahap} />
            </InspectorSection>
          </SheetBody>
        )}
      </SheetContent>
    </Sheet>
  );
}
