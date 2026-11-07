"use client";

import {
  formatNilaiBidang,
  labelBidang,
  type DiffBaris,
  type JenisPerubahan,
} from "@/lib/api/master-perubahan";
import { useKecamatanByKode } from "@/lib/query/hooks";

/** Nilai satu bidang; kode kecamatan dilengkapi nama dari server (kode tetap tampil bila nama belum ada). */
export function NilaiBidang({
  bidang,
  nilai,
}: {
  bidang: string;
  nilai: unknown;
}) {
  const kode =
    bidang === "kecamatan_kode" && typeof nilai === "string" && nilai
      ? nilai
      : null;
  const { data } = useKecamatanByKode(kode);
  if (kode) {
    return (
      <span>
        {data ? `${data.nama} ` : ""}
        <span className="num text-[11px] text-muted-foreground">
          {data ? `(${kode})` : kode}
        </span>
      </span>
    );
  }
  return <>{formatNilaiBidang(bidang, nilai)}</>;
}

/** Tabel bidang | sebelum | sesudah dari satu usulan. Usulan tinjauan status tidak punya beda isi. */
export function DiffTabel({
  diff,
  jenis,
}: {
  diff: DiffBaris[];
  jenis: JenisPerubahan;
}) {
  if (diff.length === 0) {
    return (
      <p className="text-xs text-muted-foreground">
        {jenis === "TINJAU"
          ? "Tinjauan status: isi data tidak berubah."
          : "Tidak ada perubahan isi."}
      </p>
    );
  }
  return (
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
          {diff.map((d) => (
            <tr key={d.bidang} className="border-t border-border align-top">
              <td className="px-2 py-1.5">{labelBidang(d.bidang)}</td>
              <td className="num break-all px-2 py-1.5 text-defisit-ink dark:text-rose-300">
                <NilaiBidang bidang={d.bidang} nilai={d.sebelum} />
              </td>
              <td className="num break-all px-2 py-1.5 text-normal-ink dark:text-emerald-300">
                <NilaiBidang bidang={d.bidang} nilai={d.sesudah} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
