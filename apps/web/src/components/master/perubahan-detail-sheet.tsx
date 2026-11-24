"use client";

import type { ReactNode } from "react";
import {
  InspectorRingkasan,
  InspectorSection,
  MetaGrid,
  MetaItem,
} from "@/components/garda/inspector";
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
import {
  ENTITAS_LABEL,
  JENIS_LABEL,
  pesanGalatMaster,
  ringkasDiff,
  statusTampil,
  type PerubahanMaster,
} from "@/lib/api/master-perubahan";
import { usePerubahanDetail } from "@/lib/query/hooks";
import { CatatanSheet, MemuatSheet } from "./detail-baris";
import { DiffTabel } from "./diff-tabel";
import {
  AksiUsulan,
  InfoKeputusan,
  useHakUsulan,
  waktuUsulan,
} from "./usulan-kartu";

/**
 * Detail satu usulan di antrean: diff lengkap (bidang, sebelum → sesudah), alasan, info keputusan, dan aksi
 * setujui/tolak/batalkan menurut izin dan kepemilikan.
 */
export function PerubahanDetailSheet({
  id,
  onClose,
}: {
  id: string | null;
  onClose: () => void;
}) {
  const { data, isLoading, error } = usePerubahanDetail(id);
  const galat = error ? (
    <div className="px-5 py-4">
      <CatatanSheet galat>{pesanGalatMaster(error)}</CatatanSheet>
    </div>
  ) : null;
  return (
    <Sheet open={!!id} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="sm:max-w-lg">
        <SheetHeader>
          <SheetTitle>
            {data
              ? `${ENTITAS_LABEL[data.entitas]} ${data.entitasNama ?? data.entitasId}`
              : "Detail usulan"}
          </SheetTitle>
          <SheetDescription>
            {data ? (
              <span className="num">{data.entitasId}</span>
            ) : (
              "Usulan perubahan master data"
            )}
          </SheetDescription>
        </SheetHeader>
        {data ? (
          <IsiUsulan usulan={data} galat={galat} />
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

function IsiUsulan({
  usulan,
  galat,
}: {
  usulan: PerubahanMaster;
  /** Galat muat ulang: data lama tetap tampil di bawahnya. */
  galat: ReactNode;
}) {
  const { adaAksi } = useHakUsulan(usulan);
  const status = statusTampil(usulan);
  return (
    <>
      <SheetBody>
        {galat}
        <InspectorRingkasan
          className="border-b-0"
          status={
            <>
              <StatusBadge tone="data" dot={false}>
                {JENIS_LABEL[usulan.jenis]}
              </StatusBadge>
              <StatusBadge tone={status.tone}>{status.label}</StatusBadge>
            </>
          }
          nilai={ringkasDiff(usulan)}
          keterangan={
            usulan.status === "MENUNGGU"
              ? `Bila disetujui, data ini menjadi versi ${usulan.versi}`
              : usulan.status === "DISETUJUI"
                ? `Data ini menjadi versi ${usulan.versi}`
                : "Data tidak berubah"
          }
        />
        <InspectorSection title="Pengajuan">
          <MetaGrid>
            <MetaItem label="Diajukan oleh">
              {usulan.diajukanOleh?.nama ?? "pengguna tak dikenal"}
            </MetaItem>
            <MetaItem label="Waktu">
              <span className="num">{waktuUsulan(usulan.dibuatPada)}</span>
            </MetaItem>
            {usulan.alasan && (
              <MetaItem label="Alasan" wide>
                {usulan.alasan}
              </MetaItem>
            )}
          </MetaGrid>
        </InspectorSection>
        <InspectorSection title="Perubahan">
          <DiffTabel diff={usulan.diff} jenis={usulan.jenis} />
        </InspectorSection>
        {usulan.status !== "MENUNGGU" && (
          <InspectorSection title="Keputusan">
            <InfoKeputusan usulan={usulan} />
          </InspectorSection>
        )}
      </SheetBody>
      {adaAksi && (
        <SheetFooter className="flex-col sm:flex-col sm:flex-nowrap">
          <AksiUsulan usulan={usulan} diFooter />
        </SheetFooter>
      )}
    </>
  );
}
