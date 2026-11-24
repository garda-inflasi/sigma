"use client";

import {
  ActivityTimeline,
  InspectorRingkasan,
  InspectorSection,
  MetaGrid,
  MetaItem,
  type TimelineEntry,
} from "@/components/garda/inspector";
import { PanelGalat } from "@/components/garda/panel-galat";
import { Shimmer } from "@/components/garda/shimmer";
import { STATUS_EWS, StatusBadge } from "@/components/garda/status-badge";
import {
  Sheet,
  SheetBody,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import type { EwsRiwayat } from "@/lib/api/ews";
import type { StatusEws } from "@/lib/data/types";
import { severityMeta } from "@/lib/design/tokens";
import { formatTanggal } from "@/lib/format";
import { useEwsDetail, useEwsRiwayat } from "@/lib/query/hooks";
import { JENIS_EWS, formatNilaiEws } from "./ews-meta";

const NADA_RIWAYAT: Record<StatusEws, TimelineEntry["tone"]> = {
  BARU: "data",
  DIAKUI: "data",
  DIESKALASI: "defisit",
  SELESAI: "netral",
};

/** Timeline perubahan status dari server: siapa, kapan, dan catatannya. */
function RiwayatAlert({ id }: { id: string }) {
  const { data, isLoading, isError, error } = useEwsRiwayat(id);
  if (isLoading)
    return (
      <div role="status" aria-label="Memuat riwayat" className="space-y-3">
        <Shimmer className="h-10 w-full" />
        <Shimmer className="h-10 w-full" />
      </div>
    );
  if (isError)
    return <PanelGalat judul="Riwayat status gagal dimuat." error={error} />;
  return (
    <ActivityTimeline
      label="Riwayat status"
      empty="Belum ada riwayat status."
      entries={(data ?? []).map((h: EwsRiwayat) => ({
        id: h.id,
        title: STATUS_EWS[h.ke].label,
        meta: (
          <>
            {h.olehNama ?? "Sistem"},{" "}
            <span className="num">{formatTanggal(h.waktu, "d MMM HH.mm")}</span>
          </>
        ),
        note: h.catatan || undefined,
        tone: NADA_RIWAYAT[h.ke],
      }))}
    />
  );
}

/** Panel samping detail satu alert: data segar dari GET /v1/ews/{id} dan riwayat statusnya. */
export function EwsDetailSheet({
  id,
  onClose,
}: {
  id: string | null;
  onClose: () => void;
}) {
  const { data: a, isLoading, isError, error } = useEwsDetail(id);
  const meta = a ? severityMeta[a.severity] : null;

  return (
    <Sheet open={!!id} onOpenChange={(o: boolean) => !o && onClose()}>
      <SheetContent className="sm:max-w-md">
        <SheetHeader>
          <SheetTitle>{a?.judul ?? "Detail peringatan"}</SheetTitle>
          <SheetDescription>
            {a ? `${a.wilayah}, ${JENIS_EWS[a.jenis].label}` : (id ?? "")}
          </SheetDescription>
        </SheetHeader>
        <SheetBody>
          {isLoading && (
            <div
              role="status"
              aria-label="Memuat detail"
              className="space-y-3 px-5 py-4"
            >
              <Shimmer className="h-9 w-1/3" />
              <Shimmer className="h-12 w-full" />
              <Shimmer className="h-32 w-full" />
            </div>
          )}
          {isError && (
            <div className="px-5 py-4">
              <PanelGalat
                judul="Detail peringatan gagal dimuat."
                error={error}
              />
            </div>
          )}
          {a && meta && id && (
            <>
              <InspectorRingkasan
                status={
                  <>
                    <StatusBadge tone={meta.tone}>{meta.label}</StatusBadge>
                    <StatusBadge tone={STATUS_EWS[a.status].tone}>
                      {STATUS_EWS[a.status].label}
                    </StatusBadge>
                  </>
                }
                nilai={formatNilaiEws(a)}
                keterangan={a.pesan}
              />
              <InspectorSection title="Rincian">
                <MetaGrid>
                  <MetaItem label="Komoditas">
                    {a.komoditasNama ?? "Semua komoditas"}
                  </MetaItem>
                  <MetaItem label="Skor dampak">
                    <span className="num">{a.skorDampak}</span>
                  </MetaItem>
                  <MetaItem label="Sumber">{a.sumber}</MetaItem>
                  <MetaItem label="Dibuat">
                    <span className="num">
                      {formatTanggal(a.waktu, "d MMM yyyy HH.mm")}
                    </span>
                  </MetaItem>
                  {a.diakuiPada && (
                    <MetaItem label="Diakui">
                      <span className="num">
                        {formatTanggal(a.diakuiPada, "d MMM yyyy HH.mm")}
                      </span>
                    </MetaItem>
                  )}
                  {a.dieskalasiPada && (
                    <MetaItem label="Dieskalasi">
                      <span className="num">
                        {formatTanggal(a.dieskalasiPada, "d MMM yyyy HH.mm")}
                      </span>
                    </MetaItem>
                  )}
                  {a.selesaiPada && (
                    <MetaItem label="Selesai">
                      <span className="num">
                        {formatTanggal(a.selesaiPada, "d MMM yyyy HH.mm")}
                      </span>
                    </MetaItem>
                  )}
                  {a.aturanId && (
                    <MetaItem label="Aturan">
                      <span className="num">{a.aturanKode ?? a.aturanId}</span>
                    </MetaItem>
                  )}
                </MetaGrid>
              </InspectorSection>
              <InspectorSection title="Riwayat status">
                <RiwayatAlert id={id} />
              </InspectorSection>
            </>
          )}
        </SheetBody>
      </SheetContent>
    </Sheet>
  );
}
