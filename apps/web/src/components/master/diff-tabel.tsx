"use client";

import {
  formatNilaiBidang,
  labelBidang,
  type DiffBaris,
  type JenisPerubahan,
} from "@/lib/api/master-perubahan";
import { useKecamatanByKode } from "@/lib/query/hooks";

const KEPALA =
  "h-9 border-b border-border bg-secondary px-3 text-[11px] font-medium tracking-wide text-muted-foreground uppercase";
const SEL = "h-11 border-b border-border px-3 py-3";

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
      <table className="w-full text-left text-[13px]">
        <thead>
          <tr>
            <th className={KEPALA}>Bidang</th>
            <th className={KEPALA}>Sebelum</th>
            <th className={KEPALA}>Sesudah</th>
          </tr>
        </thead>
        <tbody className="[&>tr:last-child>td]:border-b-0">
          {diff.map((d) => (
            <tr key={d.bidang} className="align-top hover:bg-secondary/60">
              <td className={SEL}>{labelBidang(d.bidang)}</td>
              <td
                className={`${SEL} num [overflow-wrap:anywhere] text-defisit-ink dark:text-rose-300`}
              >
                <NilaiBidang bidang={d.bidang} nilai={d.sebelum} />
              </td>
              <td
                className={`${SEL} num [overflow-wrap:anywhere] text-normal-ink dark:text-emerald-300`}
              >
                <NilaiBidang bidang={d.bidang} nilai={d.sesudah} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
