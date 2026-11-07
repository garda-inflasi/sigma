"use client";

import { Check, ChevronsUp, CircleCheckBig, Sparkles } from "lucide-react";
import { STATUS_EWS, StatusBadge } from "@/components/garda/status-badge";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import type { EwsRiwayat } from "@/lib/api/ews";
import type { StatusEws } from "@/lib/data/types";
import { severityMeta, toneClass } from "@/lib/design/tokens";
import { formatTanggal } from "@/lib/format";
import { useEwsDetail, useEwsRiwayat } from "@/lib/query/hooks";
import { cn } from "@/lib/utils";
import { JENIS_EWS, formatNilaiEws } from "./ews-meta";

const IKON_STATUS = {
  BARU: Sparkles,
  DIAKUI: Check,
  DIESKALASI: ChevronsUp,
  SELESAI: CircleCheckBig,
} satisfies Record<StatusEws, unknown>;

/** Timeline perubahan status dari server: siapa, kapan, dan catatannya. */
function RiwayatAlert({ id }: { id: string }) {
  const { data, isLoading, isError } = useEwsRiwayat(id);
  if (isLoading)
    return <p className="text-sm text-muted-foreground">Memuat riwayat…</p>;
  if (isError)
    return (
      <p className="text-sm text-defisit-ink">Riwayat status gagal dimuat.</p>
    );
  if (!data?.length)
    return (
      <p className="text-sm text-muted-foreground">Belum ada riwayat status.</p>
    );
  return (
    <ol aria-label="Riwayat status" className="relative space-y-4 pl-7">
      <span className="absolute top-2 bottom-2 left-[9px] w-0.5 bg-border" />
      {data.map((h: EwsRiwayat) => {
        const Ikon = IKON_STATUS[h.ke];
        return (
          <li key={h.id} className="relative">
            <span
              className={cn(
                "absolute top-0.5 -left-7 grid size-5 place-items-center rounded-full text-white",
                toneClass[STATUS_EWS[h.ke].tone].bg,
              )}
            >
              <Ikon className="size-3" aria-hidden />
            </span>
            <p className="text-sm font-medium">{STATUS_EWS[h.ke].label}</p>
            <p className="text-xs text-muted-foreground">
              {h.olehNama ?? "Sistem"}, {formatTanggal(h.waktu, "d MMM HH.mm")}
            </p>
            {h.catatan && <p className="mt-1 text-xs">{h.catatan}</p>}
          </li>
        );
      })}
    </ol>
  );
}

function Baris({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex items-start justify-between gap-4 text-sm">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="text-right">{children}</dd>
    </div>
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
      <SheetContent className="glass-strong w-full overflow-y-auto border-0 sm:max-w-md">
        <SheetHeader>
          <SheetTitle>{a?.judul ?? "Detail peringatan"}</SheetTitle>
          <SheetDescription>
            {a ? `${a.wilayah}, ${JENIS_EWS[a.jenis].label}` : (id ?? "")}
          </SheetDescription>
        </SheetHeader>
        <div className="space-y-5 px-4 pb-6">
          {isLoading && (
            <p className="text-sm text-muted-foreground">Memuat detail…</p>
          )}
          {isError && (
            <p className="rounded-xl bg-defisit-soft p-3 text-sm text-defisit-ink">
              {error instanceof Error
                ? error.message
                : "Detail peringatan gagal dimuat."}
            </p>
          )}
          {a && meta && id && (
            <>
              <div className="flex items-end justify-between gap-3">
                <div>
                  <p
                    className={cn(
                      "num text-3xl font-semibold",
                      toneClass[meta.tone].text,
                    )}
                  >
                    {formatNilaiEws(a)}
                  </p>
                  <p
                    className={cn(
                      "text-xs font-medium",
                      toneClass[meta.tone].text,
                    )}
                  >
                    {meta.label}
                  </p>
                </div>
                <StatusBadge tone={STATUS_EWS[a.status].tone}>
                  {STATUS_EWS[a.status].label}
                </StatusBadge>
              </div>
              <p className="text-sm">{a.pesan}</p>
              <dl className="space-y-2">
                <Baris label="Komoditas">
                  {a.komoditasNama ?? "Semua komoditas"}
                </Baris>
                <Baris label="Skor dampak">
                  <span className="num">{a.skorDampak}</span>
                </Baris>
                <Baris label="Sumber">{a.sumber}</Baris>
                <Baris label="Dibuat">
                  {formatTanggal(a.waktu, "d MMM yyyy HH.mm")}
                </Baris>
                {a.diakuiPada && (
                  <Baris label="Diakui">
                    {formatTanggal(a.diakuiPada, "d MMM yyyy HH.mm")}
                  </Baris>
                )}
                {a.dieskalasiPada && (
                  <Baris label="Dieskalasi">
                    {formatTanggal(a.dieskalasiPada, "d MMM yyyy HH.mm")}
                  </Baris>
                )}
                {a.selesaiPada && (
                  <Baris label="Selesai">
                    {formatTanggal(a.selesaiPada, "d MMM yyyy HH.mm")}
                  </Baris>
                )}
                {a.aturanId && (
                  <Baris label="Aturan">{a.aturanKode ?? a.aturanId}</Baris>
                )}
              </dl>
              <div>
                <h3 className="mb-3 text-sm font-medium">Riwayat status</h3>
                <RiwayatAlert id={id} />
              </div>
            </>
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}
