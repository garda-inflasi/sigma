"use client";

import { AlertTriangle } from "lucide-react";
import { useState } from "react";
import { SpringButton } from "@/components/garda/spring-button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  ENTITAS_LABEL,
  KODE_MASTER_ADA_USULAN,
  KODE_MASTER_DIRUJUK,
  kodeGalatMaster,
  pesanGalatMaster,
  type EntitasPerubahan,
} from "@/lib/api/master-perubahan";
import { useHapusMaster } from "@/lib/query/hooks";

/**
 * Konfirmasi hapus komoditas/pasar (izin master.setujui). Server menolak dengan 409 bila data masih dirujuk
 * (MASTER_DIRUJUK: sarankan arsip/nonaktifkan) atau masih ada usulan menunggu (MASTER_ADA_USULAN); pesannya
 * tampil di dialog yang tetap terbuka.
 */
export function HapusDialog({
  entitas,
  kode,
  nama,
  open,
  onOpenChange,
  onDihapus,
}: {
  entitas: EntitasPerubahan;
  kode: string;
  nama: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDihapus: () => void;
}) {
  const hapus = useHapusMaster();
  const [galat, setGalat] = useState<{ pesan: string; kode?: string } | null>(
    null,
  );
  const label = ENTITAS_LABEL[entitas].toLowerCase();
  const saran =
    entitas === "komoditas" ? "mengarsipkan komoditas" : "menonaktifkan pasar";
  const tertahan =
    galat?.kode === KODE_MASTER_DIRUJUK ||
    galat?.kode === KODE_MASTER_ADA_USULAN;

  function tutup(v: boolean) {
    if (!v) setGalat(null);
    onOpenChange(v);
  }

  async function kirim() {
    setGalat(null);
    try {
      await hapus.mutateAsync({ entitas, kode });
      tutup(false);
      onDihapus();
    } catch (err) {
      setGalat({ pesan: pesanGalatMaster(err), kode: kodeGalatMaster(err) });
    }
  }

  return (
    <Dialog open={open} onOpenChange={tutup}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            Hapus {label} {kode}?
          </DialogTitle>
          <DialogDescription>
            {nama} dihapus permanen dan tercatat di audit trail. Data yang sudah
            dipakai harga atau tugas lapangan tidak dapat dihapus; untuk itu
            pilih {saran}.
          </DialogDescription>
        </DialogHeader>
        {galat && (
          <div
            role="alert"
            className="flex gap-2 rounded-xl bg-waspada-soft/80 p-3 text-sm text-waspada-ink dark:bg-waspada/15 dark:text-amber-300"
          >
            <AlertTriangle className="mt-0.5 size-4 shrink-0" />
            <div className="space-y-1">
              <p>{galat.pesan}</p>
              {galat.kode === KODE_MASTER_DIRUJUK && (
                <p className="text-xs">Hapus tidak mungkin; gunakan {saran}.</p>
              )}
            </div>
          </div>
        )}
        <DialogFooter>
          <SpringButton
            variant="soft"
            onClick={() => tutup(false)}
            disabled={hapus.isPending}
          >
            {tertahan ? "Tutup" : "Batal"}
          </SpringButton>
          {!tertahan && (
            <SpringButton
              variant="danger"
              onClick={() => void kirim()}
              disabled={hapus.isPending}
            >
              {hapus.isPending ? "Menghapus…" : "Hapus"}
            </SpringButton>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
