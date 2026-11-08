"use client";

import { Laptop, Loader2, LogOut, Smartphone } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { MODE_MOCK } from "@/lib/api/client";
import { jumlahSesiLain, rincianSesi, type Sesi } from "@/lib/api/sesi";
import { keluarDariSemuaPerangkat } from "@/lib/auth/logout-client";
import { formatRelatif, formatTanggal } from "@/lib/format";
import {
  useCabutSesi,
  useCabutSesiLain,
  useSesiAktif,
} from "@/lib/query/sesi-hooks";
import { cn } from "@/lib/utils";

const adalahPonsel = (s: Sesi) => /Android|iOS|iPadOS/.test(s.perangkat);

/**
 * Satu sesi. Sesi ini ditandai "Sesi Ini" dan tidak punya tombol cabut. Sesi lain dicabut lewat konfirmasi
 * cepat di tempat ("Konfirmasi cabut" / "Batal"); Esc membatalkan dan fokus kembali ke tombol Cabut.
 */
function KartuSesi({
  sesi,
  konfirmasi,
  mencabut,
  onMinta,
  onBatal,
  onKonfirmasi,
}: {
  sesi: Sesi;
  konfirmasi: boolean;
  mencabut: boolean;
  onMinta: () => void;
  onBatal: () => void;
  onKonfirmasi: () => void;
}) {
  const Ikon = adalahPonsel(sesi) ? Smartphone : Laptop;
  const pemicu = useRef<HTMLButtonElement>(null);
  const batal = useRef<HTMLButtonElement>(null);
  const pernahKonfirmasi = useRef(false);

  // Tombol yang diganti konfirmasi membuat fokus keyboard hilang: pindahkan ke "Batal", lalu kembalikan.
  useEffect(() => {
    if (konfirmasi) {
      pernahKonfirmasi.current = true;
      batal.current?.focus();
    } else if (pernahKonfirmasi.current) {
      pernahKonfirmasi.current = false;
      pemicu.current?.focus();
    }
  }, [konfirmasi]);

  return (
    <li
      className={cn(
        "flex flex-col gap-3 rounded-xl border p-3.5 sm:flex-row sm:items-start",
        sesi.saatIni
          ? "border-data-300 bg-data-50/60 dark:border-data-500/40 dark:bg-data-500/10"
          : "border-border bg-white/60 dark:bg-white/[0.04]",
      )}
      onKeyDown={(e) => {
        if (e.key === "Escape" && konfirmasi && !mencabut) onBatal();
      }}
    >
      <div className="flex min-w-0 flex-1 items-start gap-3">
        <span className="grid size-10 shrink-0 place-items-center rounded-xl bg-secondary text-muted-foreground">
          <Ikon className="size-5" aria-hidden />
        </span>
        <div className="min-w-0 flex-1 space-y-0.5">
          <p className="flex flex-wrap items-center gap-2 text-sm font-medium">
            <span title={sesi.userAgent || undefined}>{sesi.perangkat}</span>
            {sesi.saatIni && <StatusBadge tone="normal">Sesi Ini</StatusBadge>}
          </p>
          <p className="num text-xs text-muted-foreground">
            {rincianSesi(sesi)}
          </p>
          <p className="text-xs text-muted-foreground">
            Masuk{" "}
            <time dateTime={sesi.dibuatPada}>
              {formatTanggal(sesi.dibuatPada, "d MMM yyyy, HH:mm")}
            </time>
          </p>
          <p className="text-xs text-muted-foreground">
            Aktif terakhir{" "}
            <time dateTime={sesi.terakhirDipakai}>
              {formatRelatif(sesi.terakhirDipakai)}
            </time>
          </p>
        </div>
      </div>

      {!sesi.saatIni &&
        (konfirmasi ? (
          <div
            role="group"
            aria-label={`Konfirmasi pencabutan sesi ${sesi.perangkat}`}
            className="flex gap-2 sm:shrink-0"
          >
            <Button
              variant="destructive"
              size="sm"
              disabled={mencabut}
              onClick={onKonfirmasi}
              className="max-sm:h-11 max-sm:flex-1"
            >
              {mencabut && <Loader2 className="animate-spin" aria-hidden />}
              Konfirmasi cabut
            </Button>
            <Button
              ref={batal}
              variant="ghost"
              size="sm"
              disabled={mencabut}
              onClick={onBatal}
              className="max-sm:h-11 max-sm:flex-1"
            >
              Batal
            </Button>
          </div>
        ) : (
          <Button
            ref={pemicu}
            variant="outline"
            size="sm"
            onClick={onMinta}
            aria-label={`Cabut sesi ${sesi.perangkat}${sesi.ip ? `, IP ${sesi.ip}` : ""}`}
            className="max-sm:h-11 sm:shrink-0"
          >
            Cabut
          </Button>
        ))}
    </li>
  );
}

/**
 * Daftar sesi aktif beserta pencabutannya: satu sesi (konfirmasi cepat), semua perangkat lain, atau semua
 * perangkat termasuk ini. Sesi yang sedang dipakai ditandai dan tidak punya tombol Cabut.
 */
export function DaftarSesi() {
  const { data: sesi, isLoading, isError, error, refetch } = useSesiAktif();
  const cabut = useCabutSesi();
  const cabutLain = useCabutSesiLain();
  // Satu konfirmasi cepat pada satu waktu: meminta pencabutan sesi lain membatalkan yang sebelumnya.
  const [idKonfirmasi, setIdKonfirmasi] = useState<string | null>(null);
  const [dialogLain, setDialogLain] = useState(false);
  const [konfirmasiSemua, setKonfirmasiSemua] = useState(false);
  const [keluarSibuk, setKeluarSibuk] = useState(false);

  async function keluarSemua() {
    setKeluarSibuk(true);
    try {
      if (!(await keluarDariSemuaPerangkat())) {
        // Masih ada entri lapangan belum terkirim: dialog konfirmasi keluar yang mengambil alih.
        setKeluarSibuk(false);
        setKonfirmasiSemua(false);
      }
    } catch (err) {
      setKeluarSibuk(false);
      setKonfirmasiSemua(false);
      toast.error("Gagal keluar dari semua perangkat", {
        description: err instanceof Error ? err.message : "Coba lagi.",
      });
    }
  }

  if (MODE_MOCK) {
    return (
      <p className="rounded-xl bg-muted/50 p-3.5 text-sm text-muted-foreground">
        Sesi aktif tersedia setelah backend aktif (mode demo tidak memiliki sesi
        server).
      </p>
    );
  }

  const lain = sesi ? jumlahSesiLain(sesi) : 0;

  return (
    <div className="space-y-5">
      <div className="space-y-3">
        {isLoading && (
          <div
            role="status"
            aria-busy="true"
            aria-label="Memuat sesi aktif"
            className="space-y-2.5"
          >
            <Shimmer className="h-[104px] w-full rounded-xl" />
            <Shimmer className="h-[104px] w-full rounded-xl" />
          </div>
        )}

        {isError && (
          <div role="alert" className="space-y-2 text-sm">
            <p className="font-medium text-destructive">
              {error instanceof Error
                ? error.message
                : "Gagal memuat sesi aktif."}
            </p>
            <Button variant="outline" size="sm" onClick={() => void refetch()}>
              Coba lagi
            </Button>
          </div>
        )}

        {sesi && (
          <ul aria-label="Sesi aktif" className="space-y-2.5">
            {sesi.map((s) => (
              <KartuSesi
                key={s.id}
                sesi={s}
                konfirmasi={idKonfirmasi === s.id}
                mencabut={cabut.isPending && cabut.variables === s.id}
                onMinta={() => setIdKonfirmasi(s.id)}
                onBatal={() => setIdKonfirmasi(null)}
                onKonfirmasi={() =>
                  cabut.mutate(s.id, {
                    onSettled: () => setIdKonfirmasi(null),
                  })
                }
              />
            ))}
          </ul>
        )}

        {sesi?.length === 0 && (
          <p className="text-sm text-muted-foreground">
            Tidak ada sesi aktif yang tercatat.
          </p>
        )}

        {sesi && sesi.length > 0 && lain === 0 && (
          <p className="text-xs text-muted-foreground">
            Tidak ada perangkat lain yang sedang masuk.
          </p>
        )}
      </div>

      <div className="flex flex-col gap-2 sm:flex-row">
        <Button
          variant="outline"
          className="flex-1 max-sm:h-11"
          disabled={lain === 0 || cabutLain.isPending}
          onClick={() => setDialogLain(true)}
        >
          {cabutLain.isPending ? (
            <Loader2 className="animate-spin" aria-hidden />
          ) : (
            <LogOut aria-hidden />
          )}
          Keluarkan semua perangkat lain
        </Button>
        <Button
          variant="destructive"
          className="flex-1 max-sm:h-11"
          disabled={keluarSibuk}
          onClick={() => setKonfirmasiSemua(true)}
        >
          Keluar dari semua perangkat
        </Button>
      </div>

      <AlertDialog open={dialogLain} onOpenChange={setDialogLain}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Keluarkan semua perangkat lain?</AlertDialogTitle>
            <AlertDialogDescription>
              {lain.toLocaleString("id-ID")} perangkat lain akan diakhiri
              sesinya dan harus masuk lagi dengan kata sandi. Sesi di perangkat
              ini tetap aktif.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={cabutLain.isPending}>
              Batal
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={cabutLain.isPending}
              onClick={(e) => {
                e.preventDefault();
                cabutLain.mutate(undefined, {
                  onSettled: () => setDialogLain(false),
                });
              }}
            >
              {cabutLain.isPending && (
                <Loader2 className="animate-spin" aria-hidden />
              )}
              Ya, keluarkan
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={konfirmasiSemua} onOpenChange={setKonfirmasiSemua}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Keluar dari semua perangkat?</AlertDialogTitle>
            <AlertDialogDescription>
              Semua sesi diakhiri, termasuk perangkat ini. Anda harus masuk lagi
              dengan kata sandi di setiap perangkat.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={keluarSibuk}>Batal</AlertDialogCancel>
            <AlertDialogAction
              disabled={keluarSibuk}
              onClick={(e) => {
                e.preventDefault();
                void keluarSemua();
              }}
            >
              Ya, keluar semua
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
