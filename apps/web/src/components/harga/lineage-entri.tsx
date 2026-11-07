"use client";

import { GitBranch } from "lucide-react";
import { useLineageHarga } from "@/lib/query/hooks";
import Link from "next/link";
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

function Baris({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <>
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="min-w-0">{children}</dd>
    </>
  );
}

/** Isi lineage: asal, cek otomatis, validasi, publikasi, penolakan, dan garis waktu versi. */
export function IsiLineage({ data }: { data: LineageHarga }) {
  return (
    <div className="space-y-3">
      <dl className="grid grid-cols-[5.5rem_1fr] gap-x-3 gap-y-1.5 text-xs">
        <Baris label="Dicatat">
          {data.petugasNama ?? "Petugas tidak diketahui"},{" "}
          {formatTanggal(data.dicatatPada, WAKTU)}
        </Baris>
        <Baris label="Diterima">
          server {formatTanggal(data.diterimaPada, WAKTU)}
        </Baris>
        <Baris label="Sumber">
          {LABEL_SUMBER[data.sumber] ?? data.sumber}
          {data.kodePedagang ? <> · pedagang {data.kodePedagang}</> : null}
        </Baris>
        {data.tugas.length > 0 && (
          <Baris label="Tugas">{data.tugas.join(", ")}</Baris>
        )}
        <Baris label="Cek otomatis">
          {data.bendera.length > 0
            ? data.bendera.join("; ")
            : "Lolos semua cek"}
          <span className="block text-muted-foreground">
            Skor anomali{" "}
            <span className="num">{data.skorAnomali.toFixed(2)}</span> · deviasi{" "}
            <span className="num">
              {formatPersen(data.deviasiPct, { signed: true })}
            </span>
          </span>
        </Baris>
        <Baris label="Validasi">
          {data.validasi ? (
            ringkasTahap(data.validasi)
          ) : (
            <span className="text-muted-foreground">Belum</span>
          )}
        </Baris>
        {!data.penolakan && (
          <Baris label="Publikasi">
            {data.publikasi ? (
              ringkasTahap(data.publikasi)
            ) : (
              <span className="text-muted-foreground">Belum</span>
            )}
          </Baris>
        )}
        {data.penolakan && (
          <Baris label="Penolakan">
            {ringkasTahap(data.penolakan)}
            {data.penolakan.alasan && (
              <span className="block text-muted-foreground">
                {data.penolakan.alasan}
              </span>
            )}
          </Baris>
        )}
      </dl>

      <div>
        <p className="mb-1.5 text-xs font-medium">
          Versi isi{" "}
          <span className="font-normal text-muted-foreground">
            (terkini: v{data.versiTerkini})
          </span>
        </p>
        {data.versi.length === 0 ? (
          <p className="text-xs text-muted-foreground">
            Belum ada versi tercatat.
          </p>
        ) : (
          <ol aria-label="Versi entri" className="grid gap-2">
            {data.versi.map((v) => (
              <li
                key={v.versi}
                className="grid gap-0.5 border-l-2 border-data-300 pl-3 text-xs"
              >
                <p className="flex flex-wrap items-center gap-x-2 gap-y-1">
                  <span className="num font-medium">v{v.versi}</span>
                  <span className="font-medium">
                    {LABEL_AKSI_VERSI[v.aksi] ?? v.aksi}
                  </span>
                  <StatusBadge
                    tone={STATUS_ENTRI[v.status]?.tone ?? "netral"}
                    className="h-5 px-2 text-[11px]"
                  >
                    {STATUS_ENTRI[v.status]?.label ?? v.status}
                  </StatusBadge>
                  <span className="num">{formatRupiah(v.harga)}</span>
                </p>
                <p className="text-muted-foreground">
                  {v.olehNama ?? "Sistem"}, {formatTanggal(v.waktu, WAKTU)}
                </p>
                {v.perubahan.length > 0 && (
                  <p className="text-muted-foreground">
                    Berubah: {ringkasPerubahan(v.perubahan)}
                  </p>
                )}
              </li>
            ))}
          </ol>
        )}
      </div>

      {data.aksiAudit.length > 0 && (
        <p className="text-xs text-muted-foreground">
          Tercatat di audit trail: {data.aksiAudit.join(", ")}
        </p>
      )}
    </div>
  );
}

/** Bagian "Lineage data" pada panel detail entri harga: jejak asal, validasi, publikasi, dan versi. */
export function LineageEntri({ id }: { id: string }) {
  const { data, isLoading, isError } = useLineageHarga(id);
  return (
    <section
      aria-labelledby="lineage-judul"
      className="space-y-3 rounded-xl bg-mist-100/70 p-3 dark:bg-white/5"
    >
      <div className="flex items-center justify-between gap-3">
        <h3
          id="lineage-judul"
          className="flex items-center gap-1.5 text-sm font-medium"
        >
          <GitBranch className="size-4 text-muted-foreground" aria-hidden />{" "}
          Lineage data
        </h3>
        <Link
          href={tautanAuditEntri(id)}
          className="text-xs font-medium text-data-700 underline-offset-4 hover:underline dark:text-data-300"
        >
          Lihat di audit trail
        </Link>
      </div>
      {isLoading ? (
        <div
          className="h-16 animate-pulse rounded-lg bg-white/60 dark:bg-white/5"
          aria-busy="true"
        />
      ) : isError || !data ? (
        <p role="alert" className="text-xs text-destructive">
          Lineage gagal dimuat.
        </p>
      ) : (
        <IsiLineage data={data} />
      )}
    </section>
  );
}
