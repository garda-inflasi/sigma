"use client";

import { Loader2, ShieldAlert } from "lucide-react";
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
import { useRiwayatKartu } from "@/lib/query/hooks";

function Baris({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex items-start justify-between gap-4 py-2">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="text-right font-medium">{children}</dd>
    </div>
  );
}

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
  return (
    <ol
      className="relative space-y-4 border-l border-border/70 pl-4"
      aria-label="Riwayat kartu"
    >
      {data.map((r) => (
        <li key={r.id} className="relative">
          <span className="absolute top-1.5 -left-[21px] size-2.5 rounded-full bg-siklus ring-2 ring-background" />
          <p className="text-sm font-medium">
            {deskripsiRiwayat(r, labelTahap)}
          </p>
          <p className="text-xs text-muted-foreground">
            {r.olehNama ?? "Sistem"} ·{" "}
            <time
              dateTime={r.waktu}
              title={formatTanggal(r.waktu, "d MMM yyyy HH:mm")}
            >
              {formatRelatif(r.waktu)}
            </time>
          </p>
          {r.alasan && (
            <p className="mt-1 rounded-lg bg-secondary/60 px-2.5 py-1.5 text-xs">
              {r.alasan}
            </p>
          )}
        </li>
      ))}
    </ol>
  );
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
      <SheetContent
        className="glass-strong w-full gap-0 overflow-y-auto sm:max-w-lg"
        data-siklus={kartu?.siklus}
      >
        <SheetHeader>
          <SheetTitle>{kartu?.judul ?? "Detail intervensi"}</SheetTitle>
          <SheetDescription>
            {kartu
              ? `${kartu.id} · ${siklusMeta[kartu.siklus].label} · ${labelTahap(kartu.tahap)}`
              : "Rincian dan riwayat kartu."}
          </SheetDescription>
        </SheetHeader>
        {kartu && (
          <div className="space-y-6 px-4 pb-8 text-sm">
            <div className="flex flex-wrap gap-2">
              <StatusBadge tone={tone}>
                Prioritas {kartu.prioritas.toLowerCase()}
              </StatusBadge>
              {kartu.dieskalasi && (
                <span className="inline-flex items-center gap-1 rounded-full bg-defisit-soft px-2 py-0.5 text-[11px] font-medium text-defisit-ink dark:bg-defisit/15 dark:text-rose-300">
                  <ShieldAlert className="size-3" /> Dieskalasi ke Dewan
                  Pengarah
                </span>
              )}
              {kartu.selesaiPada && (
                <StatusBadge tone="normal">
                  Selesai {formatTanggal(kartu.selesaiPada)}
                </StatusBadge>
              )}
            </div>

            {kartu.deskripsi && (
              <p className="text-muted-foreground">{kartu.deskripsi}</p>
            )}

            <dl className="divide-y divide-border/60">
              <Baris label="Komoditas">
                <span className="inline-flex items-center gap-1.5">
                  <KomoditasIcon
                    kode={kartu.komoditasKode}
                    className="size-3.5"
                  />
                  {komoditasByKode[kartu.komoditasKode]?.nama ??
                    kartu.komoditasKode}
                </span>
              </Baris>
              <Baris label="Wilayah">{namaWilayah(kartu.kabupatenKode)}</Baris>
              <Baris label="Tenggat">{formatTanggal(kartu.tenggat)}</Baris>
              <Baris label="Progres checklist">
                {kartu.progres.selesai}/{kartu.progres.total}
              </Baris>
              <Baris label="Penanggung jawab">
                {kartu.penanggungJawab.length
                  ? kartu.penanggungJawab.map((p) => p.nama).join(", ")
                  : "Belum ada"}
              </Baris>
              {kartu.ewsAlertId && (
                <Baris label="Alert EWS">{kartu.ewsAlertId}</Baris>
              )}
              {kartu.dibuatPada && (
                <Baris label="Dibuat">{formatTanggal(kartu.dibuatPada)}</Baris>
              )}
            </dl>

            {depan.length > 0 && (
              <section aria-label="Persetujuan di depan">
                <h3 className="mb-2 font-sans text-sm font-semibold">
                  Persetujuan yang akan dilalui
                </h3>
                <ul className="space-y-2">
                  {depan.map(({ tahap, alur: a }) => (
                    <li
                      key={a.kode}
                      className="rounded-lg bg-secondary/50 px-3 py-2 text-xs"
                    >
                      <p className="font-medium">{tahap.judul}</p>
                      <p className="text-muted-foreground">
                        {ringkasRantai(a)}
                      </p>
                      <p className="mt-0.5 text-muted-foreground">
                        Izin: {a.langkah.map((l) => l.izin).join(", ")}
                      </p>
                    </li>
                  ))}
                </ul>
              </section>
            )}

            <section aria-label="Riwayat kartu">
              <h3 className="mb-3 font-sans text-sm font-semibold">Riwayat</h3>
              <Riwayat id={kartu.id} labelTahap={labelTahap} />
            </section>
          </div>
        )}
      </SheetContent>
    </Sheet>
  );
}
