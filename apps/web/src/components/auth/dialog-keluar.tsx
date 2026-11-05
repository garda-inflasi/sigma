"use client";

import { Loader2 } from "lucide-react";
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
import { useDialogKeluar } from "@/lib/auth/dialog-keluar-store";
import { pesanHasilKirim } from "@/lib/auth/konfirmasi-keluar";
import { keluarSekarang, keluarSemuaSekarang } from "@/lib/auth/logout-client";
import { kirimSebelumKeluar } from "@/lib/offline/kirim-dulu-klien";
import { useOnline } from "@/lib/offline/use-online";

type Fase = "tanya" | "mengirim" | "keluar";

/**
 * Konfirmasi keluar saat masih ada entri lapangan yang belum terkirim. Entri itu hanya ada di perangkat ini
 * dan ikut terhapus bersama data offline, jadi pengguna memilih: "Kirim dulu" (sinkronkan sekarang bila
 * online, lalu keluar bila semuanya terkirim), "Tetap keluar" (buang entri), atau "Batal".
 * Dirender sekali di Providers; dibuka oleh `keluarDariSesi()` dan `keluarDariSemuaPerangkat()`.
 */
export function DialogKeluar() {
  const terbuka = useDialogKeluar((s) => s.terbuka);
  const sisa = useDialogKeluar((s) => s.sisa);
  const cakupan = useDialogKeluar((s) => s.cakupan);
  const online = useOnline();
  const [fase, setFase] = useState<Fase>("tanya");
  const [pesan, setPesan] = useState<string | null>(null);
  const sibuk = fase !== "tanya";

  async function keluarKarena() {
    setFase("keluar");
    if (cakupan === "sesi") return keluarSekarang();
    try {
      await keluarSemuaSekarang();
    } catch (err) {
      setFase("tanya");
      toast.error("Gagal keluar dari semua perangkat", {
        description: err instanceof Error ? err.message : "Coba lagi.",
      });
    }
  }

  async function kirimDulu() {
    setFase("mengirim");
    setPesan(null);
    const hasil = await kirimSebelumKeluar();
    if (hasil.jenis === "selesai") return keluarKarena();
    if (hasil.jenis !== "offline")
      useDialogKeluar.getState().setSisa(hasil.sisa);
    setPesan(pesanHasilKirim(hasil));
    setFase("tanya");
  }

  function tutup() {
    if (sibuk) return;
    setPesan(null);
    useDialogKeluar.getState().tutup();
  }

  return (
    <AlertDialog open={terbuka} onOpenChange={(o: boolean) => !o && tutup()}>
      <AlertDialogContent className="rounded-xl">
        <AlertDialogHeader>
          <AlertDialogTitle>Ada entri yang belum terkirim</AlertDialogTitle>
          <AlertDialogDescription>
            <strong className="num font-semibold text-foreground">
              {sisa} entri harga
            </strong>{" "}
            belum terkirim ke server dan hanya tersimpan di perangkat ini. Jika
            Anda tetap keluar, entri tersebut akan dihapus dan tidak dapat
            dipulihkan.
          </AlertDialogDescription>
        </AlertDialogHeader>
        {!online && fase === "tanya" && !pesan && (
          <p role="status" className="text-sm text-muted-foreground">
            Perangkat sedang offline, entri belum bisa dikirim sekarang.
          </p>
        )}
        {pesan && (
          <p role="status" className="text-sm text-waspada-ink">
            {pesan}
          </p>
        )}
        <AlertDialogFooter>
          <AlertDialogCancel className="rounded-xl" disabled={sibuk}>
            Batal
          </AlertDialogCancel>
          <Button
            variant="outline"
            className="rounded-xl"
            disabled={sibuk || !online}
            onClick={() => void kirimDulu()}
          >
            {fase === "mengirim" ? (
              <Loader2 className="size-4 animate-spin" />
            ) : null}
            Kirim dulu
          </Button>
          <AlertDialogAction
            variant="destructive"
            className="rounded-xl"
            disabled={sibuk}
            onClick={(e: React.MouseEvent) => {
              e.preventDefault();
              void keluarKarena();
            }}
          >
            {fase === "keluar" ? (
              <Loader2 className="size-4 animate-spin" />
            ) : null}
            Tetap keluar
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
