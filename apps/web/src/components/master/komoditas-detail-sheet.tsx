"use client";

import { FilePen, Trash2 } from "lucide-react";
import { useState, type ReactNode } from "react";
import {
  InspectorRingkasan,
  InspectorSection,
  MetaGrid,
  MetaItem,
} from "@/components/garda/inspector";
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
import { pesanGalatMaster } from "@/lib/api/master-perubahan";
import type { KomoditasDetail } from "@/lib/api/master";
import { useCan } from "@/lib/auth/session-context";
import type { MasterKomoditasRow } from "@/lib/data/types";
import { formatAngka, formatRupiah } from "@/lib/format";
import { useKomoditasDetail } from "@/lib/query/hooks";
import { CatatanSheet, MemuatSheet, SeksiBernama } from "./detail-baris";
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
  const galat = error ? (
    <div className="px-5 py-4">
      <CatatanSheet galat>{pesanGalatMaster(error)}</CatatanSheet>
    </div>
  ) : null;
  return (
    <Sheet open={!!kode} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="sm:max-w-lg">
        <SheetHeader>
          <SheetTitle>{data?.komoditas.nama ?? "Detail komoditas"}</SheetTitle>
          <SheetDescription>{kode}</SheetDescription>
        </SheetHeader>
        {data ? (
          <Isi detail={data} galat={galat} onUbah={onUbah} onClose={onClose} />
        ) : (
          <SheetBody>
            {isLoading && <MemuatSheet />}
            {galat}
          </SheetBody>
        )}
      </SheetContent>
    </Sheet>
  );
}

function Isi({
  detail,
  galat,
  onUbah,
  onClose,
}: {
  detail: KomoditasDetail;
  /** Galat muat ulang: data lama tetap tampil di bawahnya. */
  galat: ReactNode;
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
      <SheetBody>
        {galat}
        <InspectorRingkasan
          className="border-b-0"
          status={<StatusBadge tone={status.tone}>{status.label}</StatusBadge>}
          nilai={
            <>
              {formatRupiah(k.hargaAcuan)}
              <span className="text-sm font-normal text-muted-foreground">
                /{k.satuan}
              </span>
            </>
          }
          keterangan={
            menunggu
              ? `Harga acuan · versi ${k.versi}. Ada usulan menunggu keputusan.`
              : `Harga acuan · versi ${k.versi}`
          }
        />
        <InspectorSection title="Rincian">
          <MetaGrid>
            <MetaItem label="Kode">
              <span className="num">{k.kode}</span>
            </MetaItem>
            <MetaItem label="Kategori">{k.kategori}</MetaItem>
            <MetaItem label="Satuan">{k.satuan}</MetaItem>
            <MetaItem label="Harga acuan">
              <span className="num">{formatRupiah(k.hargaAcuan)}</span>
            </MetaItem>
            <MetaItem label="Umur simpan">
              <span className="num">{formatAngka(k.shelfLifeHari)} hari</span>
            </MetaItem>
            <MetaItem label="Komoditas pilot">
              {k.pilot ? "Ya" : "Tidak"}
            </MetaItem>
          </MetaGrid>
          {bisaAjukan && k.status === "DIARSIPKAN" && (
            <CatatanSheet>
              Komoditas yang diarsipkan tidak dapat diubah.
            </CatatanSheet>
          )}
        </InspectorSection>

        {menunggu && (
          <SeksiBernama
            label="Usulan menunggu"
            title="Usulan menunggu keputusan"
          >
            <UsulanKartu usulan={menunggu} />
          </SeksiBernama>
        )}

        <SeksiBernama label="Riwayat versi" title="Riwayat versi">
          <RiwayatVersiList riwayat={riwayat} />
        </SeksiBernama>
      </SheetBody>

      {(bisaAjukan || bisaHapus) && (
        <SheetFooter>
          {bisaHapus && (
            <SpringButton
              variant="outline"
              size="sm"
              onClick={() => setHapus(true)}
            >
              <Trash2 /> Hapus
            </SpringButton>
          )}
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
        </SheetFooter>
      )}

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
