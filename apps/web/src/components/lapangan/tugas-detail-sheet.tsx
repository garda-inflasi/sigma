"use client";

import { CheckCircle2, Circle, MapPin, Play } from "lucide-react";
import { KomoditasIcon } from "@/components/garda/komoditas-icon";
import { SpringLink } from "@/components/garda/spring-button";
import { StatusBadge } from "@/components/garda/status-badge";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { hrefIsiTugas, type RincianKomoditasTugas } from "@/lib/api/lapangan";
import { komoditasByKode } from "@/lib/data/reference";
import type { TugasLapangan } from "@/lib/data/types";
import { formatRelatif, formatTanggal } from "@/lib/format";
import { useTugasDetail } from "@/lib/query/hooks";
import { cn } from "@/lib/utils";

const STATUS = {
  BELUM: { label: "Belum mulai", tone: "netral" },
  BERJALAN: { label: "Berjalan", tone: "data" },
  SELESAI: { label: "Selesai", tone: "normal" },
  TERLAMBAT: { label: "Terlambat", tone: "defisit" },
} as const;

/**
 * Detail satu tugas survei sebagai lembar bawah (bukan rute baru) agar tetap berfungsi saat
 * offline: bila detail dari server tidak bisa dimuat, daftar komoditas diambil dari baris daftar
 * dan tautan "Mulai isi" tetap bekerja.
 */
export function TugasDetailSheet({
  id,
  daftar,
  onClose,
}: {
  id: string | null;
  /** Baris tugas dari daftar: sumber awal selama detail dimuat atau bila gagal dimuat. */
  daftar: TugasLapangan | undefined;
  onClose: () => void;
}) {
  const { data: detail, isLoading, isError } = useTugasDetail(id);
  const t = detail ?? daftar;
  const rincian: RincianKomoditasTugas[] | undefined = detail?.rincian;

  return (
    <Sheet open={!!id} onOpenChange={(o: boolean) => !o && onClose()}>
      <SheetContent
        side="bottom"
        className="glass-strong mx-auto max-h-[85dvh] max-w-md overflow-y-auto rounded-t-2xl border-0"
      >
        <SheetHeader>
          <SheetTitle>{t?.judul ?? "Detail tugas"}</SheetTitle>
          <SheetDescription className="flex items-center gap-1">
            <MapPin className="size-3.5" />
            {t
              ? `${t.pasarNama}, harga ${t.tipe === "KONSUMEN" ? "konsumen" : "produsen"}`
              : (id ?? "")}
          </SheetDescription>
        </SheetHeader>
        {t && (
          <div className="space-y-5 px-4 pb-6">
            <div className="flex items-center justify-between gap-3">
              <StatusBadge tone={STATUS[t.status].tone}>
                {STATUS[t.status].label}
              </StatusBadge>
              <p
                className={cn(
                  "text-right text-xs",
                  t.status === "TERLAMBAT"
                    ? "text-defisit-ink"
                    : "text-muted-foreground",
                )}
              >
                Tenggat {formatTanggal(t.tenggat, "d MMM yyyy HH.mm")} (
                {formatRelatif(t.tenggat)})
              </p>
            </div>

            <div className="space-y-1.5">
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted-foreground">Progres</span>
                <span className="num font-medium">
                  {Math.min(t.terisi, t.target)}/{t.target} komoditas
                </span>
              </div>
              <div
                role="progressbar"
                aria-label="Progres tugas"
                aria-valuemin={0}
                aria-valuemax={t.target}
                aria-valuenow={Math.min(t.terisi, t.target)}
                className="h-1.5 overflow-hidden rounded-full bg-mist-200 dark:bg-white/10"
              >
                <div
                  className={cn(
                    "h-full rounded-full",
                    t.status === "SELESAI"
                      ? "bg-normal"
                      : t.status === "TERLAMBAT"
                        ? "bg-defisit"
                        : "bg-data-500",
                  )}
                  style={{
                    width: `${Math.min(100, (t.terisi / t.target) * 100)}%`,
                  }}
                />
              </div>
            </div>

            <section className="space-y-2">
              <h3 className="text-sm font-medium">Komoditas</h3>
              {isLoading && !rincian && (
                <p className="text-sm text-muted-foreground">Memuat rincian…</p>
              )}
              {isError && !rincian && (
                <p className="text-xs text-muted-foreground">
                  Rincian terbaru tidak dapat dimuat (mungkin sedang offline);
                  menampilkan daftar komoditas saja.
                </p>
              )}
              <ul aria-label="Komoditas tugas" className="space-y-1.5">
                {(
                  rincian ??
                  t.komoditas.map((kode) => ({
                    kode,
                    nama: komoditasByKode[kode]?.nama ?? kode,
                    terisi: undefined,
                  }))
                ).map((r) => (
                  <li
                    key={r.kode}
                    className="flex items-center gap-3 rounded-xl bg-white/50 p-3 text-sm dark:bg-white/5"
                  >
                    <KomoditasIcon
                      kode={r.kode}
                      className="size-4 text-muted-foreground"
                    />
                    <span className="min-w-0 flex-1 truncate">{r.nama}</span>
                    {r.terisi === undefined ? null : r.terisi ? (
                      <span className="inline-flex items-center gap-1 text-xs text-normal-ink">
                        <CheckCircle2 className="size-4" aria-hidden />
                        Sudah diisi
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
                        <Circle className="size-4" aria-hidden />
                        Belum diisi
                      </span>
                    )}
                  </li>
                ))}
              </ul>
            </section>

            {t.status !== "SELESAI" && (
              <SpringLink href={hrefIsiTugas(t)} className="w-full">
                <Play /> Mulai isi
              </SpringLink>
            )}
          </div>
        )}
      </SheetContent>
    </Sheet>
  );
}
