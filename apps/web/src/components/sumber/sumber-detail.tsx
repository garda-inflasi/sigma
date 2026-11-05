"use client";

import { Loader2, Play } from "lucide-react";
import { useEffect, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { StatusBadge } from "@/components/garda/status-badge";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetDescription,
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
import { formatAngka, formatRelatif } from "@/lib/format";
import {
  useSumberEksekusi,
  useSumberJalankan,
  useSumberSampel,
} from "@/lib/query/hooks";
import { qk } from "@/lib/query/keys";

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
    <li className="space-y-1.5 rounded-xl bg-white/50 p-3 text-sm dark:bg-white/5">
      <div className="flex items-center justify-between gap-2">
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
        <p className="break-words text-xs text-defisit-ink">{e.galat}</p>
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
  const bolehKelola = useCan("sumber.kelola");
  const { data: riwayat, isLoading } = useSumberEksekusi(id);
  const { data: sampel } = useSumberSampel(id);
  const jalankan = useSumberJalankan();
  const qc = useQueryClient();

  // Saat run berjalan selesai, katalog (status, sukses 7 hari) dan sampel ikut disegarkan.
  const berjalan = riwayat?.some((e) => e.status === "BERJALAN") ?? false;
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
      <SheetContent className="glass-strong w-full overflow-y-auto border-0 sm:max-w-md">
        {sumber && (
          <>
            <SheetHeader>
              <SheetTitle>{sumber.nama}</SheetTitle>
              <SheetDescription>
                {sumber.penyedia}, {sumber.jadwal}
              </SheetDescription>
            </SheetHeader>
            <div className="space-y-5 px-4 pb-6">
              {bolehKelola && (
                <div className="space-y-1.5">
                  <Button
                    onClick={() => jalankan.mutate(sumber.id)}
                    disabled={
                      !sumber.bisaDijalankan ||
                      sumber.status === "DIJEDA" ||
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
                  </Button>
                  {!sumber.bisaDijalankan && (
                    <p className="text-xs text-muted-foreground">
                      Sumber ini belum memiliki konektor ingestion.
                    </p>
                  )}
                  {sumber.status === "DIJEDA" && (
                    <p className="text-xs text-muted-foreground">
                      Sumber dijeda; lanjutkan lebih dulu.
                    </p>
                  )}
                </div>
              )}

              <section className="space-y-2">
                <h3 className="text-sm font-medium">Riwayat eksekusi</h3>
                {isLoading && (
                  <p className="text-sm text-muted-foreground">Memuat…</p>
                )}
                {riwayat && riwayat.length === 0 && (
                  <p className="text-sm text-muted-foreground">
                    Belum pernah dijalankan.
                  </p>
                )}
                <ul className="space-y-2">
                  {riwayat?.map((e) => (
                    <BarisEksekusi key={e.id} e={e} />
                  ))}
                </ul>
              </section>

              {sampel && sampel.length > 0 && (
                <section className="space-y-2">
                  <h3 className="text-sm font-medium">
                    Sampel data mentah (bronze)
                  </h3>
                  {sampel.map((s) => (
                    <details
                      key={s.id}
                      className="rounded-xl bg-white/50 p-3 text-xs dark:bg-white/5"
                    >
                      <summary className="cursor-pointer">
                        {s.kunci}, {formatAngka(s.ukuran)} karakter,{" "}
                        {formatRelatif(s.diambilPada)}
                      </summary>
                      <pre className="mt-2 max-h-48 overflow-auto whitespace-pre-wrap break-all text-[11px] text-muted-foreground">
                        {s.cuplikan}
                      </pre>
                    </details>
                  ))}
                </section>
              )}
            </div>
          </>
        )}
      </SheetContent>
    </Sheet>
  );
}
