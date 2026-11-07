"use client";

import {
  formatNilaiBidang,
  labelBidang,
  perubahanAntarVersi,
  type RiwayatVersi,
} from "@/lib/api/master-perubahan";
import { formatTanggal } from "@/lib/format";
import { cn } from "@/lib/utils";

/**
 * Garis waktu versi komoditas, terbaru di atas: nomor versi, rentang berlaku, dan bidang yang berbeda dari
 * versi sebelumnya (harga lama tetap merujuk definisi yang berlaku saat itu).
 */
export function RiwayatVersiList({ riwayat }: { riwayat: RiwayatVersi[] }) {
  if (riwayat.length === 0)
    return (
      <p className="text-xs text-muted-foreground">Belum ada riwayat versi.</p>
    );
  const baris = perubahanAntarVersi(riwayat).sort(
    (a, b) => b.versi.versi - a.versi.versi,
  );
  return (
    <ol
      className="relative space-y-4 border-l border-border pl-4"
      aria-label="Riwayat versi"
    >
      {baris.map(({ versi: v, perubahan }) => {
        const berjalan = v.berlakuSampai === null;
        return (
          <li key={v.id} className="relative">
            <span
              className={cn(
                "absolute top-1.5 -left-[21px] size-2.5 rounded-full ring-2 ring-background",
                berjalan ? "bg-data-500" : "bg-mist-300 dark:bg-white/20",
              )}
            />
            <p className="flex flex-wrap items-center gap-2 text-sm font-medium">
              Versi {v.versi}
              {berjalan && (
                <span className="rounded-full bg-data-50 px-2 py-0.5 text-[11px] font-medium text-data-700 dark:bg-data-500/15 dark:text-data-300">
                  Berlaku
                </span>
              )}
            </p>
            <p className="num text-xs text-muted-foreground">
              {formatTanggal(v.berlakuDari)} –{" "}
              {v.berlakuSampai ? formatTanggal(v.berlakuSampai) : "sekarang"}
            </p>
            {perubahan.length > 0 ? (
              <ul className="mt-1 space-y-0.5 text-xs">
                {perubahan.map((p) => (
                  <li key={p.bidang}>
                    <span className="text-muted-foreground">
                      {labelBidang(p.bidang)}:
                    </span>{" "}
                    <span className="num text-defisit-ink dark:text-rose-300">
                      {formatNilaiBidang(p.bidang, p.sebelum)}
                    </span>
                    {" → "}
                    <span className="num text-normal-ink dark:text-emerald-300">
                      {formatNilaiBidang(p.bidang, p.sesudah)}
                    </span>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="mt-1 text-xs text-muted-foreground">
                {v.versi === Math.min(...riwayat.map((r) => r.versi))
                  ? "Versi awal."
                  : "Tanpa perubahan isi (hanya tinjauan status)."}
              </p>
            )}
          </li>
        );
      })}
    </ol>
  );
}
