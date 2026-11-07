"use client";

import { FilePen, Loader2, Trash2 } from "lucide-react";
import { useState } from "react";
import { SpringButton } from "@/components/garda/spring-button";
import { StatusBadge } from "@/components/garda/status-badge";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { pesanGalatMaster } from "@/lib/api/master-perubahan";
import type { KomoditasDetail } from "@/lib/api/master";
import { useCan } from "@/lib/auth/session-context";
import type { MasterKomoditasRow } from "@/lib/data/types";
import { formatAngka, formatRupiah } from "@/lib/format";
import { useKomoditasDetail } from "@/lib/query/hooks";
import { CatatanSheet, DlBaris } from "./detail-baris";
import { HapusDialog } from "./hapus-dialog";
import { RiwayatVersiList } from "./riwayat-versi";
import { STATUS_KOMODITAS } from "./status";
import { UsulanKartu } from "./usulan-kartu";

/**
 * Detail komoditas: bidang, usulan yang menunggu (dengan aksi setujui/tolak/batalkan), riwayat versi, dan
 * hapus. `onUbah` membuka dialog "Ajukan perubahan" milik halaman.
 */
export function KomoditasDetailSheet({
  kode,
  onClose,
  onUbah,
}: {
  kode: string | null;
  onClose: () => void;
  onUbah: (k: MasterKomoditasRow) => void;
}) {
  const { data, isLoading, error } = useKomoditasDetail(kode);
  return (
    <Sheet open={!!kode} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="glass-strong w-full gap-0 overflow-y-auto border-0 sm:max-w-lg">
        <SheetHeader>
          <SheetTitle>{data?.komoditas.nama ?? "Detail komoditas"}</SheetTitle>
          <SheetDescription>{kode}</SheetDescription>
        </SheetHeader>
        <div className="space-y-5 px-4 pb-6 text-sm">
          {isLoading && (
            <Loader2 className="mx-auto size-5 animate-spin text-muted-foreground" />
          )}
          {error && (
            <CatatanSheet galat>{pesanGalatMaster(error)}</CatatanSheet>
          )}
          {data && <Isi detail={data} onUbah={onUbah} onClose={onClose} />}
        </div>
      </SheetContent>
    </Sheet>
  );
}

function Isi({
  detail,
  onUbah,
  onClose,
}: {
  detail: KomoditasDetail;
  onUbah: (k: MasterKomoditasRow) => void;
  onClose: () => void;
}) {
  const { komoditas: k, riwayat, menunggu } = detail;
  const bisaAjukan = useCan("master.ajukan");
  const bisaHapus = useCan("master.setujui");
  const [hapus, setHapus] = useState(false);
  const status = STATUS_KOMODITAS[k.status];
  return (
    <>
      <div className="flex flex-wrap items-center gap-2">
        <StatusBadge tone={status.tone}>{status.label}</StatusBadge>
        <span className="num text-xs text-muted-foreground">
          Versi {k.versi}
        </span>
      </div>

      <dl className="grid grid-cols-[8rem_1fr] gap-x-3 gap-y-2">
        <DlBaris label="Kode">
          <span className="num">{k.kode}</span>
        </DlBaris>
        <DlBaris label="Kategori">{k.kategori}</DlBaris>
        <DlBaris label="Satuan">{k.satuan}</DlBaris>
        <DlBaris label="Harga acuan">
          <span className="num">{formatRupiah(k.hargaAcuan)}</span>
        </DlBaris>
        <DlBaris label="Umur simpan">
          <span className="num">{formatAngka(k.shelfLifeHari)} hari</span>
        </DlBaris>
        <DlBaris label="Komoditas pilot">{k.pilot ? "Ya" : "Tidak"}</DlBaris>
      </dl>

      {(bisaAjukan || bisaHapus) && (
        <div className="flex flex-wrap gap-2">
          {bisaAjukan && (
            <SpringButton
              variant="primary"
              size="sm"
              disabled={k.status !== "AKTIF"}
              onClick={() => onUbah(k)}
            >
              <FilePen /> Ajukan perubahan
            </SpringButton>
          )}
          {bisaHapus && (
            <SpringButton
              variant="outline"
              size="sm"
              onClick={() => setHapus(true)}
            >
              <Trash2 /> Hapus
            </SpringButton>
          )}
        </div>
      )}
      {bisaAjukan && k.status === "DIARSIPKAN" && (
        <CatatanSheet>
          Komoditas yang diarsipkan tidak dapat diubah.
        </CatatanSheet>
      )}

      {menunggu && (
        <section className="space-y-2" aria-label="Usulan menunggu">
          <h3 className="font-medium">Usulan menunggu keputusan</h3>
          <UsulanKartu usulan={menunggu} />
        </section>
      )}

      <section className="space-y-2" aria-label="Riwayat versi">
        <h3 className="font-medium">Riwayat versi</h3>
        <RiwayatVersiList riwayat={riwayat} />
      </section>

      <HapusDialog
        entitas="komoditas"
        kode={k.kode}
        nama={k.nama}
        open={hapus}
        onOpenChange={setHapus}
        onDihapus={onClose}
      />
    </>
  );
}
