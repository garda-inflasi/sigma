"use client";

import { motion } from "motion/react";
import { Check, TriangleAlert, X } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useMemo, useState } from "react";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { DataTable, type GroupBy } from "@/components/data-table/data-table";
import { ActivityTimeline, ChecklistProgress, InspectorSection, MetaGrid, MetaItem, type TimelineEntry } from "@/components/garda/inspector";
import { Stagger, StaggerItem } from "@/components/garda/page-enter";
import { PageHeader } from "@/components/garda/page-header";
import { STATUS_ENTRI, StatusBadge } from "@/components/garda/status-badge";
import { CountUp } from "@/components/motion/count-up";
import type { EntriHarga, StatusEntri } from "@/lib/data/types";
import { toneClass } from "@/lib/design/tokens";
import { formatPersen, formatRupiah, formatTanggal } from "@/lib/format";
import { fadeUp, stagger } from "@/lib/motion/presets";
import { Camera, MapPin, MapPinOff } from "lucide-react";
import type { HargaParams } from "@/lib/api/harga";
import { MODE_MOCK } from "@/lib/api/client";
import { useDetailHarga, useEntriHarga, useRingkasanHarga, useRiwayatStatus } from "@/lib/query/hooks";
import { qk } from "@/lib/query/keys";
import { cn } from "@/lib/utils";
import { FACET_STATUS_ENTRI, useHargaBulkActions, useHargaColumns } from "./harga-table";
import { LineageEntri } from "./lineage-entri";

const STATUS_VALID = Object.keys(STATUS_ENTRI) as StatusEntri[];
const statusDariQuery = (s: string | null) => STATUS_VALID.find((v) => v === s);
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

/** Bendera validasi otomatis, lokasi GPS, dan bukti satu entri dari server. */
function BuktiOtomatis({ id }: { id: string }) {
  const { data } = useDetailHarga(id);
  if (!data) return null;
  return (
    <div className="space-y-3">
      {data.bendera.length > 0 ? (
        <ul className="space-y-1.5">
          {data.bendera.map((b) => (
            <li key={b.kode} className="flex items-start gap-2 rounded-md bg-waspada-soft px-3 py-2 text-[13px] text-waspada-ink">
              <TriangleAlert className="mt-0.5 size-4 shrink-0" />{b.label}
            </li>
          ))}
        </ul>
      ) : (
        <p className="rounded-md bg-normal-soft px-3 py-2 text-[13px] text-normal-ink">Lolos semua cek otomatis.</p>
      )}
      {data.lat !== undefined && data.lng !== undefined && (
        <p className="text-xs text-muted-foreground">
          Lokasi GPS <span className="num">{data.lat.toFixed(5)}, {data.lng.toFixed(5)}</span>
          {data.jarakMeter ? <> ({data.jarakMeter.toLocaleString("id-ID")} m dari pasar)</> : null}
          {" "}<a className="text-data-700 underline-offset-4 hover:underline" href={`https://www.openstreetmap.org/?mlat=${data.lat}&mlon=${data.lng}#map=17/${data.lat}/${data.lng}`} target="_blank" rel="noreferrer">Buka peta</a>
        </p>
      )}
      {data.median ? <p className="text-xs text-muted-foreground">Pembanding <span className="num">{formatRupiah(data.median)}</span></p> : null}
    </div>
  );
}

/** Linimasa perpindahan status dari server: siapa, kapan, dan alasannya. */
function RiwayatEntri({ id }: { id: string }) {
  const { data, isLoading } = useRiwayatStatus(id);
  if (isLoading) return <p className="text-[13px] text-muted-foreground">Memuat riwayat</p>;
  const entries: TimelineEntry[] = (data ?? []).map((h, i) => ({
    id: `${h.ke}-${h.waktu}-${i}`,
    title: STATUS_ENTRI[h.ke].label,
    meta: `${h.olehNama ?? "Sistem"}, ${formatTanggal(h.waktu, "d MMM HH.mm")}`,
    note: h.alasan ?? undefined,
    tone: h.ke === "DITOLAK" ? "defisit" : "data",
  }));
  return <ActivityTimeline entries={entries} empty="Belum ada riwayat status." />;
}

/** Urutan tampil grup status: yang butuh tindakan lebih dulu. */
const URUTAN_GRUP: StatusEntri[] = ["DIAJUKAN", "DIVALIDASI", "DRAF", "DITOLAK", "DIPUBLIKASI"];
const GRUP_STATUS: GroupBy<EntriHarga> = {
  key: (r) => r.status,
  order: URUTAN_GRUP,
  label: (k) => STATUS_ENTRI[k]?.label ?? k,
  tone: (k) => STATUS_ENTRI[k]?.tone,
};

export function HargaPage() {
  const params = useSearchParams();
  const status = statusDariQuery(params.get("status"));
  const query = useMemo<HargaParams>(() => (status ? { status } : {}), [status]);
  const { data, isLoading } = useEntriHarga(query);
  const { data: ringkasan } = useRingkasanHarga();
  const columns = useHargaColumns();
  const actions = useHargaBulkActions(qk.harga.list(query));
  const [detail, setDetail] = useState<EntriHarga | null>(null);

  const alur = ALUR.map((st) => ({ id: st, label: STATUS_ENTRI[st].label, done: detail ? (detail.status === "DITOLAK" ? st === "DRAF" || st === "DIAJUKAN" : ALUR.indexOf(st) <= ALUR.indexOf(detail.status)) : false }));

  return (
    <div className="space-y-5">
      <PageHeader title="Validasi harga" description="Ringkasan menghitung tujuh hari terakhir. Entri dikelompokkan per status; centang beberapa baris untuk memproses sekaligus (Shift + klik untuk rentang), atau klik baris untuk membuka rincian." />
      <Stagger className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {[
          { label: "Menunggu validasi", v: ringkasan?.diajukan ?? 0, tone: "waspada" as const },
          { label: "Divalidasi", v: ringkasan?.divalidasi ?? 0, tone: "data" as const },
          { label: "Dipublikasi", v: ringkasan?.dipublikasi ?? 0, tone: "normal" as const },
          { label: "Deviasi > 25%", v: ringkasan?.anomali ?? 0, tone: "defisit" as const },
        ].map((k) => (
          <StaggerItem key={k.label} className="glass rounded-lg p-4">
            <p className="flex items-center gap-2 text-xs text-muted-foreground"><span className={cn("size-1.5 rounded-full", toneClass[k.tone].dot)} />{k.label}</p>
            <CountUp value={k.v} className="mt-1.5 block text-2xl font-semibold tracking-tight" />
          </StaggerItem>
        ))}
      </Stagger>
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
        groupBy={GRUP_STATUS}
        onRowClick={setDetail}
      />
      <Sheet open={!!detail} onOpenChange={(o: boolean) => !o && setDetail(null)}>
        <SheetContent className="w-full gap-0 overflow-y-auto sm:max-w-[480px]">
          {detail && (
            <>
              <SheetHeader className="gap-1 border-b border-border p-5 pr-12">
                <div className="flex items-center gap-2.5">
                  <SheetTitle className="text-base">{detail.komoditasNama}</SheetTitle>
                  <StatusBadge tone={STATUS_ENTRI[detail.status].tone}>{STATUS_ENTRI[detail.status].label}</StatusBadge>
                </div>
                <SheetDescription>{detail.pasarNama}, {detail.wilayah}</SheetDescription>
                <p className="num pt-1 text-2xl font-semibold tracking-tight">{formatRupiah(detail.harga)}<span className="text-sm font-normal text-muted-foreground">/{detail.satuan}</span></p>
              </SheetHeader>
              <InspectorSection title="Rincian">
                <MetaGrid>
                  <MetaItem label="Deviasi (median 7 hari)"><span className="num">{formatPersen(detail.deviasiPct, { signed: true })}</span></MetaItem>
                  <MetaItem label="Petugas">{detail.petugasNama}</MetaItem>
                  <MetaItem label="Dicatat"><span className="num">{formatTanggal(detail.dicatatPada, "d MMM yyyy, HH.mm")}</span></MetaItem>
                  <MetaItem label="Kualitas / tipe">{detail.kualitas} · {detail.tipe}</MetaItem>
                  <MetaItem label="Foto"><span className="inline-flex items-center gap-1.5"><Camera className="size-3.5 text-muted-foreground" strokeWidth={1.75} />{detail.adaFoto ? "Terlampir" : "Tanpa foto"}</span></MetaItem>
                  <MetaItem label="Lokasi">{detail.dalamGeofence ? <span className="inline-flex items-center gap-1.5"><MapPin className="size-3.5 text-normal-ink" strokeWidth={1.75} />Dalam geofence</span> : <span className="inline-flex items-center gap-1.5"><MapPinOff className="size-3.5 text-waspada-ink" strokeWidth={1.75} />Di luar geofence</span>}</MetaItem>
                </MetaGrid>
                {detail.alasanPenolakan && (
                  <p className="rounded-md bg-defisit-soft p-3 text-[13px] text-defisit-ink"><span className="font-medium">Alasan penolakan:</span> {detail.alasanPenolakan}</p>
                )}
              </InspectorSection>
              <InspectorSection title="Progres alur">
                <ChecklistProgress label="Progres alur entri" items={alur} />
              </InspectorSection>
              {!MODE_MOCK && (
                <InspectorSection title="Cek otomatis">
                  <BuktiOtomatis id={detail.id} />
                </InspectorSection>
              )}
              <InspectorSection title="Aktivitas">
                {MODE_MOCK ? <AlurEntri e={detail} /> : <RiwayatEntri id={detail.id} />}
              </InspectorSection>
              <InspectorSection title="Asal data">
                <LineageEntri id={detail.id} />
              </InspectorSection>
            </>
          )}
        </SheetContent>
      </Sheet>
    </div>
  );
}
