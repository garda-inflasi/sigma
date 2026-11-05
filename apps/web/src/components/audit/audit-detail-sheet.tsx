"use client";

import { ExternalLink, Loader2 } from "lucide-react";
import Link from "next/link";
import { StatusBadge } from "@/components/garda/status-badge";
import {
  Sheet,
  SheetContent,
  SheetDescription,
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
  const { data, isLoading, isError } = useAuditDetail(id);

  return (
    <Sheet open={!!id} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="glass-strong w-full gap-0 overflow-y-auto sm:max-w-xl">
        <SheetHeader>
          <SheetTitle>Detail catatan audit</SheetTitle>
          <SheetDescription>
            Catatan bersifat tetap: tidak dapat diubah atau dihapus.
          </SheetDescription>
        </SheetHeader>
        <div className="space-y-5 px-4 pb-6 text-sm">
          {isLoading && (
            <Loader2 className="mx-auto size-5 animate-spin text-muted-foreground" />
          )}
          {isError && (
            <p role="alert" className="text-destructive">
              Catatan tidak dapat dimuat.
            </p>
          )}
          {data && <Isi detail={data} onFilter={onFilter} onClose={onClose} />}
        </div>
      </SheetContent>
    </Sheet>
  );
}

function Isi({
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
    <>
      <div className="flex flex-wrap items-center gap-2">
        <span className="num rounded-md bg-mist-100 px-1.5 py-0.5 text-xs dark:bg-white/5">
          {d.aksi}
        </span>
        <StatusBadge tone={HASIL_TONE[d.hasil]}>{d.hasil}</StatusBadge>
        <span className="num text-xs text-muted-foreground">
          {formatTanggal(d.waktu, "d MMM yyyy HH.mm.ss")}
        </span>
      </div>

      <dl className="grid grid-cols-[7rem_1fr] gap-x-3 gap-y-2">
        <Baris label="Aktor">
          {d.aktor}
          {d.peran && (
            <span className="text-muted-foreground"> · {d.peran}</span>
          )}
          {d.email && (
            <span className="block text-xs text-muted-foreground">
              {d.email}
            </span>
          )}
        </Baris>
        <Baris label="Objek">
          <span className="num">
            {d.entitas} {d.entitasId}
          </span>
          <span className="mt-1 flex flex-wrap gap-3 text-xs">
            <button
              type="button"
              className="text-primary hover:underline"
              onClick={() => saring({ entitas_id: d.entitasId })}
            >
              Jejak objek ini
            </button>
            {tautan && (
              <Link
                href={tautan}
                className="inline-flex items-center gap-1 text-primary hover:underline"
              >
                Buka halaman terkait <ExternalLink className="size-3" />
              </Link>
            )}
          </span>
        </Baris>
        <Baris label="IP">
          <span className="num">{d.ip || "-"}</span>
        </Baris>
        {d.userAgent && (
          <Baris label="Perangkat">
            <span className="break-all text-xs">{d.userAgent}</span>
          </Baris>
        )}
        {d.correlationId && (
          <Baris label="Permintaan">
            <span className="num break-all text-xs">{d.correlationId}</span>
            <button
              type="button"
              className="mt-1 block text-xs text-primary hover:underline"
              onClick={() => saring({ correlation_id: d.correlationId })}
            >
              Semua catatan dari permintaan ini
            </button>
          </Baris>
        )}
      </dl>

      <section aria-label="Perubahan" className="space-y-2">
        <h3 className="font-medium">Perubahan sebelum → sesudah</h3>
        {d.perubahan.length === 0 ? (
          <p className="text-xs text-muted-foreground">
            Aksi ini tidak mencatat keadaan sebelum/sesudah.
          </p>
        ) : (
          <div className="overflow-x-auto rounded-lg border border-border">
            <table className="w-full text-left text-xs">
              <thead className="bg-secondary/60 text-muted-foreground">
                <tr>
                  <th className="px-2 py-1.5 font-medium">Bidang</th>
                  <th className="px-2 py-1.5 font-medium">Sebelum</th>
                  <th className="px-2 py-1.5 font-medium">Sesudah</th>
                </tr>
              </thead>
              <tbody>
                {d.perubahan.map((p) => (
                  <tr key={p.path} className="border-t border-border align-top">
                    <td className="px-2 py-1.5">
                      <span className="num">{p.path}</span>
                      <span className="block text-[10px] text-muted-foreground">
                        {JENIS_LABEL[p.jenis]}
                      </span>
                    </td>
                    <td className="num break-all px-2 py-1.5 text-rose-700 dark:text-rose-300">
                      {formatNilaiAudit(p.sebelum) || "-"}
                    </td>
                    <td className="num break-all px-2 py-1.5 text-emerald-700 dark:text-emerald-300">
                      {formatNilaiAudit(p.sesudah) || "-"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {d.metadata && Object.keys(d.metadata).length > 0 && (
        <section aria-label="Metadata" className="space-y-2">
          <h3 className="font-medium">Metadata</h3>
          <pre className="num overflow-x-auto rounded-lg bg-secondary/60 p-2 text-xs">
            {JSON.stringify(d.metadata, null, 2)}
          </pre>
        </section>
      )}
    </>
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
    <>
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd>{children}</dd>
    </>
  );
}
