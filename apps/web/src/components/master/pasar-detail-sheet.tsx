"use client";

import { FilePen, Trash2 } from "lucide-react";
import { useState, type ReactNode } from "react";
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
import {
  CatatanSheet,
  MemuatSheet,
  SeksiBernama,
  formatKoordinat,
} from "./detail-baris";
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
  const galat = error ? (
    <div className="px-5 py-4">
      <CatatanSheet galat>{pesanGalatMaster(error)}</CatatanSheet>
    </div>
  ) : null;
  return (
    <Sheet open={!!kode} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="sm:max-w-lg">
        <SheetHeader>
          <SheetTitle>{data?.pasar.nama ?? "Detail pasar"}</SheetTitle>
          <SheetDescription>{data?.pasar.kode ?? kode}</SheetDescription>
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
  detail: PasarDetail;
  /** Galat muat ulang: data lama tetap tampil di bawahnya. */
  galat: ReactNode;
  onUbah: (p: MasterPasarRow) => void;
  onClose: () => void;
}) {
  const { pasar: p, jumlahPedagang } = detail;
  // Sejak 04-D `id` pasar adalah angka surrogate; usulan, pedagang, dan hapus merujuk kode pasar.
  const kode = p.kode ?? p.id;
  const bisaAjukan = useCan("master.ajukan");
  const bisaHapus = useCan("master.setujui");
  const [hapus, setHapus] = useState(false);
  // Detail pasar tidak memuat usulan; dicari di antrean menurut pasar ini (hanya satu yang bisa menunggu).
  const usulan = usePerubahanHalaman({
    status: "MENUNGGU",
    entitas: "pasar",
    entitasId: kode,
    limit: 1,
  });
  const pedagang = usePedagangHalaman({ pasarId: kode, limit: 50 });
  const menunggu = usulan.data?.rows[0];
  const status = STATUS_PASAR[p.aktif ? "AKTIF" : "NONAKTIF"];
  return (
    <>
      <SheetBody>
        {galat}
        <InspectorRingkasan
          className="border-b-0"
          status={<StatusBadge tone={status.tone}>{status.label}</StatusBadge>}
          nilai={`${formatAngka(jumlahPedagang)} pedagang`}
          keterangan={
            menunggu
              ? `${p.kabupaten} · versi ${p.versi}. Ada usulan menunggu keputusan.`
              : `${p.kabupaten} · versi ${p.versi}`
          }
        />
        <InspectorSection title="Rincian">
          <MetaGrid>
            <MetaItem label="Kode">
              <span className="num">{kode}</span>
            </MetaItem>
            <MetaItem label="Kabupaten/kota">{p.kabupaten}</MetaItem>
            <MetaItem label="Kecamatan">
              {p.kecamatanNama ?? (
                <span className="font-normal text-muted-foreground">
                  Belum diisi
                </span>
              )}
              {p.kecamatanKode && (
                <span className="num ml-1 text-xs font-normal text-muted-foreground">
                  ({p.kecamatanKode})
                </span>
              )}
            </MetaItem>
            <MetaItem label="Koordinat">
              <span className="num">{formatKoordinat(p.lat, p.lng)}</span>
            </MetaItem>
            <MetaItem label="Geofence">
              <span className="num">{formatAngka(p.radiusMeter)} m</span>
            </MetaItem>
            <MetaItem label="Pedagang">
              <span className="num">{formatAngka(jumlahPedagang)}</span>
            </MetaItem>
          </MetaGrid>
        </InspectorSection>

        {menunggu && (
          <SeksiBernama
            label="Usulan menunggu"
            title="Usulan menunggu keputusan"
          >
            <UsulanKartu usulan={menunggu} />
          </SeksiBernama>
        )}

        <SeksiBernama label="Pedagang" title="Pedagang di pasar ini">
          {pedagang.isLoading ? (
            <div
              role="status"
              aria-label="Memuat pedagang"
              className="space-y-2"
            >
              <Shimmer className="h-8 w-full" />
              <Shimmer className="h-8 w-full" />
              <Shimmer className="h-8 w-full" />
            </div>
          ) : pedagang.isError ? (
            <CatatanSheet galat>
              {pesanGalatMaster(pedagang.error)}
            </CatatanSheet>
          ) : (pedagang.data?.rows.length ?? 0) === 0 ? (
            <EmptyState
              title="Belum ada pedagang tercatat di pasar ini."
              className="py-6"
            />
          ) : (
            <ul className="divide-y divide-border rounded-lg border border-border text-xs">
              {pedagang.data!.rows.map((x) => (
                <li
                  key={x.id}
                  className="flex items-center justify-between gap-3 px-3 py-2"
                >
                  <span className="min-w-0 break-words">
                    <span className="num">{x.kode}</span>
                    <span className="ml-2">
                      {x.nama ?? (
                        <span className="text-muted-foreground">
                          tanpa nama
                        </span>
                      )}
                    </span>
                  </span>
                  <span className="flex shrink-0 items-center gap-2 text-muted-foreground">
                    <span className="num">
                      {formatAngka(x.jumlahEntri)} entri
                    </span>
                    {!x.aktif && (
                      <span className="rounded-md bg-secondary px-1.5 py-0.5">
                        nonaktif
                      </span>
                    )}
                  </span>
                </li>
              ))}
            </ul>
          )}
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
            <SpringButton variant="primary" size="sm" onClick={() => onUbah(p)}>
              <FilePen /> Ajukan perubahan
            </SpringButton>
          )}
        </SheetFooter>
      )}

      <HapusDialog
        entitas="pasar"
        kode={kode}
        nama={p.nama}
        open={hapus}
        onOpenChange={setHapus}
        onDihapus={onClose}
      />
    </>
  );
}
