"use client";

import {
  ArrowLeft,
  KeyRound,
  Laptop,
  Loader2,
  LogOut,
  ShieldCheck,
  Smartphone,
} from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { toast } from "sonner";
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

function BarisSesi({
  sesi,
  mencabut,
  onCabut,
}: {
  sesi: Sesi;
  mencabut: boolean;
  onCabut: () => void;
}) {
  const Ikon = adalahPonsel(sesi) ? Smartphone : Laptop;
  return (
    <li
      className={cn(
        "flex items-start gap-3 rounded-xl border p-3.5",
        sesi.saatIni
          ? "border-data-300 bg-data-50/60 dark:border-data-500/40 dark:bg-data-500/10"
          : "border-border bg-white/60 dark:bg-white/[0.04]",
      )}
    >
      <Ikon
        className="mt-0.5 size-5 shrink-0 text-muted-foreground"
        aria-hidden
      />
      <div className="min-w-0 flex-1">
        <p className="flex flex-wrap items-center gap-2 text-sm font-medium">
          <span>{sesi.perangkat}</span>
          {sesi.saatIni && (
            <span className="rounded-full bg-normal-soft px-2 py-0.5 text-[11px] font-semibold text-normal-ink">
              Perangkat ini
            </span>
          )}
        </p>
        <p className="num mt-0.5 text-xs text-muted-foreground">
          {rincianSesi(sesi)}
        </p>
        <p className="mt-0.5 text-xs text-muted-foreground">
          Masuk {formatTanggal(sesi.dibuatPada, "d MMM yyyy, HH:mm")} · aktif{" "}
          {formatRelatif(sesi.terakhirDipakai)}
        </p>
      </div>
      {!sesi.saatIni && (
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={mencabut}
          onClick={onCabut}
          aria-label={`Cabut sesi ${sesi.perangkat}${sesi.ip ? `, IP ${sesi.ip}` : ""}`}
        >
          {mencabut ? <Loader2 className="animate-spin" /> : null}
          Cabut
        </Button>
      )}
    </li>
  );
}

/** Daftar sesi aktif beserta aksi pencabutan; sesi yang sedang dipakai ditandai dan tidak punya tombol Cabut. */
export function KeamananPanel() {
  const { data: sesi, isLoading, isError, error, refetch } = useSesiAktif();
  const cabut = useCabutSesi();
  const cabutLain = useCabutSesiLain();
  const [konfirmasiSemua, setKonfirmasiSemua] = useState(false);
  const [keluarSibuk, setKeluarSibuk] = useState(false);

  async function keluarSemua() {
    setKeluarSibuk(true);
    try {
      await keluarDariSemuaPerangkat();
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
      <p className="mt-6 rounded-xl bg-muted/50 p-3.5 text-sm text-muted-foreground">
        Sesi aktif tersedia setelah backend aktif (mode demo tidak memiliki sesi
        server).
      </p>
    );
  }

  const lain = sesi ? jumlahSesiLain(sesi) : 0;

  return (
    <div className="mt-6 space-y-5">
      <section aria-labelledby="judul-sesi" className="space-y-3">
        <h2 id="judul-sesi" className="text-sm font-semibold">
          Sesi aktif
        </h2>

        {isLoading && (
          <div
            className="glass h-24 animate-pulse rounded-xl"
            aria-busy="true"
            aria-label="Memuat sesi aktif"
          />
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
              <BarisSesi
                key={s.id}
                sesi={s}
                mencabut={cabut.isPending && cabut.variables === s.id}
                onCabut={() => cabut.mutate(s.id)}
              />
            ))}
          </ul>
        )}

        {sesi && lain === 0 && (
          <p className="text-xs text-muted-foreground">
            Tidak ada perangkat lain yang sedang masuk.
          </p>
        )}
      </section>

      <div className="flex flex-col gap-2 sm:flex-row">
        <Button
          type="button"
          variant="outline"
          className="flex-1"
          disabled={lain === 0 || cabutLain.isPending}
          onClick={() => cabutLain.mutate()}
        >
          {cabutLain.isPending ? (
            <Loader2 className="animate-spin" />
          ) : (
            <LogOut />
          )}
          Keluar dari perangkat lain
        </Button>
        <Button
          type="button"
          variant="destructive"
          className="flex-1"
          disabled={keluarSibuk}
          onClick={() => setKonfirmasiSemua(true)}
        >
          Keluar dari semua perangkat
        </Button>
      </div>

      <nav
        aria-label="Pengaturan keamanan lain"
        className="flex flex-wrap gap-x-5 gap-y-2 border-t border-border pt-4 text-sm"
      >
        <Link
          href="/ganti-password"
          className="inline-flex items-center gap-1.5 text-navy-800 hover:underline dark:text-data-300"
        >
          <KeyRound className="size-4" /> Ganti kata sandi
        </Link>
        <Link
          href="/mfa"
          className="inline-flex items-center gap-1.5 text-navy-800 hover:underline dark:text-data-300"
        >
          <ShieldCheck className="size-4" /> Verifikasi dua langkah
        </Link>
        <Link
          href="/"
          className="inline-flex items-center gap-1.5 text-muted-foreground hover:underline"
        >
          <ArrowLeft className="size-4" /> Kembali ke beranda
        </Link>
      </nav>

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
