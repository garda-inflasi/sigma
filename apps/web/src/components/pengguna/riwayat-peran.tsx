"use client";

import { ArrowRight, History } from "lucide-react";
import Link from "next/link";
import { formatRelatif, formatTanggal } from "@/lib/format";
import { tautanAuditPengguna } from "@/lib/pengguna/riwayat-peran";
import { useRiwayatPeran } from "@/lib/query/hooks";

/**
 * Riwayat perubahan peran satu pengguna, dibaca dari audit trail (terbaru dulu), dengan tautan ke /audit yang
 * sudah disaring pada pengguna ini. Dipakai di dialog "Ubah pengguna".
 */
export function RiwayatPeran({ userId }: { userId: string }) {
  const { data, isLoading, isError, error } = useRiwayatPeran(userId);

  return (
    <section
      aria-labelledby="riwayat-peran-judul"
      className="grid gap-2 rounded-xl bg-mist-100/70 p-3 text-sm dark:bg-white/5"
    >
      <div className="flex items-center justify-between gap-3">
        <h3
          id="riwayat-peran-judul"
          className="flex items-center gap-1.5 font-medium"
        >
          <History className="size-4 text-muted-foreground" aria-hidden />{" "}
          Riwayat peran
        </h3>
        <Link
          href={tautanAuditPengguna(userId)}
          className="text-xs font-medium text-data-700 underline-offset-4 hover:underline dark:text-data-300"
        >
          Lihat di audit trail
        </Link>
      </div>

      {isLoading ? (
        <div
          className="h-12 animate-pulse rounded-lg bg-white/60 dark:bg-white/5"
          aria-busy="true"
        />
      ) : isError ? (
        <p role="alert" className="text-xs text-destructive">
          {error instanceof Error
            ? error.message
            : "Riwayat peran gagal dimuat."}
        </p>
      ) : !data || data.entri.length === 0 ? (
        <p className="text-xs text-muted-foreground">
          Belum ada perubahan peran yang tercatat untuk akun ini.
        </p>
      ) : (
        <>
          <ol aria-label="Riwayat peran" className="grid gap-2">
            {data.entri.map((e) => (
              <li
                key={e.id}
                className="grid gap-0.5 border-l-2 border-data-300 pl-3"
              >
                <p className="font-medium">{e.judul}</p>
                <p className="flex flex-wrap items-center gap-1.5">
                  {e.dari !== null && (
                    <>
                      <span className="text-muted-foreground">{e.dari}</span>
                      <ArrowRight
                        className="size-3 text-muted-foreground"
                        aria-label="menjadi"
                      />
                    </>
                  )}
                  <span>{e.ke}</span>
                </p>
                <p className="text-xs text-muted-foreground">
                  oleh {e.aktor} ·{" "}
                  <time
                    dateTime={e.waktu}
                    title={formatTanggal(e.waktu, "d MMM yyyy HH.mm")}
                  >
                    {formatRelatif(e.waktu)}
                  </time>
                </p>
              </li>
            ))}
          </ol>
          {data.terpotong && (
            <p className="text-xs text-muted-foreground">
              Menampilkan {data.entri.length} perubahan terbaru; yang lebih lama
              ada di audit trail.
            </p>
          )}
        </>
      )}
    </section>
  );
}
