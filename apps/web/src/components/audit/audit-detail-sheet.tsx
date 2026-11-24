"use client";

import { ExternalLink } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";
import { EmptyState } from "@/components/garda/empty-state";
import {
  InspectorRingkasan,
  InspectorSection,
  MetaGrid,
  MetaItem,
} from "@/components/garda/inspector";
import { PanelGalat } from "@/components/garda/panel-galat";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import { Button } from "@/components/ui/button";
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
  formatNilaiAudit,
  halamanObjekAudit,
  type AuditFilter,
} from "@/lib/api/audit";
import type { AuditDetail, AuditPerubahan } from "@/lib/data/types";
import { formatTanggal } from "@/lib/format";
import { useAuditDetail } from "@/lib/query/hooks";

const HASIL_TONE = {
  BERHASIL: "normal",
  GAGAL: "defisit",
  DITOLAK: "defisit",
} as const;

const HASIL_LABEL = {
  BERHASIL: "Berhasil",
  GAGAL: "Gagal",
  DITOLAK: "Ditolak",
} as const;

const JENIS_LABEL: Record<AuditPerubahan["jenis"], string> = {
  DITAMBAH: "Ditambah",
  DIUBAH: "Diubah",
  DIHAPUS: "Dihapus",
};

/**
 * Panel detail satu entri audit: metadata, tautan ke objek terkait, dan diff
 * sebelum → sesudah. `onFilter` menyaring daftar ke objek atau permintaan yang sama.
 */
export function AuditDetailSheet({
  id,
  onClose,
  onFilter,
}: {
  id: string | undefined;
  onClose: () => void;
  onFilter: (patch: AuditFilter) => void;
}) {
  const { data, isLoading, isError, error } = useAuditDetail(id);

  return (
    <Sheet open={!!id} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="sm:max-w-xl">
        <SheetHeader>
          <SheetTitle>Detail catatan audit</SheetTitle>
          <SheetDescription>
            Catatan bersifat tetap: tidak dapat diubah atau dihapus.
          </SheetDescription>
        </SheetHeader>
        <SheetBody>
          {isLoading && (
            <div
              role="status"
              aria-label="Memuat catatan"
              className="space-y-3 px-5 py-4"
            >
              <Shimmer className="h-4 w-1/3" />
              <Shimmer className="h-24 w-full" />
              <Shimmer className="h-32 w-full" />
            </div>
          )}
          {isError && (
            <div className="px-5 py-4">
              <PanelGalat judul="Catatan tidak dapat dimuat." error={error} />
            </div>
          )}
          {data && (
            <>
              <InspectorRingkasan
                status={
                  <StatusBadge tone={HASIL_TONE[data.hasil]}>
                    {HASIL_LABEL[data.hasil]}
                  </StatusBadge>
                }
                nilai={<span className="break-all">{data.aksi}</span>}
                keterangan={
                  <>
                    {data.aktor} ·{" "}
                    <span className="num">
                      {formatTanggal(data.waktu, "d MMM yyyy HH.mm.ss")}
                    </span>
                  </>
                }
              />
              <Isi detail={data} />
            </>
          )}
        </SheetBody>
        {data && (
          <AksiDetail detail={data} onFilter={onFilter} onClose={onClose} />
        )}
      </SheetContent>
    </Sheet>
  );
}

/** `InspectorSection` yang tetap menjadi region bernama; garis pemisah ada di pembungkus. */
function Seksi({
  label,
  title,
  children,
}: {
  label: string;
  title: string;
  children: ReactNode;
}) {
  return (
    <section
      aria-label={label}
      className="border-t border-border first:border-t-0"
    >
      <InspectorSection title={title}>{children}</InspectorSection>
    </section>
  );
}

/** Tindakan lanjutan dari satu catatan: menyaring daftar atau membuka halaman objeknya. */
function AksiDetail({
  detail: d,
  onFilter,
  onClose,
}: {
  detail: AuditDetail;
  onFilter: (patch: AuditFilter) => void;
  onClose: () => void;
}) {
  const tautan = halamanObjekAudit(d.entitas);
  const saring = (patch: AuditFilter) => {
    onFilter(patch);
    onClose();
  };
  return (
    <SheetFooter>
      {d.correlationId && (
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={() => saring({ correlation_id: d.correlationId })}
        >
          Semua catatan dari permintaan ini
        </Button>
      )}
      {tautan && (
        <Button asChild variant="outline" size="sm">
          <Link href={tautan}>
            Buka halaman terkait <ExternalLink className="size-3" />
          </Link>
        </Button>
      )}
      <Button
        type="button"
        variant="outline"
        size="sm"
        onClick={() => saring({ entitas_id: d.entitasId })}
      >
        Jejak objek ini
      </Button>
    </SheetFooter>
  );
}

function Isi({ detail: d }: { detail: AuditDetail }) {
  return (
    <>
      <InspectorSection title="Rincian" className="border-t-0">
        <MetaGrid>
          <MetaItem label="Aktor">
            {d.aktor}
            {d.peran && (
              <span className="font-normal text-muted-foreground">
                {" "}
                · {d.peran}
              </span>
            )}
            {d.email && (
              <span className="block text-xs font-normal text-muted-foreground">
                {d.email}
              </span>
            )}
          </MetaItem>
          <MetaItem label="IP">
            <span className="num">{d.ip || "-"}</span>
          </MetaItem>
          <MetaItem label="Objek" wide>
            <span className="num">
              {d.entitas} {d.entitasId}
            </span>
          </MetaItem>
          {d.userAgent && (
            <MetaItem label="Perangkat" wide>
              <span className="text-xs font-normal break-all">
                {d.userAgent}
              </span>
            </MetaItem>
          )}
          {d.correlationId && (
            <MetaItem label="Permintaan" wide>
              <span className="num text-xs font-normal break-all">
                {d.correlationId}
              </span>
            </MetaItem>
          )}
        </MetaGrid>
      </InspectorSection>

      <Seksi label="Perubahan" title="Perubahan sebelum → sesudah">
        {d.perubahan.length === 0 ? (
          <EmptyState
            title="Aksi ini tidak mencatat keadaan sebelum/sesudah."
            className="py-6"
          />
        ) : (
          <div className="overflow-x-auto rounded-lg border border-border">
            <table className="w-full text-left text-[13px]">
              <thead className="bg-secondary text-[11px] tracking-wide text-muted-foreground uppercase">
                <tr>
                  <th className="h-9 px-3 font-medium">Bidang</th>
                  <th className="h-9 px-3 font-medium">Sebelum</th>
                  <th className="h-9 px-3 font-medium">Sesudah</th>
                </tr>
              </thead>
              <tbody>
                {d.perubahan.map((p) => (
                  <tr
                    key={p.path}
                    className="border-t border-border align-top hover:bg-secondary/60"
                  >
                    <td className="px-3 py-2">
                      <span className="num">{p.path}</span>
                      <span className="block text-[11px] text-muted-foreground">
                        {JENIS_LABEL[p.jenis]}
                      </span>
                    </td>
                    <td className="num px-3 py-2 [overflow-wrap:anywhere] text-defisit-ink dark:text-rose-300">
                      {formatNilaiAudit(p.sebelum) || "-"}
                    </td>
                    <td className="num px-3 py-2 [overflow-wrap:anywhere] text-normal-ink dark:text-emerald-300">
                      {formatNilaiAudit(p.sesudah) || "-"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Seksi>

      {d.metadata && Object.keys(d.metadata).length > 0 && (
        <Seksi label="Metadata" title="Metadata">
          <pre className="num overflow-x-auto rounded-lg bg-secondary p-3 text-xs">
            {JSON.stringify(d.metadata, null, 2)}
          </pre>
        </Seksi>
      )}
    </>
  );
}
