"use client";

import {
  Search,
  ShieldAlert,
  ShieldCheck,
  ShieldQuestionMark,
} from "lucide-react";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import { penjelasanIntegritas, rentangPartisiAudit } from "@/lib/api/audit";
import type { AuditIntegritas, AuditVerifikasi } from "@/lib/data/types";
import { toneClass, type Tone } from "@/lib/design/tokens";
import { formatTanggal } from "@/lib/format";
import { useIntegritasAudit } from "@/lib/query/hooks";
import { cn } from "@/lib/utils";

const STATUS: Record<
  AuditVerifikasi["status"],
  { tone: Tone; label: string; Ikon: typeof ShieldCheck }
> = {
  VALID: { tone: "normal", label: "Utuh", Ikon: ShieldCheck },
  RUSAK: { tone: "defisit", label: "Rusak", Ikon: ShieldAlert },
  GAGAL: { tone: "waspada", label: "Verifikasi gagal", Ikon: ShieldAlert },
};

const bulanUtc = new Intl.DateTimeFormat("id-ID", {
  month: "short",
  year: "numeric",
  timeZone: "UTC",
});
/** Partisi berbatas awal bulan UTC; ditulis dalam UTC agar tidak bergeser di zona waktu lain. */
const bulan = (iso: string) => bulanUtc.format(new Date(iso));

/** Baris kecil: jumlah partisi aktif beserta rentangnya, dan arsip (bila ada). */
function RingkasanPartisi({ data }: { data: AuditIntegritas }) {
  const { jumlah, terlama, terbaru } = rentangPartisiAudit(data.partisi);
  const arsipTerbaru = data.arsip[0];
  if (jumlah === 0 && !arsipTerbaru) return null;
  return (
    <p className="text-xs text-muted-foreground">
      {jumlah > 0 && terlama && terbaru && (
        <>
          {jumlah} partisi bulanan aktif (
          {terlama === terbaru
            ? bulan(terlama)
            : `${bulan(terlama)} – ${bulan(terbaru)}`}
          )
        </>
      )}
      {arsipTerbaru && (
        <>
          {jumlah > 0 && " · "}
          {data.arsip.length} partisi diarsipkan (terbaru:{" "}
          <span className="num" title={`sha256 ${arsipTerbaru.sha256}`}>
            {arsipTerbaru.berkas}
          </span>
          )
        </>
      )}
    </p>
  );
}

/**
 * Status rantai hash audit trail: hasil verifikasi terakhir, partisi bulanan aktif, dan arsip.
 * Galat dari endpoint tidak mengganggu halaman; kartu hanya menampilkan bahwa status tidak tersedia.
 */
export function AuditIntegritasCard({
  onLihatBaris,
}: {
  /** Membuka detail baris pertama yang melanggar rantai (status RUSAK). */
  onLihatBaris: (id: string) => void;
}) {
  const { data, isLoading, isError } = useIntegritasAudit();

  if (isLoading) {
    return (
      <div
        className="glass space-y-3 rounded-xl p-4"
        role="status"
        aria-label="Memuat status integritas"
      >
        <Shimmer className="h-5 w-44" />
        <Shimmer className="h-4 w-80 max-w-full" />
        <Shimmer className="h-3 w-56 max-w-full" />
      </div>
    );
  }

  if (isError || !data) {
    return (
      <div className="glass flex items-center gap-2 rounded-xl px-4 py-3 text-sm text-muted-foreground">
        <ShieldQuestionMark className="size-4 shrink-0" aria-hidden />
        Status integritas tidak tersedia
      </div>
    );
  }

  const v = data.verifikasiTerakhir;
  const meta = v ? STATUS[v.status] : undefined;
  const tone: Tone = meta?.tone ?? "netral";
  const Ikon = meta?.Ikon ?? ShieldQuestionMark;
  const waktu = v ? (v.selesai ?? v.mulai) : undefined;
  const barisBermasalah = v?.status === "RUSAK" ? v.masalahPertama : undefined;

  return (
    <section
      aria-labelledby="audit-integritas-judul"
      className={cn(
        "glass space-y-2 rounded-xl p-4",
        v && v.status !== "VALID" && cn("ring-1", toneClass[tone].ring),
      )}
    >
      <div className="flex flex-wrap items-start justify-between gap-x-6 gap-y-2">
        <div className="min-w-0 space-y-1.5">
          <div className="flex flex-wrap items-center gap-2">
            <Ikon
              className={cn("size-4 shrink-0", toneClass[tone].text)}
              aria-hidden
            />
            <h2
              id="audit-integritas-judul"
              className="text-sm font-semibold text-navy-900 dark:text-white"
            >
              Integritas rantai hash
            </h2>
            <StatusBadge tone={tone}>
              {meta?.label ?? "Belum diverifikasi"}
            </StatusBadge>
          </div>
          <p className="text-sm">{penjelasanIntegritas(v)}</p>
        </div>
        {barisBermasalah && (
          <button
            type="button"
            onClick={() => onLihatBaris(barisBermasalah.id)}
            className="inline-flex h-9 items-center gap-2 rounded-xl border border-defisit/30 bg-white/80 px-3 text-sm font-medium text-defisit-ink shadow-ambient-sm hover:bg-white dark:bg-white/[0.06] dark:text-rose-300"
          >
            <Search className="size-4" aria-hidden />
            Lihat baris #{barisBermasalah.id}
          </button>
        )}
      </div>

      {v && waktu && (
        <p className="text-xs text-muted-foreground">
          Diverifikasi{" "}
          <time dateTime={waktu}>
            {formatTanggal(waktu, "d MMM yyyy HH.mm")}
          </time>{" "}
          ({v.pemicu === "MANUAL" ? "manual" : "terjadwal"})
          {v.hashAkhir && (
            <>
              {" · "}Hash akhir{" "}
              <span className="num" title={v.hashAkhir}>
                {v.hashAkhir.slice(0, 12)}…
              </span>
            </>
          )}
        </p>
      )}

      <RingkasanPartisi data={data} />
    </section>
  );
}
