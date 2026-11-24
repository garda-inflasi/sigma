"use client";

import { MapPinOff } from "lucide-react";
import { EmptyState } from "@/components/garda/empty-state";
import { IsiPas, PanelPas } from "@/components/garda/halaman-pas";
import { Istilah } from "@/components/garda/istilah";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import {
  BadgeSiklusMl,
  KosongMl,
  PeringatanSiklusMl,
} from "@/components/ml/siklus-ml";
import { toFilterQuery } from "@/lib/api/dashboard";
import type {
  BlokEwsMl,
  BlokForecastRingkasMl,
  ForecastMl,
  LampiranMl,
} from "@/lib/api/ml";
import { KABUPATEN } from "@/lib/data/reference";
import { severityMeta, toneClass } from "@/lib/design/tokens";
import { formatPersen, formatRupiah } from "@/lib/format";
import {
  akhiranSatuan,
  hariIniWib,
  HORIZON_ML,
  ringkasForecastPerWilayah,
  titikHorizon,
} from "@/lib/ml/forecast";
import {
  keadaanBlok,
  LABEL_PERSISTENSI,
  LABEL_WILAYAH_TANPA_DATA,
  labelModelForecast,
  namaKomoditasMl,
  namaWilayahMl,
  tanggalBisnis,
} from "@/lib/ml/label";
import {
  useLampiranMl,
  useMlAktif,
  useSiklusMlTerpilih,
} from "@/lib/query/ml-hooks";
import { cn } from "@/lib/utils";
import { useFilterOperasional } from "./use-filter-operasional";

const KEPALA =
  "sticky top-0 z-10 border-b border-border bg-secondary px-3 py-2 text-left text-xs font-medium text-muted-foreground";
const SEL = "border-b border-border px-3 py-2 align-top";

/**
 * Lampiran `ml` dashboard operasional untuk filter aktif (kabupaten, komoditas) dan siklus di URL. Rute KPI dipakai
 * karena `data`-nya paling ringan; blok `forecast_ringkas` dan `ews` sama untuk semua rute operasional.
 */
export function useOperasionalMl(enabled = true) {
  const aktif = useMlAktif();
  const { saringan } = useFilterOperasional();
  const [cycleId] = useSiklusMlTerpilih();
  return useLampiranMl(
    `/v1/dashboard/operasional/kpi${toFilterQuery(saringan)}`,
    { cycleId, enabled: aktif && enabled },
  );
}

/** Tab proyeksi operasional tampil bila lampiran tersedia (bukan karena Mongo mati, mode staging, atau tanpa izin). */
export function adaProyeksiOps(lampiran: LampiranMl | undefined): boolean {
  return keadaanBlok(lampiran, lampiran?.forecastRingkas).jenis !== "sembunyi";
}

/** Ringkasan forecast dan kesegaran harga per wilayah, plus sinyal EWS pipeline ringkas. */
export function TabProyeksiOpsMl({
  lampiran,
  memuat,
}: {
  lampiran: LampiranMl | undefined;
  memuat: boolean;
}) {
  const { saringan } = useFilterOperasional();
  const fr = keadaanBlok(lampiran, lampiran?.forecastRingkas, memuat);
  const ews = keadaanBlok(lampiran, lampiran?.ews, memuat);
  return (
    <>
      {fr.jenis === "siklus" && (
        <PeringatanSiklusMl pesan={fr.pesan} className="shrink-0" />
      )}
      <IsiPas kolom="lg:grid-cols-[minmax(0,1fr)_20rem]">
        <PanelPas
          judul={
            saringan.komoditasKode
              ? `Proyeksi ${namaKomoditasMl(saringan.komoditasKode)} per wilayah`
              : "Proyeksi dan kesegaran harga per wilayah"
          }
          deskripsi="P50 hasil training; kesegaran dari tanggal harga asli terakhir"
          aksi={<BadgeSiklusMl siklus={lampiran?.siklus} />}
          tanpaPadding
          isiClassName="overflow-x-auto"
        >
          {fr.jenis === "memuat" ? (
            <Shimmer className="m-3.5 h-48" />
          ) : fr.jenis === "tampil" ? (
            saringan.komoditasKode ? (
              <TabelKomoditas blok={fr.blok} />
            ) : (
              <TabelWilayah blok={fr.blok} />
            )
          ) : fr.jenis === "kosong" &&
            (fr.alasan === "WILAYAH_TANPA_DATA" ||
              fr.alasan === "LUAR_CAKUPAN") ? (
            <EmptyState
              icon={MapPinOff}
              title={
                fr.alasan === "LUAR_CAKUPAN"
                  ? "Wilayah ini di luar cakupan tugas Anda"
                  : LABEL_WILAYAH_TANPA_DATA
              }
              description={
                fr.blok && fr.blok.wilayahTersedia.length > 0
                  ? `Siklus ini punya forecast untuk ${fr.blok.wilayahTersedia.map(namaWilayahMl).join(", ")}.`
                  : undefined
              }
              className="py-10"
            />
          ) : (
            <KosongMl
              pesan={
                fr.jenis === "kosong"
                  ? fr.pesan
                  : "Hasil training belum dapat ditampilkan."
              }
              className="m-3.5"
            />
          )}
        </PanelPas>
        <PanelPas
          judul="Sinyal EWS pipeline"
          deskripsi="Ringkas, paling parah dahulu"
          tanpaPadding
        >
          {ews.jenis === "tampil" ? (
            <SinyalRingkas blok={ews.blok} />
          ) : ews.jenis === "memuat" ? (
            <Shimmer className="m-3.5 h-32" />
          ) : (
            <KosongMl
              pesan={
                ews.jenis === "kosong"
                  ? ews.pesan
                  : "Sinyal EWS pipeline tidak tersedia untuk Anda."
              }
              className="m-3.5"
            />
          )}
        </PanelPas>
      </IsiPas>
    </>
  );
}

function Kesegaran({ umurHari }: { umurHari: number | null }) {
  if (umurHari === null)
    return (
      <StatusBadge tone="netral" className="h-5 text-[11px]">
        Tidak diketahui
      </StatusBadge>
    );
  const tone = umurHari <= 2 ? "normal" : umurHari <= 7 ? "waspada" : "defisit";
  return (
    <StatusBadge tone={tone} className="h-5 text-[11px]">
      {umurHari === 0 ? "Hari ini" : `${umurHari} hari lalu`}
    </StatusBadge>
  );
}

function TabelWilayah({ blok }: { blok: BlokForecastRingkasMl }) {
  const rows = ringkasForecastPerWilayah(blok.items, hariIniWib());
  const ada = new Set(rows.map((r) => r.kabupatenKode));
  const tanpa = blok.kabupatenKode
    ? []
    : KABUPATEN.filter((k) => !ada.has(k.kode));
  return (
    <>
      <table className="w-full min-w-[620px] border-separate border-spacing-0 text-[13px]">
        <thead>
          <tr>
            <th className={KEPALA}>Wilayah</th>
            <th className={KEPALA}>Harga asli terakhir</th>
            <th className={cn(KEPALA, "text-right")}>
              <Istilah kunci="rentangKuantil">Median Δ P50 14 hari</Istilah>
            </th>
            <th className={KEPALA}>Naik terbesar</th>
            <th className={KEPALA}>Model</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.kabupatenKode} className="hover:bg-secondary/60">
              <td className={SEL}>
                <p className="font-medium">{namaWilayahMl(r.kabupatenKode)}</p>
                <p className="text-xs text-muted-foreground">
                  {r.jumlahKomoditas} komoditas
                </p>
              </td>
              <td className={SEL}>
                <p className="text-xs">{tanggalBisnis(r.tanggalTerakhir)}</p>
                <Kesegaran umurHari={r.umurHari} />
              </td>
              <td className={cn(SEL, "num text-right font-semibold")}>
                {r.medianPct === null
                  ? "–"
                  : formatPersen(r.medianPct, { signed: true })}
              </td>
              <td className={cn(SEL, "text-xs")}>
                {r.naikTerbesar ? (
                  <>
                    {namaKomoditasMl(r.naikTerbesar.komoditasKode)}{" "}
                    <span className="num font-semibold text-defisit-ink dark:text-rose-300">
                      {formatPersen(r.naikTerbesar.pct, { signed: true })}
                    </span>
                  </>
                ) : (
                  <span className="text-muted-foreground">
                    Tidak ada yang naik
                  </span>
                )}
              </td>
              <td className={cn(SEL, "text-xs")}>
                {r.jumlahPersistensi === r.jumlahKomoditas ? (
                  <span title={LABEL_PERSISTENSI}>
                    <Istilah kunci="persistensi">Persistensi</Istilah> (semua{" "}
                    {r.jumlahKomoditas})
                  </span>
                ) : (
                  `${r.jumlahKomoditas - r.jumlahPersistensi} model terlatih, ${r.jumlahPersistensi} persistensi`
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {tanpa.length > 0 && (
        <p
          title={tanpa.map((k) => k.nama).join(", ")}
          className="px-3 py-2 text-xs text-muted-foreground"
        >
          {tanpa.length} wilayah lain: {LABEL_WILAYAH_TANPA_DATA.toLowerCase()}.
        </p>
      )}
    </>
  );
}

function TabelKomoditas({ blok }: { blok: BlokForecastRingkasMl }) {
  const items = [...blok.items].sort((a, b) =>
    a.kabupatenKode.localeCompare(b.kabupatenKode),
  );
  return (
    <table className="w-full min-w-[620px] border-separate border-spacing-0 text-[13px]">
      <thead>
        <tr>
          <th className={KEPALA}>Wilayah</th>
          <th className={KEPALA}>Harga asli terakhir</th>
          {HORIZON_ML.map((h) => (
            <th key={h} className={cn(KEPALA, "text-right")}>
              P50 H+{h}
            </th>
          ))}
          <th className={KEPALA}>Model</th>
        </tr>
      </thead>
      <tbody>
        {items.map((f: ForecastMl) => {
          const s = akhiranSatuan(f.satuan);
          return (
            <tr key={f.kabupatenKode} className="hover:bg-secondary/60">
              <td className={cn(SEL, "font-medium")}>
                {namaWilayahMl(f.kabupatenKode)}
              </td>
              <td className={cn(SEL, "text-xs")}>
                <span className="num">
                  {f.hargaTerakhir === null
                    ? "–"
                    : `${formatRupiah(f.hargaTerakhir)}${s}`}
                </span>
                <span className="block text-muted-foreground">
                  {tanggalBisnis(f.tanggalTerakhir)}
                </span>
              </td>
              {HORIZON_ML.map((h) => {
                const t = titikHorizon(f.kurva, h);
                return (
                  <td key={h} className={cn(SEL, "num text-right")}>
                    {t?.p50 == null ? "–" : formatRupiah(t.p50)}
                    <span className="block text-xs text-muted-foreground">
                      {t?.perubahanP50Pct == null
                        ? ""
                        : formatPersen(t.perubahanP50Pct, { signed: true })}
                    </span>
                  </td>
                );
              })}
              <td className={cn(SEL, "text-xs")}>
                {labelModelForecast(f.model)}
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

function SinyalRingkas({ blok }: { blok: BlokEwsMl }) {
  const r = blok.ringkasan;
  return (
    <div>
      <p className="border-b border-border px-3.5 py-2 text-xs text-muted-foreground">
        <span className="num font-medium text-foreground">{r.aktif}</span>{" "}
        aktif: {r.kritis} kritis, {r.siaga} siaga, {r.waspada} waspada, {r.info}{" "}
        info
      </p>
      <ul className="divide-y divide-border">
        {blok.sinyal.map((s) => {
          const m = severityMeta[s.severity];
          return (
            <li key={s.sinyalKunci} className="px-3.5 py-2 text-xs">
              <p className="flex items-center gap-1.5">
                <span
                  aria-hidden
                  className={cn("size-1.5 rounded-full", toneClass[m.tone].dot)}
                />
                <span className={cn("font-medium", toneClass[m.tone].text)}>
                  {m.label}
                </span>
                <span className="truncate text-muted-foreground">
                  {s.kabupatenKode
                    ? namaWilayahMl(s.kabupatenKode)
                    : "Tingkat sistem"}
                </span>
              </p>
              <p title={s.pesan} className="mt-0.5 truncate text-[13px]">
                {s.judul}
              </p>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
