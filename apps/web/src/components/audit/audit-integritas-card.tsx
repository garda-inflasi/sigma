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
    <p className="min-w-0 truncate">
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
        className="glass flex h-full min-h-[4.5rem] flex-col justify-center gap-1.5 rounded-lg px-3 py-2"
        role="status"
        aria-label="Memuat status integritas"
      >
        <Shimmer className="h-3.5 w-44" />
        <Shimmer className="h-4 w-80 max-w-full" />
        <Shimmer className="h-3 w-56 max-w-full" />
      </div>
    );
  }

  if (isError || !data) {
    return (
      <div className="glass flex h-full min-h-[4.5rem] items-center gap-2 rounded-lg px-3 py-2 text-[13px] text-muted-foreground">
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

  const penjelasan = penjelasanIntegritas(v);

  return (
    <section
      aria-labelledby="audit-integritas-judul"
      className={cn(
        "glass flex h-full min-w-0 flex-col justify-center gap-0.5 rounded-lg px-3 py-2",
        v && v.status !== "VALID" && cn("ring-1", toneClass[tone].ring),
      )}
    >
      <div className="flex min-w-0 items-center gap-1.5">
        <Ikon
          className={cn("size-3.5 shrink-0", toneClass[tone].text)}
          aria-hidden
        />
        <h2
          id="audit-integritas-judul"
          className="truncate text-xs font-normal text-muted-foreground"
        >
          Integritas rantai hash
        </h2>
        <StatusBadge tone={tone} className="h-5 shrink-0 text-[10px]">
          {meta?.label ?? "Belum diverifikasi"}
        </StatusBadge>
        {barisBermasalah && (
          <button
            type="button"
            onClick={() => onLihatBaris(barisBermasalah.id)}
            className="ml-auto inline-flex h-6 shrink-0 items-center gap-1 rounded-md border border-defisit/30 bg-card px-2 text-xs font-medium text-defisit-ink transition-colors hover:bg-defisit-soft"
          >
            <Search className="size-3" aria-hidden />
            Lihat baris #{barisBermasalah.id}
          </button>
        )}
      </div>
      <p title={penjelasan} className="truncate text-[13px] font-medium">
        {penjelasan}
      </p>
      <div className="flex min-w-0 flex-col text-[11px] leading-4 text-muted-foreground xl:flex-row xl:gap-2">
        {v && waktu && (
          <p className="min-w-0 truncate xl:shrink-0">
            Diverifikasi{" "}
            <time dateTime={waktu}>
              {formatTanggal(waktu, "d MMM yyyy HH.mm")}
            </time>{" "}
            ({v.pemicu === "MANUAL" ? "manual" : "terjadwal"})
            {v.hashAkhir && (
              <>
                {" · "}Hash{" "}
                <span className="num" title={v.hashAkhir}>
                  {v.hashAkhir.slice(0, 12)}…
                </span>
              </>
            )}
          </p>
        )}
        <RingkasanPartisi data={data} />
      </div>
    </section>
  );
}
