"use client";

import { motion } from "motion/react";
import { Check, TriangleAlert, X } from "lucide-react";
import { useMemo, useState } from "react";
import {
  Sheet,
  SheetBody,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { DataTable, type GroupBy } from "@/components/data-table/data-table";
import {
  HalamanPas,
  IsiPas,
  RingkasanKeputusan,
  type ItemRingkas,
} from "@/components/garda/halaman-pas";
import {
  ActivityTimeline,
  ChecklistProgress,
  InspectorRingkasan,
  InspectorSection,
  MetaGrid,
  MetaItem,
  type TimelineEntry,
} from "@/components/garda/inspector";
import { Istilah } from "@/components/garda/istilah";
import { PageHeader } from "@/components/garda/page-header";
import { PanelGalat } from "@/components/garda/panel-galat";
import { Shimmer } from "@/components/garda/shimmer";
import { STATUS_ENTRI, StatusBadge } from "@/components/garda/status-badge";
import { FilterSelect } from "@/components/master/filter-select";
import { Button } from "@/components/ui/button";
import type { EntriHarga, StatusEntri } from "@/lib/data/types";
import { toneClass } from "@/lib/design/tokens";
import { GLOSARIUM } from "@/lib/glosarium";
import {
  formatAngka,
  formatPersen,
  formatRupiah,
  formatTanggal,
} from "@/lib/format";
import { useTabHalaman } from "@/lib/hooks/use-tab-halaman";
import { fadeUp, stagger } from "@/lib/motion/presets";
import { Camera, MapPin, MapPinOff } from "lucide-react";
import type { HargaParams } from "@/lib/api/harga";
import { MODE_MOCK } from "@/lib/api/client";
import {
  useDetailHarga,
  useEntriHarga,
  useRingkasanHarga,
  useRiwayatStatus,
} from "@/lib/query/hooks";
import { qk } from "@/lib/query/keys";
import { cn } from "@/lib/utils";
import {
  FACET_STATUS_ENTRI,
  useHargaBulkActions,
  useHargaColumns,
} from "./harga-table";
import { LineageEntri } from "./lineage-entri";

const PILIHAN_STATUS = [
  "semua",
  ...(Object.keys(STATUS_ENTRI) as StatusEntri[]),
] as const;
const ALUR: StatusEntri[] = ["DRAF", "DIAJUKAN", "DIVALIDASI", "DIPUBLIKASI"];
const getId = (r: EntriHarga) => r.id;

/** Arti deviasi satu entri dalam kalimat, memakai ambang warna yang sama dengan tabel. */
function artiDeviasi(pct: number) {
  const a = Math.abs(pct);
  if (a === 0) return "Sama dengan median 7 hari pasar ini.";
  const posisi = `${formatPersen(a)} ${pct > 0 ? "di atas" : "di bawah"} median 7 hari pasar ini`;
  if (a > 25) return `${posisi}. Periksa lebih dulu sebelum divalidasi.`;
  if (a > 10) return `${posisi}. Perlu dicermati.`;
  return `${posisi}. Selisihnya kecil.`;
}

/** Timeline alur entri: draf → diajukan → divalidasi → dipublikasi (atau ditolak). */
function AlurEntri({ e }: { e: EntriHarga }) {
  const ditolak = e.status === "DITOLAK";
  const idx = ditolak ? 1 : ALUR.indexOf(e.status);
  const label: Record<StatusEntri, string> = {
    DRAF: "Dicatat agen",
    DIAJUKAN: "Diajukan & cek otomatis",
    DIVALIDASI: "Divalidasi koordinator",
    DIPUBLIKASI: "Dipublikasi ke gold layer",
    DITOLAK: "Ditolak",
  };
  return (
    <motion.ol
      variants={stagger(0.08)}
      initial="hidden"
      animate="show"
      className="relative space-y-4 pl-7"
    >
      <span className="absolute top-2 bottom-2 left-[9px] w-0.5 bg-border" />
      {[...ALUR, ...(ditolak ? (["DITOLAK"] as StatusEntri[]) : [])].map(
        (s, i) => {
          const done = ditolak ? i <= 1 || s === "DITOLAK" : i <= idx;
          const tone = s === "DITOLAK" ? "defisit" : done ? "data" : "netral";
          return (
            <motion.li key={s} variants={fadeUp} className="relative">
              <span
                className={cn(
                  "absolute top-0.5 -left-7 grid size-5 place-items-center rounded-full text-white",
                  done ? toneClass[tone].bg : "bg-navy-300",
                )}
              >
                {s === "DITOLAK" ? (
                  <X className="size-3" />
                ) : done ? (
                  <Check className="size-3" />
                ) : null}
              </span>
              <p
                className={cn(
                  "text-[13px]",
                  done ? "font-medium" : "text-muted-foreground",
                )}
              >
                {label[s]}
              </p>
              {done && s === "DRAF" && (
                <p className="text-xs text-muted-foreground">
                  {e.petugasNama},{" "}
                  <span className="num">
                    {formatTanggal(e.dicatatPada, "d MMM HH.mm")}
                  </span>
                </p>
              )}
              {done && s === "DIAJUKAN" && (
                <p className="text-xs text-muted-foreground">
                  Rentang wajar, outlier, geofence, duplikasi
                </p>
              )}
            </motion.li>
          );
        },
      )}
    </motion.ol>
  );
}

/** Bendera validasi otomatis, lokasi GPS, dan bukti satu entri dari server. */
function BuktiOtomatis({ id }: { id: string }) {
  const { data, isLoading, isError, error } = useDetailHarga(id);
  if (isLoading)
    return (
      <div role="status" aria-label="Memuat cek otomatis">
        <Shimmer className="h-10 w-full" />
      </div>
    );
  if (isError)
    return <PanelGalat judul="Cek otomatis gagal dimuat." error={error} />;
  if (!data) return null;
  return (
    <div className="space-y-3">
      {data.bendera.length > 0 ? (
        <ul className="space-y-1.5">
          {data.bendera.map((b) => (
            <li
              key={b.kode}
              className="flex items-start gap-2 rounded-md bg-waspada-soft px-3 py-2 text-[13px] text-waspada-ink"
            >
              <TriangleAlert className="mt-0.5 size-4 shrink-0" />
              {b.label}
            </li>
          ))}
        </ul>
      ) : (
        <p className="rounded-md bg-normal-soft px-3 py-2 text-[13px] text-normal-ink">
          Lolos semua cek otomatis.
        </p>
      )}
      {data.lat !== undefined && data.lng !== undefined && (
        <p className="text-xs text-muted-foreground">
          Lokasi GPS{" "}
          <span className="num">
            {data.lat.toFixed(5)}, {data.lng.toFixed(5)}
          </span>
          {data.jarakMeter ? (
            <> ({data.jarakMeter.toLocaleString("id-ID")} m dari pasar)</>
          ) : null}{" "}
          <a
            className="text-data-700 underline-offset-4 hover:underline"
            href={`https://www.openstreetmap.org/?mlat=${data.lat}&mlon=${data.lng}#map=17/${data.lat}/${data.lng}`}
            target="_blank"
            rel="noreferrer"
          >
            Buka peta
          </a>
        </p>
      )}
      {data.median ? (
        <p className="text-xs text-muted-foreground">
          Pembanding <span className="num">{formatRupiah(data.median)}</span>
        </p>
      ) : null}
    </div>
  );
}

/** Linimasa perpindahan status dari server: siapa, kapan, dan alasannya. */
function RiwayatEntri({ id }: { id: string }) {
  const { data, isLoading, isError, error } = useRiwayatStatus(id);
  if (isLoading)
    return (
      <div role="status" aria-label="Memuat riwayat" className="space-y-3">
        <Shimmer className="h-10 w-full" />
        <Shimmer className="h-10 w-full" />
      </div>
    );
  if (isError)
    return <PanelGalat judul="Riwayat status gagal dimuat." error={error} />;
  const entries: TimelineEntry[] = (data ?? []).map((h, i) => ({
    id: `${h.ke}-${h.waktu}-${i}`,
    title: STATUS_ENTRI[h.ke].label,
    meta: `${h.olehNama ?? "Sistem"}, ${formatTanggal(h.waktu, "d MMM HH.mm")}`,
    note: h.alasan ?? undefined,
    tone: h.ke === "DITOLAK" ? "defisit" : "data",
  }));
  return (
    <ActivityTimeline entries={entries} empty="Belum ada riwayat status." />
  );
}

/** Urutan tampil grup status: yang butuh tindakan lebih dulu. */
const URUTAN_GRUP: StatusEntri[] = [
  "DIAJUKAN",
  "DIVALIDASI",
  "DRAF",
  "DITOLAK",
  "DIPUBLIKASI",
];
const GRUP_STATUS: GroupBy<EntriHarga> = {
  key: (r) => r.status,
  order: URUTAN_GRUP,
  label: (k) => STATUS_ENTRI[k]?.label ?? k,
  tone: (k) => STATUS_ENTRI[k]?.tone,
};

/**
 * Grup tabel dibentuk per halaman, jadi baris harus sudah terurut sebelum dipaginasi agar yang butuh tindakan
 * berada di halaman pertama. Di dalam status: deviasi terbesar, lalu terbaru.
 */
const urutPrioritas = (rows: EntriHarga[]) =>
  [...rows].sort(
    (a, b) =>
      URUTAN_GRUP.indexOf(a.status) - URUTAN_GRUP.indexOf(b.status) ||
      Math.abs(b.deviasiPct) - Math.abs(a.deviasiPct) ||
      Date.parse(b.dicatatPada) - Date.parse(a.dicatatPada),
  );

export function HargaPage() {
  const [pilihan, setPilihan] = useTabHalaman<StatusEntri | "semua">(
    PILIHAN_STATUS,
    "semua",
    "status",
  );
  const status = pilihan === "semua" ? undefined : pilihan;
  const query = useMemo<HargaParams>(
    () => (status ? { status } : {}),
    [status],
  );
  const { data, isLoading } = useEntriHarga(query);
  const baris = useMemo(() => data && urutPrioritas(data), [data]);
  const { data: ringkasan } = useRingkasanHarga();
  const columns = useHargaColumns();
  const actions = useHargaBulkActions(qk.harga.list(query));
  const [detail, setDetail] = useState<EntriHarga | null>(null);

  const selStatus = (
    s: StatusEntri,
    label: string,
    jumlah: number | undefined,
    keterangan: string,
    arti: string,
  ): ItemRingkas => ({
    label,
    nilai: jumlah === undefined ? "–" : `${formatAngka(jumlah)} entri`,
    keterangan,
    tone: STATUS_ENTRI[s].tone,
    arti,
    aktif: status === s,
    onClick: () => setPilihan(status === s ? "semua" : s),
  });
  const itemRingkasan: ItemRingkas[] = [
    selStatus(
      "DIAJUKAN",
      "Menunggu validasi",
      ringkasan?.diajukan,
      "Perlu diperiksa koordinator",
      "Entri 7 hari terakhir yang belum divalidasi. Klik untuk menampilkan semua entri berstatus ini.",
    ),
    selStatus(
      "DIVALIDASI",
      "Siap dipublikasi",
      ringkasan?.divalidasi,
      "Sudah divalidasi",
      "Entri 7 hari terakhir yang sudah divalidasi dan menunggu publikasi. Klik untuk menyaring.",
    ),
    selStatus(
      "DITOLAK",
      "Ditolak",
      ringkasan?.ditolak,
      "Dikembalikan ke petugas",
      "Entri 7 hari terakhir yang ditolak untuk survei ulang. Klik untuk menyaring.",
    ),
    selStatus(
      "DIPUBLIKASI",
      "Dipublikasikan",
      ringkasan?.dipublikasi,
      "Sudah terbit",
      "Entri 7 hari terakhir yang sudah dipublikasikan. Klik untuk menyaring.",
    ),
    {
      label: "Deviasi di atas 25%",
      nilai:
        ringkasan === undefined
          ? "–"
          : `${formatAngka(ringkasan.anomali)} entri`,
      keterangan: "Jauh dari median pasar",
      tone: "defisit",
      arti: GLOSARIUM.deviasi,
    },
  ];

  const alur = ALUR.map((st) => ({
    id: st,
    label: STATUS_ENTRI[st].label,
    done: detail
      ? detail.status === "DITOLAK"
        ? st === "DRAF" || st === "DIAJUKAN"
        : ALUR.indexOf(st) <= ALUR.indexOf(detail.status)
      : false,
  }));

  return (
    <HalamanPas>
      <PageHeader
        title="Validasi harga"
        description="Centang entri untuk memvalidasi, menolak, atau memublikasikan sekaligus; klik baris untuk melihat buktinya."
      />
      <RingkasanKeputusan
        items={itemRingkasan}
        label="Ringkasan validasi harga, 7 hari terakhir"
      />
      <IsiPas>
        <p className="shrink-0 text-xs text-muted-foreground">
          <Istilah kunci="deviasi">Deviasi</Istilah>: selisih harga terhadap
          median 7 hari di pasar yang sama.{" "}
          <span className="font-medium text-waspada-ink">
            Di atas 10% perlu dicermati
          </span>
          ,{" "}
          <span className="font-medium text-defisit-ink">
            di atas 25% periksa lebih dulu
          </span>
          . Ringkasan: 7 hari terakhir.
        </p>
        <DataTable
          key={status ?? "semua"}
          isiPenuh
          data={baris}
          isLoading={isLoading}
          columns={columns}
          getRowId={getId}
          entityLabel="entri"
          bulkActions={actions}
          hiddenColumns={["petugasNama"]}
          toolbar={
            <FilterSelect
              label="Status"
              value={status}
              onChange={(v) => setPilihan((v as StatusEntri) ?? "semua")}
              options={FACET_STATUS_ENTRI.options}
            />
          }
          searchPlaceholder="Cari komoditas, pasar, wilayah, atau petugas"
          rowTone={(r) =>
            Math.abs(r.deviasiPct) > 25
              ? "defisit"
              : Math.abs(r.deviasiPct) > 10
                ? "waspada"
                : undefined
          }
          groupBy={GRUP_STATUS}
          onRowClick={setDetail}
          empty={
            status
              ? {
                  title: `Tidak ada entri berstatus ${STATUS_ENTRI[status].label.toLowerCase()}`,
                  description:
                    "Ubah kata kunci, atau tampilkan semua status untuk melihat entri lain.",
                  action: (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setPilihan("semua")}
                    >
                      Tampilkan semua status
                    </Button>
                  ),
                }
              : undefined
          }
        />
      </IsiPas>
      <Sheet
        open={!!detail}
        onOpenChange={(o: boolean) => !o && setDetail(null)}
      >
        <SheetContent className="sm:max-w-lg">
          {detail && (
            <>
              <SheetHeader>
                <SheetTitle>{detail.komoditasNama}</SheetTitle>
                <SheetDescription>
                  {detail.pasarNama}, {detail.wilayah}
                </SheetDescription>
              </SheetHeader>
              <SheetBody>
                <InspectorRingkasan
                  className="border-b-0"
                  status={
                    <StatusBadge tone={STATUS_ENTRI[detail.status].tone}>
                      {STATUS_ENTRI[detail.status].label}
                    </StatusBadge>
                  }
                  nilai={
                    <>
                      {formatRupiah(detail.harga)}
                      <span className="text-sm font-normal text-muted-foreground">
                        /{detail.satuan}
                      </span>
                    </>
                  }
                  keterangan={artiDeviasi(detail.deviasiPct)}
                />
                <InspectorSection title="Rincian">
                  <MetaGrid>
                    <MetaItem label="Deviasi (median 7 hari)">
                      <span className="num">
                        {formatPersen(detail.deviasiPct, { signed: true })}
                      </span>
                    </MetaItem>
                    <MetaItem label="Petugas">{detail.petugasNama}</MetaItem>
                    <MetaItem label="Dicatat">
                      <span className="num">
                        {formatTanggal(detail.dicatatPada, "d MMM yyyy, HH.mm")}
                      </span>
                    </MetaItem>
                    <MetaItem label="Kualitas / tipe">
                      {detail.kualitas} · {detail.tipe}
                    </MetaItem>
                    <MetaItem label="Foto">
                      <span className="inline-flex items-center gap-1.5">
                        <Camera
                          className="size-3.5 text-muted-foreground"
                          strokeWidth={1.75}
                        />
                        {detail.adaFoto ? "Terlampir" : "Tanpa foto"}
                      </span>
                    </MetaItem>
                    <MetaItem label="Lokasi">
                      {detail.dalamGeofence ? (
                        <span className="inline-flex items-center gap-1.5">
                          <MapPin
                            className="size-3.5 text-normal-ink"
                            strokeWidth={1.75}
                          />
                          Dalam geofence
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1.5">
                          <MapPinOff
                            className="size-3.5 text-waspada-ink"
                            strokeWidth={1.75}
                          />
                          Di luar geofence
                        </span>
                      )}
                    </MetaItem>
                  </MetaGrid>
                  {detail.alasanPenolakan && (
                    <p className="rounded-md bg-defisit-soft p-3 text-[13px] text-defisit-ink">
                      <span className="font-medium">Alasan penolakan:</span>{" "}
                      {detail.alasanPenolakan}
                    </p>
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
                  {MODE_MOCK ? (
                    <AlurEntri e={detail} />
                  ) : (
                    <RiwayatEntri id={detail.id} />
                  )}
                </InspectorSection>
                <InspectorSection title="Asal data">
                  <LineageEntri id={detail.id} />
                </InspectorSection>
              </SheetBody>
            </>
          )}
        </SheetContent>
      </Sheet>
    </HalamanPas>
  );
}
