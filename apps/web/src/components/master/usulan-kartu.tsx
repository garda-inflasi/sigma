"use client";

import { Ban, CheckCircle2, XCircle } from "lucide-react";
import { useState } from "react";
import { SpringButton } from "@/components/garda/spring-button";
import { StatusBadge } from "@/components/garda/status-badge";
import {
  ENTITAS_LABEL,
  JENIS_LABEL,
  bolehBatalkan,
  pengajuSendiri,
  statusTampil,
  type PerubahanMaster,
} from "@/lib/api/master-perubahan";
import { useCan, useOptionalSession } from "@/lib/auth/session-context";
import { formatTanggal } from "@/lib/format";
import {
  useBatalkanPerubahanMaster,
  useSetujuiPerubahanMaster,
  useTolakPerubahanMaster,
} from "@/lib/query/hooks";
import { cn } from "@/lib/utils";
import { AlasanDialog } from "./alasan-dialog";
import { DiffTabel } from "./diff-tabel";

export const waktuUsulan = (iso: string) =>
  formatTanggal(iso, "d MMM yyyy HH.mm");

/** Hak pengguna atas satu usulan: Setujui/Tolak (bukan pengajunya sendiri) dan Batalkan (hanya pengaju). */
export function useHakUsulan(usulan: PerubahanMaster) {
  const sesi = useOptionalSession();
  const bisaSetujui = useCan("master.setujui");
  const bisaAjukan = useCan("master.ajukan");
  const bisaBatal = bisaAjukan && bolehBatalkan(usulan, sesi?.id);
  return {
    bisaSetujui,
    bisaBatal,
    sendiri: pengajuSendiri(usulan, sesi?.id),
    adaAksi: usulan.status === "MENUNGGU" && (bisaSetujui || bisaBatal),
  };
}

/** Siapa yang memutuskan usulan, kapan, dan alasan penolakannya. */
export function InfoKeputusan({ usulan }: { usulan: PerubahanMaster }) {
  const status = statusTampil(usulan);
  if (usulan.status === "DISETUJUI")
    return (
      <p className="rounded-lg bg-normal-soft/70 p-2 text-xs text-normal-ink dark:bg-normal/15 dark:text-emerald-300">
        Disetujui oleh{" "}
        <span className="font-medium">{usulan.disetujuiOleh?.nama ?? "-"}</span>
        {usulan.disetujuiPada && (
          <>
            {" "}
            · <span className="num">{waktuUsulan(usulan.disetujuiPada)}</span>
          </>
        )}
      </p>
    );
  if (usulan.status === "DITOLAK")
    return (
      <p className="rounded-lg bg-defisit-soft/80 p-2 text-xs text-defisit-ink dark:bg-defisit/15 dark:text-rose-300">
        {status.label === "Dibatalkan" ? "Dibatalkan" : "Ditolak"} oleh{" "}
        <span className="font-medium">{usulan.ditolakOleh?.nama ?? "-"}</span>
        {usulan.ditolakPada && (
          <>
            {" "}
            · <span className="num">{waktuUsulan(usulan.ditolakPada)}</span>
          </>
        )}
        {usulan.alasanPenolakan && (
          <span className="mt-1 block">{usulan.alasanPenolakan}</span>
        )}
      </p>
    );
  return null;
}

/** Tombol keputusan usulan beserta dialog alasannya; `diFooter` merapikan tombol ke kanan di `SheetFooter`. */
export function AksiUsulan({
  usulan,
  diFooter,
}: {
  usulan: PerubahanMaster;
  diFooter?: boolean;
}) {
  const { bisaSetujui, bisaBatal, sendiri, adaAksi } = useHakUsulan(usulan);
  const setujui = useSetujuiPerubahanMaster();
  const tolak = useTolakPerubahanMaster();
  const batalkan = useBatalkanPerubahanMaster();
  const [dialog, setDialog] = useState<"tolak" | "batalkan" | null>(null);
  const nama = usulan.entitasNama ?? usulan.entitasId;
  const entitas = ENTITAS_LABEL[usulan.entitas].toLowerCase();

  return (
    <>
      {adaAksi && (
        <div className="space-y-2">
          <div
            className={cn("flex flex-wrap gap-2", diFooter && "sm:justify-end")}
          >
            {bisaSetujui && (
              <>
                <SpringButton
                  variant="data"
                  size="sm"
                  disabled={sendiri || setujui.isPending}
                  onClick={() => setujui.mutate(usulan.id)}
                >
                  <CheckCircle2 />{" "}
                  {setujui.isPending ? "Menyetujui…" : "Setujui"}
                </SpringButton>
                <SpringButton
                  variant="outline"
                  size="sm"
                  disabled={sendiri}
                  onClick={() => setDialog("tolak")}
                >
                  <XCircle /> Tolak
                </SpringButton>
              </>
            )}
            {bisaBatal && (
              <SpringButton
                variant="ghost"
                size="sm"
                onClick={() => setDialog("batalkan")}
              >
                <Ban /> Batalkan usulan
              </SpringButton>
            )}
          </div>
          {bisaSetujui && sendiri && (
            <p className="text-xs text-muted-foreground">
              Anda yang mengajukan usulan ini. Keputusan harus dari pemegang
              izin persetujuan lain (pemisahan tugas).
            </p>
          )}
        </div>
      )}

      <AlasanDialog
        open={dialog === "tolak"}
        onOpenChange={(v) => !v && setDialog(null)}
        judul={`Tolak usulan untuk ${nama}?`}
        deskripsi={`Data ${entitas} ${nama} tetap seperti sekarang. Alasan penolakan tampil bagi pengaju dan tercatat di audit trail.`}
        label="Alasan penolakan"
        tombol="Tolak usulan"
        wajib
        onKirim={(alasan) => tolak.mutateAsync({ id: usulan.id, alasan })}
      />
      <AlasanDialog
        open={dialog === "batalkan"}
        onOpenChange={(v) => !v && setDialog(null)}
        judul={`Batalkan usulan untuk ${nama}?`}
        deskripsi={`Usulan ditarik dari antrean dan data ${entitas} ${nama} tetap seperti sekarang. Anda dapat mengajukan usulan baru sesudahnya.`}
        label="Alasan pembatalan (opsional)"
        tombol="Batalkan usulan"
        wajib={false}
        onKirim={(alasan) =>
          batalkan.mutateAsync({ id: usulan.id, alasan: alasan || undefined })
        }
      />
    </>
  );
}

/**
 * Satu usulan perubahan: jenis dan status, pengaju, alasan, diff, info keputusan, dan aksi menurut izin.
 * Setujui/Tolak untuk pemegang master.setujui (dinonaktifkan bila ia pengaju: pemisahan tugas); Batalkan
 * hanya untuk pengaju. Dipakai di sheet detail komoditas/pasar.
 */
export function UsulanKartu({ usulan }: { usulan: PerubahanMaster }) {
  const status = statusTampil(usulan);
  const nama = usulan.entitasNama ?? usulan.entitasId;

  return (
    <section
      aria-label={`Usulan ${usulan.id}`}
      className="min-w-0 space-y-3 rounded-lg border border-border bg-secondary p-3 text-[13px]"
    >
      <div className="flex flex-wrap items-center gap-2">
        <StatusBadge tone="data" dot={false}>
          {JENIS_LABEL[usulan.jenis]}
        </StatusBadge>
        <StatusBadge tone={status.tone}>{status.label}</StatusBadge>
        <span className="num text-xs text-muted-foreground">
          {ENTITAS_LABEL[usulan.entitas]} {nama} · menjadi versi {usulan.versi}
        </span>
      </div>

      <p className="text-xs text-muted-foreground">
        Diajukan oleh{" "}
        <span className="font-medium text-foreground">
          {usulan.diajukanOleh?.nama ?? "pengguna tak dikenal"}
        </span>{" "}
        · <span className="num">{waktuUsulan(usulan.dibuatPada)}</span>
      </p>

      {usulan.alasan && (
        <p>
          <span className="font-medium">Alasan:</span> {usulan.alasan}
        </p>
      )}

      <DiffTabel diff={usulan.diff} jenis={usulan.jenis} />
      <InfoKeputusan usulan={usulan} />
      <AksiUsulan usulan={usulan} />
    </section>
  );
}
