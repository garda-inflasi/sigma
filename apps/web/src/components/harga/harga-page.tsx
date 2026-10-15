"use client";

import { motion } from "motion/react";
import { Camera, Check, MapPin, MapPinOff, X } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useState } from "react";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { DataTable } from "@/components/data-table/data-table";
import { PageHeader } from "@/components/garda/page-header";
import { STATUS_ENTRI, StatusBadge } from "@/components/garda/status-badge";
import { CountUp } from "@/components/motion/count-up";
import type { EntriHarga, StatusEntri } from "@/lib/data/types";
import { toneClass } from "@/lib/design/tokens";
import { formatPersen, formatRupiah, formatTanggal } from "@/lib/format";
import { fadeUp, stagger } from "@/lib/motion/presets";
import { useEntriHarga } from "@/lib/query/hooks";
import { cn } from "@/lib/utils";
import { FACET_STATUS_ENTRI, useHargaBulkActions, useHargaColumns } from "./harga-table";

const ALUR: StatusEntri[] = ["DRAF", "DIAJUKAN", "DIVALIDASI", "DIPUBLIKASI"];
const getId = (r: EntriHarga) => r.id;

/** Timeline alur entri: draf → diajukan → divalidasi → dipublikasi (atau ditolak). */
function AlurEntri({ e }: { e: EntriHarga }) {
  const ditolak = e.status === "DITOLAK";
  const idx = ditolak ? 1 : ALUR.indexOf(e.status);
  const label: Record<StatusEntri, string> = { DRAF: "Dicatat agen", DIAJUKAN: "Diajukan & cek otomatis", DIVALIDASI: "Divalidasi koordinator", DIPUBLIKASI: "Dipublikasi ke gold layer", DITOLAK: "Ditolak" };
  return (
    <motion.ol variants={stagger(0.08)} initial="hidden" animate="show" className="relative space-y-4 pl-7">
      <span className="absolute top-2 bottom-2 left-[9px] w-0.5 bg-border" />
      {[...ALUR, ...(ditolak ? (["DITOLAK"] as StatusEntri[]) : [])].map((s, i) => {
        const done = ditolak ? i <= 1 || s === "DITOLAK" : i <= idx;
        const tone = s === "DITOLAK" ? "defisit" : done ? "data" : "netral";
        return (
          <motion.li key={s} variants={fadeUp} className="relative">
            <span className={cn("absolute top-0.5 -left-7 grid size-5 place-items-center rounded-full text-white", done ? toneClass[tone].bg : "bg-mist-300 dark:bg-white/15")}>
              {s === "DITOLAK" ? <X className="size-3" /> : done ? <Check className="size-3" /> : null}
            </span>
            <p className={cn("text-sm", done ? "font-medium" : "text-muted-foreground")}>{label[s]}</p>
            {done && s === "DRAF" && <p className="text-xs text-muted-foreground">{e.petugasNama}, {formatTanggal(e.dicatatPada, "d MMM HH.mm")}</p>}
            {done && s === "DIAJUKAN" && <p className="text-xs text-muted-foreground">Rentang wajar, outlier, geofence, duplikasi</p>}
          </motion.li>
        );
      })}
    </motion.ol>
  );
}

export function HargaPage() {
  const params = useSearchParams();
  const status = params.get("status");
  const { data, isLoading } = useEntriHarga();
  const columns = useHargaColumns();
  const actions = useHargaBulkActions();
  const [detail, setDetail] = useState<EntriHarga | null>(null);
  const n = (s: StatusEntri) => data?.filter((e) => e.status === s).length ?? 0;
  const anomali = data?.filter((e) => Math.abs(e.deviasiPct) > 25).length ?? 0;

  return (
    <div className="mx-auto max-w-[1600px] space-y-6">
      <PageHeader title="Validasi harga" description="Entri lapangan mengalir dari draf, pengecekan otomatis, validasi koordinator, hingga publikasi. Centang beberapa baris untuk memproses sekaligus; Shift + klik untuk memilih rentang." />
      <motion.div variants={stagger(0.06)} initial="hidden" animate="show" className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {[
          { label: "Menunggu validasi", v: n("DIAJUKAN"), tone: "waspada" as const },
          { label: "Divalidasi", v: n("DIVALIDASI"), tone: "data" as const },
          { label: "Dipublikasi", v: n("DIPUBLIKASI"), tone: "normal" as const },
          { label: "Deviasi > 25%", v: anomali, tone: "defisit" as const },
        ].map((k) => (
          <motion.div key={k.label} variants={fadeUp} className="glass rounded-xl p-4">
            <p className="flex items-center gap-2 text-xs text-muted-foreground"><span className={cn("size-1.5 rounded-full", toneClass[k.tone].dot)} />{k.label}</p>
            <CountUp value={k.v} className="mt-2 block text-2xl font-semibold" />
          </motion.div>
        ))}
      </motion.div>
      <DataTable
        data={data}
        isLoading={isLoading}
        columns={columns}
        getRowId={getId}
        entityLabel="entri"
        bulkActions={actions}
        facets={[FACET_STATUS_ENTRI]}
        initialFilters={status ? [{ id: "status", value: status }] : []}
        initialSorting={[{ id: "dicatatPada", desc: true }]}
        searchPlaceholder="Cari komoditas, pasar, wilayah, atau petugas"
        rowTone={(r) => (Math.abs(r.deviasiPct) > 25 ? "defisit" : Math.abs(r.deviasiPct) > 10 ? "waspada" : undefined)}
        onRowClick={setDetail}
      />
      <Sheet open={!!detail} onOpenChange={(o: boolean) => !o && setDetail(null)}>
        <SheetContent className="glass-strong w-full overflow-y-auto border-0 sm:max-w-md">
          {detail && (
            <>
              <SheetHeader>
                <SheetTitle>{detail.komoditasNama}</SheetTitle>
                <SheetDescription>{detail.pasarNama}, {detail.wilayah}</SheetDescription>
              </SheetHeader>
              <div className="space-y-5 px-4 pb-6">
                <div className="flex items-end justify-between">
                  <div>
                    <p className="num text-3xl font-semibold">{formatRupiah(detail.harga)}<span className="text-base font-normal text-muted-foreground">/{detail.satuan}</span></p>
                    <p className="text-sm text-muted-foreground">Deviasi <span className="num">{formatPersen(detail.deviasiPct, { signed: true })}</span> dari median 7 hari</p>
                  </div>
                  <StatusBadge tone={STATUS_ENTRI[detail.status].tone}>{STATUS_ENTRI[detail.status].label}</StatusBadge>
                </div>
                <div className="grid grid-cols-2 gap-2 text-sm">
                  <div className="rounded-xl bg-white/50 p-3 dark:bg-white/5"><Camera className="size-4 text-muted-foreground" /><p className="mt-1">{detail.adaFoto ? "Foto terlampir" : "Tanpa foto"}</p></div>
                  <div className="rounded-xl bg-white/50 p-3 dark:bg-white/5">{detail.dalamGeofence ? <MapPin className="size-4 text-normal" /> : <MapPinOff className="size-4 text-waspada" />}<p className="mt-1">{detail.dalamGeofence ? "Dalam geofence" : "Di luar geofence"}</p></div>
                </div>
                <div>
                  <p className="mb-3 text-sm font-medium">Alur entri</p>
                  <AlurEntri e={detail} />
                </div>
              </div>
            </>
          )}
        </SheetContent>
      </Sheet>
    </div>
  );
}
