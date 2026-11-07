"use client";

import { Loader2 } from "lucide-react";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { ENTITAS_LABEL, pesanGalatMaster } from "@/lib/api/master-perubahan";
import { usePerubahanDetail } from "@/lib/query/hooks";
import { CatatanSheet } from "./detail-baris";
import { UsulanKartu } from "./usulan-kartu";

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
  return (
    <Sheet open={!!id} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="glass-strong w-full gap-0 overflow-y-auto border-0 sm:max-w-lg">
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
        <div className="space-y-5 px-4 pb-6 text-sm">
          {isLoading && (
            <Loader2 className="mx-auto size-5 animate-spin text-muted-foreground" />
          )}
          {error && (
            <CatatanSheet galat>{pesanGalatMaster(error)}</CatatanSheet>
          )}
          {data && <UsulanKartu usulan={data} />}
        </div>
      </SheetContent>
    </Sheet>
  );
}
