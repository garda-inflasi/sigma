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
import type { PasarDetail } from "@/lib/api/master";
import { pesanGalatMaster } from "@/lib/api/master-perubahan";
import { useCan } from "@/lib/auth/session-context";
import type { MasterPasarRow } from "@/lib/data/types";
import { formatAngka } from "@/lib/format";
import {
  usePasarDetail,
  usePedagangHalaman,
  usePerubahanHalaman,
} from "@/lib/query/hooks";
import { CatatanSheet, DlBaris } from "./detail-baris";
import { HapusDialog } from "./hapus-dialog";
import { STATUS_PASAR } from "./status";
import { UsulanKartu } from "./usulan-kartu";

/**
 * Detail pasar: kabupaten, kecamatan, versi, geofence, pedagang, dan usulan yang menunggu. `onUbah` membuka
 * dialog "Ajukan perubahan" milik halaman.
 */
export function PasarDetailSheet({
  kode,
  onClose,
  onUbah,
}: {
  kode: string | null;
  onClose: () => void;
  onUbah: (p: MasterPasarRow) => void;
}) {
  const { data, isLoading, error } = usePasarDetail(kode);
  return (
    <Sheet open={!!kode} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="glass-strong w-full gap-0 overflow-y-auto border-0 sm:max-w-lg">
        <SheetHeader>
          <SheetTitle>{data?.pasar.nama ?? "Detail pasar"}</SheetTitle>
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
  detail: PasarDetail;
  onUbah: (p: MasterPasarRow) => void;
  onClose: () => void;
}) {
  const { pasar: p, jumlahPedagang } = detail;
  const bisaAjukan = useCan("master.ajukan");
  const bisaHapus = useCan("master.setujui");
  const [hapus, setHapus] = useState(false);
  // Detail pasar tidak memuat usulan; dicari di antrean menurut pasar ini (hanya satu yang bisa menunggu).
  const usulan = usePerubahanHalaman({
    status: "MENUNGGU",
    entitas: "pasar",
    entitasId: p.id,
    limit: 1,
  });
  const pedagang = usePedagangHalaman({ pasarId: p.id, limit: 50 });
  const menunggu = usulan.data?.rows[0];
  const status = STATUS_PASAR[p.aktif ? "AKTIF" : "NONAKTIF"];
  return (
    <>
      <div className="flex flex-wrap items-center gap-2">
        <StatusBadge tone={status.tone}>{status.label}</StatusBadge>
        <span className="num text-xs text-muted-foreground">
          Versi {p.versi}
        </span>
      </div>

      <dl className="grid grid-cols-[8rem_1fr] gap-x-3 gap-y-2">
        <DlBaris label="Kode">
          <span className="num">{p.id}</span>
        </DlBaris>
        <DlBaris label="Kabupaten/kota">{p.kabupaten}</DlBaris>
        <DlBaris label="Kecamatan">
          {p.kecamatanNama ?? (
            <span className="text-muted-foreground">Belum diisi</span>
          )}
          {p.kecamatanKode && (
            <span className="num ml-1 text-xs text-muted-foreground">
              ({p.kecamatanKode})
            </span>
          )}
        </DlBaris>
        <DlBaris label="Koordinat">
          <span className="num">
            {p.lat}, {p.lng}
          </span>
        </DlBaris>
        <DlBaris label="Geofence">
          <span className="num">{formatAngka(p.radiusMeter)} m</span>
        </DlBaris>
        <DlBaris label="Pedagang">
          <span className="num">{formatAngka(jumlahPedagang)}</span>
        </DlBaris>
      </dl>

      {(bisaAjukan || bisaHapus) && (
        <div className="flex flex-wrap gap-2">
          {bisaAjukan && (
            <SpringButton variant="primary" size="sm" onClick={() => onUbah(p)}>
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

      {menunggu && (
        <section className="space-y-2" aria-label="Usulan menunggu">
          <h3 className="font-medium">Usulan menunggu keputusan</h3>
          <UsulanKartu usulan={menunggu} />
        </section>
      )}

      <section className="space-y-2" aria-label="Pedagang">
        <h3 className="font-medium">Pedagang di pasar ini</h3>
        {pedagang.isLoading ? (
          <Loader2 className="size-4 animate-spin text-muted-foreground" />
        ) : pedagang.isError ? (
          <CatatanSheet galat>{pesanGalatMaster(pedagang.error)}</CatatanSheet>
        ) : (pedagang.data?.rows.length ?? 0) === 0 ? (
          <CatatanSheet>Belum ada pedagang tercatat di pasar ini.</CatatanSheet>
        ) : (
          <ul className="divide-y divide-border rounded-lg border border-border text-xs">
            {pedagang.data!.rows.map((x) => (
              <li
                key={x.id}
                className="flex items-center justify-between gap-3 px-2 py-1.5"
              >
                <span>
                  <span className="num">{x.kode}</span>
                  <span className="ml-2">
                    {x.nama ?? (
                      <span className="text-muted-foreground">tanpa nama</span>
                    )}
                  </span>
                </span>
                <span className="flex items-center gap-2 text-muted-foreground">
                  <span className="num">
                    {formatAngka(x.jumlahEntri)} entri
                  </span>
                  {!x.aktif && (
                    <span className="rounded-full bg-mist-100 px-1.5 py-0.5 dark:bg-white/10">
                      nonaktif
                    </span>
                  )}
                </span>
              </li>
            ))}
          </ul>
        )}
      </section>

      <HapusDialog
        entitas="pasar"
        kode={p.id}
        nama={p.nama}
        open={hapus}
        onOpenChange={setHapus}
        onDihapus={onClose}
      />
    </>
  );
}
