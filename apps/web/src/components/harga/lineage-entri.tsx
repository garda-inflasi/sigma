"use client";

import { useLineageHarga } from "@/lib/query/hooks";
import Link from "next/link";
import {
  ActivityTimeline,
  MetaGrid,
  MetaItem,
} from "@/components/garda/inspector";
import { PanelGalat } from "@/components/garda/panel-galat";
import { Shimmer } from "@/components/garda/shimmer";
import { STATUS_ENTRI, StatusBadge } from "@/components/garda/status-badge";
import {
  LABEL_AKSI_VERSI,
  LABEL_SUMBER,
  ringkasPerubahan,
  tautanAuditEntri,
} from "@/lib/api/harga-lineage";
import type { LineageHarga, TahapLineageHarga } from "@/lib/data/types";
import { formatPersen, formatRupiah, formatTanggal } from "@/lib/format";

const WAKTU = "d MMM HH.mm";

/** "Nama, 16 Okt 14.30"; bagian yang tidak tercatat dilewati. */
function ringkasTahap(t: TahapLineageHarga): string {
  return [
    t.olehNama ?? "Tidak tercatat",
    t.pada ? formatTanggal(t.pada, WAKTU) : null,
  ]
    .filter(Boolean)
    .join(", ");
}

/** Isi lineage: asal, cek otomatis, validasi, publikasi, penolakan, dan garis waktu versi. */
export function IsiLineage({ data }: { data: LineageHarga }) {
  return (
    <div className="space-y-4">
      <MetaGrid>
        <MetaItem label="Dicatat">
          {data.petugasNama ?? "Petugas tidak diketahui"},{" "}
          <span className="num">{formatTanggal(data.dicatatPada, WAKTU)}</span>
        </MetaItem>
        <MetaItem label="Diterima">
          server{" "}
          <span className="num">{formatTanggal(data.diterimaPada, WAKTU)}</span>
        </MetaItem>
        <MetaItem label="Sumber">
          {LABEL_SUMBER[data.sumber] ?? data.sumber}
          {data.kodePedagang ? <> · pedagang {data.kodePedagang}</> : null}
        </MetaItem>
        {data.tugas.length > 0 && (
          <MetaItem label="Tugas">{data.tugas.join(", ")}</MetaItem>
        )}
        <MetaItem label="Cek otomatis" wide>
          {data.bendera.length > 0
            ? data.bendera.join("; ")
            : "Lolos semua cek"}
          <span className="block text-xs font-normal text-muted-foreground">
            Skor anomali{" "}
            <span className="num">{data.skorAnomali.toFixed(2)}</span> · deviasi{" "}
            <span className="num">
              {formatPersen(data.deviasiPct, { signed: true })}
            </span>
          </span>
        </MetaItem>
        <MetaItem label="Validasi">
          {data.validasi ? (
            ringkasTahap(data.validasi)
          ) : (
            <span className="font-normal text-muted-foreground">Belum</span>
          )}
        </MetaItem>
        {!data.penolakan && (
          <MetaItem label="Publikasi">
            {data.publikasi ? (
              ringkasTahap(data.publikasi)
            ) : (
              <span className="font-normal text-muted-foreground">Belum</span>
            )}
          </MetaItem>
        )}
        {data.penolakan && (
          <MetaItem label="Penolakan" wide>
            {ringkasTahap(data.penolakan)}
            {data.penolakan.alasan && (
              <span className="block text-xs font-normal text-muted-foreground">
                {data.penolakan.alasan}
              </span>
            )}
          </MetaItem>
        )}
      </MetaGrid>

      <div>
        <p className="mb-2 text-xs font-medium">
          Versi isi{" "}
          <span className="font-normal text-muted-foreground">
            (terkini: v{data.versiTerkini})
          </span>
        </p>
        <ActivityTimeline
          label="Versi entri"
          empty="Belum ada versi tercatat."
          entries={data.versi.map((v) => ({
            id: String(v.versi),
            tone: v.status === "DITOLAK" ? "defisit" : "data",
            title: (
              <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
                <span className="num">v{v.versi}</span>
                {LABEL_AKSI_VERSI[v.aksi] ?? v.aksi}
                <StatusBadge
                  tone={STATUS_ENTRI[v.status]?.tone ?? "netral"}
                  className="h-5 px-2 text-[11px]"
                >
                  {STATUS_ENTRI[v.status]?.label ?? v.status}
                </StatusBadge>
                <span className="num">{formatRupiah(v.harga)}</span>
              </span>
            ),
            meta: (
              <>
                {v.olehNama ?? "Sistem"},{" "}
                <span className="num">{formatTanggal(v.waktu, WAKTU)}</span>
              </>
            ),
            note:
              v.perubahan.length > 0
                ? `Berubah: ${ringkasPerubahan(v.perubahan)}`
                : undefined,
          }))}
        />
      </div>

      {data.aksiAudit.length > 0 && (
        <p className="text-xs text-muted-foreground">
          Tercatat di audit trail: {data.aksiAudit.join(", ")}
        </p>
      )}
    </div>
  );
}

/** Isi bagian "Asal data" pada panel detail entri harga: jejak asal, validasi, publikasi, dan versi. */
export function LineageEntri({ id }: { id: string }) {
  const { data, isLoading, isError, error } = useLineageHarga(id);
  return (
    <div role="group" aria-label="Lineage data" className="space-y-3">
      {isLoading ? (
        <div aria-busy="true" role="status" aria-label="Memuat lineage">
          <Shimmer className="h-16 w-full" />
        </div>
      ) : isError || !data ? (
        <PanelGalat judul="Lineage gagal dimuat." error={error} />
      ) : (
        <IsiLineage data={data} />
      )}
      <Link
        href={tautanAuditEntri(id)}
        className="inline-block text-xs font-medium text-data-700 underline-offset-4 hover:underline dark:text-data-300"
      >
        Lihat di audit trail
      </Link>
    </div>
  );
}
