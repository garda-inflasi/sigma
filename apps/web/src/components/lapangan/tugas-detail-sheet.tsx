"use client";

import { CheckCircle2, Circle, MapPin, Play } from "lucide-react";
import {
  InspectorRingkasan,
  InspectorSection,
  MetaGrid,
  MetaItem,
} from "@/components/garda/inspector";
import { KomoditasIcon } from "@/components/garda/komoditas-icon";
import { PanelGalat } from "@/components/garda/panel-galat";
import { Shimmer } from "@/components/garda/shimmer";
import { SpringLink } from "@/components/garda/spring-button";
import { StatusBadge } from "@/components/garda/status-badge";
import {
  Sheet,
  SheetBody,
  SheetContent,
  SheetDescription,
  SheetFooter,
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
  const { data: detail, isLoading, isError, error } = useTugasDetail(id);
  const t = detail ?? daftar;
  const rincian: RincianKomoditasTugas[] | undefined = detail?.rincian;
  const terisi = t ? Math.min(t.terisi, t.target) : 0;
  const sisa = t ? t.target - terisi : 0;

  return (
    <Sheet open={!!id} onOpenChange={(o: boolean) => !o && onClose()}>
      <SheetContent side="bottom" className="mx-auto max-w-md pb-safe">
        <SheetHeader>
          <SheetTitle>{t?.judul ?? "Detail tugas"}</SheetTitle>
          <SheetDescription className="flex items-center gap-1">
            {t ? (
              <>
                <MapPin className="size-3.5 shrink-0" aria-hidden />
                {t.pasarNama}
              </>
            ) : (
              "Rincian tugas survei"
            )}
          </SheetDescription>
        </SheetHeader>
        {t ? (
          <>
            <SheetBody>
              <InspectorRingkasan
                status={
                  <StatusBadge tone={STATUS[t.status].tone}>
                    {STATUS[t.status].label}
                  </StatusBadge>
                }
                nilai={`${terisi}/${t.target} komoditas`}
                keterangan={
                  <span className="block space-y-1.5">
                    <span
                      role="progressbar"
                      aria-label="Progres tugas"
                      aria-valuemin={0}
                      aria-valuemax={t.target}
                      aria-valuenow={terisi}
                      className="block h-1.5 overflow-hidden rounded-full bg-secondary"
                    >
                      <span
                        className={cn(
                          "block h-full rounded-full",
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
                    </span>
                    <span className="block">
                      {sisa > 0
                        ? `${sisa} komoditas belum dicatat.`
                        : "Semua komoditas sudah dicatat."}
                    </span>
                  </span>
                }
              />

              <InspectorSection title="Rincian">
                <MetaGrid>
                  <MetaItem label="Tenggat">
                    <span
                      className={cn(
                        "num",
                        t.status === "TERLAMBAT" &&
                          "text-defisit-ink dark:text-rose-300",
                      )}
                    >
                      {formatTanggal(t.tenggat, "d MMM yyyy HH.mm")}
                    </span>
                    <span className="block text-xs font-normal text-muted-foreground">
                      {formatRelatif(t.tenggat)}
                    </span>
                  </MetaItem>
                  <MetaItem label="Jenis harga">
                    {t.tipe === "KONSUMEN" ? "Konsumen" : "Produsen"}
                  </MetaItem>
                </MetaGrid>
              </InspectorSection>

              <InspectorSection title="Komoditas">
                {isLoading && !rincian && (
                  <div role="status" aria-label="Memuat rincian">
                    <Shimmer className="h-3 w-32" />
                  </div>
                )}
                {isError && !rincian && (
                  <p className="text-xs text-muted-foreground">
                    Rincian terbaru belum bisa dimuat, mungkin karena offline.
                    Daftar komoditas tetap bisa dipakai.
                  </p>
                )}
                <ul aria-label="Komoditas tugas" className="space-y-1.5">
                  {[
                    ...(rincian ??
                      t.komoditas.map((kode) => ({
                        kode,
                        nama: komoditasByKode[kode]?.nama ?? kode,
                        terisi: undefined,
                      }))),
                  ]
                    .sort((a, b) => Number(!!a.terisi) - Number(!!b.terisi))
                    .map((r) => (
                      <li
                        key={r.kode}
                        className="flex min-h-11 items-center gap-3 rounded-lg border border-border bg-card px-3 py-2 text-[13px]"
                      >
                        <KomoditasIcon
                          kode={r.kode}
                          className="size-4 text-muted-foreground"
                        />
                        <span className="min-w-0 flex-1 truncate">
                          {r.nama}
                        </span>
                        {r.terisi === undefined ? null : r.terisi ? (
                          <span className="inline-flex shrink-0 items-center gap-1 text-xs text-normal-ink dark:text-emerald-300">
                            <CheckCircle2 className="size-4" aria-hidden />
                            Sudah diisi
                          </span>
                        ) : (
                          <span className="inline-flex shrink-0 items-center gap-1 text-xs text-muted-foreground">
                            <Circle className="size-4" aria-hidden />
                            Belum diisi
                          </span>
                        )}
                      </li>
                    ))}
                </ul>
              </InspectorSection>
            </SheetBody>

            {t.status !== "SELESAI" && (
              <SheetFooter>
                <SpringLink
                  href={hrefIsiTugas(t)}
                  size="xl"
                  className="w-full"
                >
                  <Play /> Mulai isi
                </SpringLink>
              </SheetFooter>
            )}
          </>
        ) : (
          <SheetBody>
            {isLoading && (
              <div
                role="status"
                aria-label="Memuat detail"
                className="space-y-3 px-5 py-4"
              >
                <Shimmer className="h-5 w-1/3" />
                <Shimmer className="h-16 w-full" />
                <Shimmer className="h-16 w-full" />
              </div>
            )}
            {isError && (
              <div className="px-5 py-4">
                <PanelGalat judul="Detail tugas gagal dimuat" error={error} />
              </div>
            )}
          </SheetBody>
        )}
      </SheetContent>
    </Sheet>
  );
}
