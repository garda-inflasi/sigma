"use client";

import { Loader2, Play } from "lucide-react";
import { useEffect, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { EmptyState } from "@/components/garda/empty-state";
import {
  InspectorRingkasan,
  InspectorSection,
  MetaGrid,
  MetaItem,
} from "@/components/garda/inspector";
import { Shimmer } from "@/components/garda/shimmer";
import { SpringButton } from "@/components/garda/spring-button";
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
import type {
  EksekusiSumber,
  StatusEksekusi,
  SumberData,
} from "@/lib/data/types";
import type { Tone } from "@/lib/design/tokens";
import { useCan } from "@/lib/auth/session-context";
import { formatAngka, formatPersen, formatRelatif } from "@/lib/format";
import {
  useSumberDetail,
  useSumberEksekusi,
  useSumberJalankan,
  useSumberSampel,
} from "@/lib/query/hooks";
import { qk } from "@/lib/query/keys";
import { STATUS_SUMBER, langkahSumber } from "./status";

const STATUS_EKSEKUSI: Record<StatusEksekusi, { tone: Tone; label: string }> = {
  BERJALAN: { tone: "data", label: "Berjalan" },
  SUKSES: { tone: "normal", label: "Sukses" },
  SEBAGIAN: { tone: "waspada", label: "Sebagian" },
  GAGAL: { tone: "defisit", label: "Gagal" },
};

const PEMICU: Record<EksekusiSumber["pemicu"], string> = {
  JADWAL: "Jadwal",
  MANUAL: "Manual",
  CLI: "CLI",
  DEMO: "Contoh",
};

function BarisEksekusi({ e }: { e: EksekusiSumber }) {
  const s = STATUS_EKSEKUSI[e.status];
  return (
    <li className="space-y-1.5 rounded-lg border border-border bg-card p-3 text-[13px]">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <StatusBadge tone={s.tone}>{s.label}</StatusBadge>
        <span className="text-xs text-muted-foreground">
          {PEMICU[e.pemicu]}, {formatRelatif(e.mulai)}
        </span>
      </div>
      <p className="num text-xs text-muted-foreground">
        {formatAngka(e.barisDiambil)} baris masuk
        {e.barisDitolak > 0 && `, ${formatAngka(e.barisDitolak)} ditolak`}
        {e.durasiMs !== null && `, ${formatAngka(e.durasiMs / 1000, 1)} dtk`}
      </p>
      {e.galat && (
        <p className="text-xs break-words text-defisit-ink">{e.galat}</p>
      )}
      {e.takTerpetakan.length > 0 && (
        <details className="text-xs">
          <summary className="cursor-pointer text-muted-foreground">
            Alias belum dipetakan ({e.takTerpetakan.length})
          </summary>
          <ul className="mt-1 space-y-0.5">
            {e.takTerpetakan.map((t) => (
              <li key={t.alias} className="flex justify-between gap-2">
                <span className="break-all">{t.alias}</span>
                <span className="num text-muted-foreground">
                  {formatAngka(t.baris)}
                </span>
              </li>
            ))}
          </ul>
        </details>
      )}
      {e.tidakValid.length > 0 && (
        <ul className="space-y-0.5 text-xs text-muted-foreground">
          {e.tidakValid.map((t) => (
            <li key={t.alasan} className="flex justify-between gap-2">
              <span>{t.alasan}</span>
              <span className="num">{formatAngka(t.baris)}</span>
            </li>
          ))}
        </ul>
      )}
    </li>
  );
}

/** Panel samping: riwayat eksekusi, galat, sampel data mentah, dan tombol jalankan sekarang. */
export function SumberDetail({
  sumber,
  onClose,
}: {
  sumber: SumberData | null;
  onClose: () => void;
}) {
  const id = sumber?.id ?? null;
  // Data segar dari GET /v1/sumber-data/{id}; baris daftar dipakai selama memuat atau bila gagal.
  const { data: segar, isError: detailGagal } = useSumberDetail(id);
  const info = segar ?? sumber;
  const bolehKelola = useCan("sumber.kelola");
  const { data: riwayat, isLoading } = useSumberEksekusi(id);
  const { data: sampel } = useSumberSampel(id);
  const jalankan = useSumberJalankan();
  const qc = useQueryClient();

  // Saat run berjalan selesai, katalog (status, sukses 7 hari) dan sampel ikut disegarkan.
  const berjalan = riwayat?.some((e) => e.status === "BERJALAN") ?? false;
  const galatTerakhir = riwayat?.find((e) => e.status === "GAGAL")?.galat;
  const sebelumnya = useRef(false);
  useEffect(() => {
    if (sebelumnya.current && !berjalan && id) {
      qc.invalidateQueries({ queryKey: qk.sumber.list() });
      qc.invalidateQueries({ queryKey: qk.sumber.sampel(id) });
    }
    sebelumnya.current = berjalan;
  }, [berjalan, id, qc]);

  return (
    <Sheet open={!!sumber} onOpenChange={(o: boolean) => !o && onClose()}>
      <SheetContent className="sm:max-w-lg">
        {info && (
          <>
            <SheetHeader>
              <SheetTitle>{info.nama}</SheetTitle>
              <SheetDescription>
                {info.penyedia}, {info.jadwal}
              </SheetDescription>
            </SheetHeader>
            <SheetBody>
              {detailGagal && (
                <p className="border-b border-border bg-secondary px-5 py-2 text-xs text-muted-foreground">
                  Data terbaru gagal dimuat; menampilkan data dari daftar.
                </p>
              )}

              <InspectorRingkasan
                className="border-b-0"
                status={
                  <StatusBadge tone={STATUS_SUMBER[info.status].tone}>
                    {STATUS_SUMBER[info.status].label}
                  </StatusBadge>
                }
                nilai={
                  info.keberhasilan7h === null
                    ? "Belum ada eksekusi"
                    : `${formatPersen(info.keberhasilan7h)} sukses`
                }
                keterangan={
                  <>
                    Langkah berikutnya: {langkahSumber(info, bolehKelola)}.
                    {info.status === "GAGAL" && galatTerakhir && (
                      <span className="mt-1 block text-xs break-words text-defisit-ink">
                        Galat terakhir: {galatTerakhir}
                      </span>
                    )}
                  </>
                }
              />

              {segar && (
                <InspectorSection title="Informasi sumber">
                  <MetaGrid>
                    <MetaItem label="Kode">
                      <code className="num text-xs">{segar.kode}</code>
                    </MetaItem>
                    <MetaItem label="Jenis">
                      {segar.jenis === "SCRAPER" ? "Scraper" : "API"}
                    </MetaItem>
                    <MetaItem label="Cakupan">{segar.cakupan}</MetaItem>
                    <MetaItem label="Jadwal cron (WIB)">
                      <code className="num text-xs">{segar.jadwalCron}</code>
                    </MetaItem>
                    <MetaItem label="Terakhir dijalankan">
                      {segar.terakhir
                        ? formatRelatif(segar.terakhir)
                        : "Belum pernah"}
                    </MetaItem>
                    <MetaItem label="Keberhasilan 7 hari">
                      <span className="num">
                        {segar.keberhasilan7h === null
                          ? "Belum ada data"
                          : `${formatAngka(segar.keberhasilan7h, 1)}%`}
                      </span>
                    </MetaItem>
                    <MetaItem label="Latensi">
                      <span className="num">
                        {formatAngka(segar.latensiMs)} ms
                      </span>
                    </MetaItem>
                    <MetaItem label="Kepatuhan robots.txt">
                      {segar.robotsTxtPatuh ? "Patuh" : "Tidak patuh"}
                    </MetaItem>
                    <MetaItem label="URL dasar" wide>
                      {segar.urlDasar ? (
                        <span className="break-all">{segar.urlDasar}</span>
                      ) : (
                        "Tidak ada"
                      )}
                    </MetaItem>
                    <MetaItem label="Atribusi" wide>
                      {segar.atribusi}
                    </MetaItem>
                  </MetaGrid>
                </InspectorSection>
              )}

              <InspectorSection title="Riwayat eksekusi">
                {isLoading && (
                  <div
                    role="status"
                    aria-label="Memuat riwayat"
                    className="space-y-2"
                  >
                    <Shimmer className="h-16 w-full" />
                    <Shimmer className="h-16 w-full" />
                  </div>
                )}
                {riwayat && riwayat.length === 0 && (
                  <EmptyState
                    title="Belum pernah dijalankan."
                    className="py-6"
                  />
                )}
                {riwayat && riwayat.length > 0 && (
                  <ul className="space-y-2">
                    {riwayat.map((e) => (
                      <BarisEksekusi key={e.id} e={e} />
                    ))}
                  </ul>
                )}
              </InspectorSection>

              {sampel && sampel.length > 0 && (
                <InspectorSection title="Sampel data mentah (bronze)">
                  <div className="space-y-2">
                    {sampel.map((s) => (
                      <details
                        key={s.id}
                        className="rounded-lg border border-border bg-card p-3 text-xs"
                      >
                        <summary className="cursor-pointer break-words">
                          {s.kunci}, {formatAngka(s.ukuran)} karakter,{" "}
                          {formatRelatif(s.diambilPada)}
                        </summary>
                        <pre className="mt-2 max-h-48 overflow-auto text-[11px] break-all whitespace-pre-wrap text-muted-foreground">
                          {s.cuplikan}
                        </pre>
                      </details>
                    ))}
                  </div>
                </InspectorSection>
              )}
            </SheetBody>

            {bolehKelola && (
              <SheetFooter className="flex-col sm:flex-col sm:flex-nowrap">
                <SpringButton
                  variant="primary"
                  onClick={() => jalankan.mutate(info.id)}
                  disabled={
                    !info.bisaDijalankan ||
                    info.status === "DIJEDA" ||
                    berjalan ||
                    jalankan.isPending
                  }
                  className="w-full"
                >
                  {berjalan || jalankan.isPending ? (
                    <Loader2 className="animate-spin" />
                  ) : (
                    <Play />
                  )}
                  Jalankan sekarang
                </SpringButton>
                {!info.bisaDijalankan && (
                  <p className="text-xs text-muted-foreground">
                    Sumber ini belum memiliki konektor ingestion.
                  </p>
                )}
                {info.status === "DIJEDA" && (
                  <p className="text-xs text-muted-foreground">
                    Sumber dijeda; lanjutkan lebih dulu.
                  </p>
                )}
              </SheetFooter>
            )}
          </>
        )}
      </SheetContent>
    </Sheet>
  );
}
