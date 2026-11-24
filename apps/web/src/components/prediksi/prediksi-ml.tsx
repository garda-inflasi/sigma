"use client";

import { MapPinOff } from "lucide-react";
import type { ReactNode } from "react";
import { EmptyState } from "@/components/garda/empty-state";
import {
  IsiPas,
  PanelPas,
  RingkasanKeputusan,
  type ItemRingkas,
} from "@/components/garda/halaman-pas";
import { Shimmer } from "@/components/garda/shimmer";
import {
  PanelForecastMl,
  PanelModelSiklusMl,
  PanelPenjelasanMl,
} from "@/components/ml/forecast-ml";
import { KosongMl, PeringatanSiklusMl } from "@/components/ml/siklus-ml";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type {
  BlokForecastMl,
  BlokForecastRingkasMl,
  BlokModelMl,
} from "@/lib/api/ml";
import { useCan } from "@/lib/auth/session-context";
import { KABUPATEN } from "@/lib/data/reference";
import { formatPersen, formatRupiah } from "@/lib/format";
import { GLOSARIUM } from "@/lib/glosarium";
import {
  akhiranSatuan,
  barisDariForecast,
  titikHorizon,
} from "@/lib/ml/forecast";
import {
  keadaanBlok,
  LABEL_WILAYAH_TANPA_DATA,
  labelModelForecast,
  namaKomoditasMl,
  namaWilayahMl,
  tanggalBisnis,
  type KeadaanBlok,
} from "@/lib/ml/label";
import {
  useLampiranMl,
  useMlAktif,
  useSiklusMlTerpilih,
} from "@/lib/query/ml-hooks";
import { AksiTindakLanjut } from "./panel-proyeksi";

/** Kabupaten bawaan forecast (sama dengan bawaan `GET /v1/prediksi`). */
export const KABUPATEN_ML_BAWAAN = "1171";

export interface PrediksiMl {
  /** true bila halaman memakai hasil training (mode Prod, izin, dan Mongo tersedia). */
  aktif: boolean;
  cycleId: string | undefined;
  forecast: KeadaanBlok<BlokForecastMl>;
  ringkas: KeadaanBlok<BlokForecastRingkasMl>;
  model: KeadaanBlok<BlokModelMl>;
  memuatUlang: boolean;
}

/**
 * Hasil training untuk `/prediksi`: detail forecast komoditas terpilih (`/v1/prediksi/:kode`) serta ringkasan semua
 * komoditas dan metrik model (`/v1/prediksi`) di kabupaten terpilih, untuk siklus di URL.
 */
export function usePrediksiMl(
  kode: string,
  kabupaten: string,
  aktifkan = true,
): PrediksiMl {
  const mlAktif = useMlAktif() && aktifkan;
  const boleh = useCan("prediksi.lihat");
  const [cycleId] = useSiklusMlTerpilih();
  const kab = encodeURIComponent(kabupaten);
  const detail = useLampiranMl(
    `/v1/prediksi/${encodeURIComponent(kode)}?kabupaten=${kab}`,
    {
      cycleId,
      enabled: mlAktif && boleh,
    },
  );
  const ringkas = useLampiranMl(`/v1/prediksi?kabupaten=${kab}`, {
    cycleId,
    enabled: mlAktif && boleh,
  });
  const forecast = keadaanBlok(
    detail.data,
    detail.data?.forecast,
    detail.isLoading,
  );
  return {
    aktif: mlAktif && boleh && forecast.jenis !== "sembunyi",
    cycleId,
    forecast,
    ringkas: keadaanBlok(
      ringkas.data,
      ringkas.data?.forecastRingkas,
      ringkas.isLoading,
    ),
    model: keadaanBlok(ringkas.data, ringkas.data?.model, ringkas.isLoading),
    memuatUlang:
      detail.isPlaceholderData || (detail.isFetching && !detail.isLoading),
  };
}

/** Kabupaten yang punya forecast pada siklus pilihan (dari blok detail, atau ringkasan bila detail kosong). */
export function wilayahTersediaMl(ml: PrediksiMl): string[] {
  for (const k of [ml.forecast, ml.ringkas]) {
    if ((k.jenis === "tampil" || k.jenis === "kosong") && k.blok) return k.blok.wilayahTersedia;
  }
  return [];
}

/** Baris tabel komoditas dari siklus pilihan; kosong bila siklus/wilayah tidak punya forecast. */
export function barisKomoditasMl(ml: PrediksiMl) {
  return ml.ringkas.jenis === "tampil"
    ? ml.ringkas.blok.items.map(barisDariForecast)
    : ml.ringkas.jenis === "memuat"
      ? undefined
      : [];
}

/** Kartu ringkasan tab Proyeksi dari hasil training. Tanpa forecast, kartu tidak berisi angka. */
export function itemRingkasanMl(
  ml: PrediksiMl,
  opsi: { bukaKomoditas?: () => void; bukaModel?: () => void },
): ItemRingkas[] {
  const item = ml.forecast.jenis === "tampil" ? ml.forecast.blok.item : null;
  const baris = barisKomoditasMl(ml) ?? [];
  const t14 = item ? titikHorizon(item.kurva, 14) : undefined;
  const pct = t14?.perubahanP50Pct ?? null;
  const satuan = item ? akhiranSatuan(item.satuan) : "";
  const perlu = baris.filter((b) => b.pct > 5).length;
  return [
    {
      label: item
        ? `Proyeksi ${namaKomoditasMl(item.komoditasKode)}`
        : "Proyeksi",
      nilai: pct === null ? "–" : formatPersen(pct, { signed: true }),
      keterangan:
        t14?.p50 != null
          ? `P50 14 hari, ${formatRupiah(t14.p50)}${satuan}`
          : "Belum ada forecast",
      tone:
        pct === null
          ? "netral"
          : pct > 5
            ? "defisit"
            : pct < -3
              ? "surplus"
              : "normal",
      arti: GLOSARIUM.rentangKuantil,
    },
    {
      label: "Harga asli terakhir",
      nilai:
        item?.hargaTerakhir != null ? formatRupiah(item.hargaTerakhir) : "–",
      keterangan: item ? tanggalBisnis(item.tanggalTerakhir) : "Belum ada data",
      tone: "data",
    },
    {
      label: "Perlu tindakan",
      nilai: ml.ringkas.jenis === "tampil" ? `${perlu} komoditas` : "–",
      keterangan: "P50 14 hari naik di atas 5%",
      tone: perlu > 0 ? "defisit" : "normal",
      arti: "Buka daftar semua komoditas, urut dari kenaikan terbesar.",
      onClick: opsi.bukaKomoditas,
    },
    {
      label: "Model pemenang",
      nilai: !item
        ? "–"
        : item.model.adalahBaseline
          ? "Persistensi"
          : labelModelForecast(item.model),
      keterangan: !item
        ? "Belum ada forecast"
        : item.model.adalahBaseline
          ? "Belum ada model yang mengalahkan baseline"
          : `MAPE ${formatPersen(item.metrik.mape ?? 0)} vs baseline ${formatPersen(item.metrik.mapeBaseline ?? 0)}`,
      tone: "netral",
      arti: GLOSARIUM.persistensi,
      onClick: opsi.bukaModel,
    },
  ];
}

/** Pemilih kabupaten forecast; kabupaten tanpa forecast tetap bisa dipilih dan diberi tanda. */
export function PilihWilayahMl({
  value,
  onChange,
  tersedia,
}: {
  value: string;
  onChange: (kode: string) => void;
  tersedia: readonly string[];
}) {
  const ada = new Set(tersedia);
  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger
        aria-label="Pilih wilayah forecast"
        className="h-8 w-full text-[13px] sm:w-48 md:text-[13px]"
      >
        <SelectValue />
      </SelectTrigger>
      <SelectContent className="max-h-72">
        {KABUPATEN.map((k) => (
          <SelectItem key={k.kode} value={k.kode} textValue={k.nama}>
            {k.nama}
            {ada.size > 0 && !ada.has(k.kode) && (
              <span className="text-[11px] text-muted-foreground">
                tanpa forecast
              </span>
            )}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

/** Tab Proyeksi dari hasil training: grafik kuantil + model/XAI, atau keterangan jujur bila tidak ada forecast. */
export function TabProyeksiMl({
  ml,
  kode,
  kabupaten,
  onKabupaten,
  onKomoditas,
  bukaKomoditas,
  bukaModel,
  pilihKomoditas,
}: {
  ml: PrediksiMl;
  kode: string;
  kabupaten: string;
  onKabupaten: (kode: string) => void;
  onKomoditas: (kode: string) => void;
  bukaKomoditas?: () => void;
  bukaModel?: () => void;
  /** Kontrol komoditas tambahan di kepala panel (halaman yang tidak punya pemilih komoditas sendiri). */
  pilihKomoditas?: ReactNode;
}) {
  const f = ml.forecast;
  const tersedia = wilayahTersediaMl(ml);
  const pilihWilayah = (
    <>
      {pilihKomoditas}
      <PilihWilayahMl
        value={kabupaten}
        onChange={onKabupaten}
        tersedia={tersedia}
      />
    </>
  );
  const wilayah = namaWilayahMl(kabupaten);
  const items = itemRingkasanMl(ml, { bukaKomoditas, bukaModel });

  return (
    <>
      {f.jenis === "siklus" && (
        <PeringatanSiklusMl pesan={f.pesan} className="shrink-0" />
      )}
      <RingkasanKeputusan
        items={items}
        label="Ringkasan proyeksi hasil training"
      />
      <IsiPas kolom="lg:grid-cols-[minmax(0,1fr)_22rem]">
        {f.jenis === "memuat" ? (
          <>
            <Shimmer aria-busy className="h-80 w-full rounded-lg lg:h-full" />
            <Shimmer className="h-64 w-full rounded-lg lg:h-full" />
          </>
        ) : f.jenis === "tampil" && f.blok.item ? (
          <>
            <PanelForecastMl
              item={f.blok.item}
              wilayah={wilayah}
              memuatUlang={ml.memuatUlang}
              aksi={pilihWilayah}
              kaki={
                <AksiTindakLanjut
                  nama={namaKomoditasMl(f.blok.item.komoditasKode)}
                  komoditasKode={f.blok.item.komoditasKode}
                  unduh={false}
                />
              }
            />
            <PanelPenjelasanMl item={f.blok.item} cycleId={f.blok.cycleId} />
          </>
        ) : (
          <PanelPas
            judul={`Proyeksi harga ${namaKomoditasMl(kode)}`}
            deskripsi={wilayah}
            aksi={pilihWilayah}
            className="lg:col-span-2"
          >
            <TanpaForecast
              ml={ml}
              kode={kode}
              onKabupaten={onKabupaten}
              onKomoditas={onKomoditas}
            />
          </PanelPas>
        )}
      </IsiPas>
    </>
  );
}

function TanpaForecast({
  ml,
  kode,
  onKabupaten,
}: {
  ml: PrediksiMl;
  kode: string;
  onKabupaten: (kode: string) => void;
  onKomoditas: (kode: string) => void;
}) {
  const f = ml.forecast;
  const alasan = f.jenis === "kosong" ? f.alasan : null;
  const tersedia = wilayahTersediaMl(ml);
  if (alasan === "WILAYAH_TANPA_DATA" || alasan === "LUAR_CAKUPAN") {
    return (
      <EmptyState
        icon={MapPinOff}
        title={
          alasan === "LUAR_CAKUPAN"
            ? "Wilayah ini di luar cakupan tugas Anda"
            : LABEL_WILAYAH_TANPA_DATA
        }
        description={
          tersedia.length > 0
            ? `Forecast ${namaKomoditasMl(kode)} siklus ini hanya ada untuk ${tersedia.map(namaWilayahMl).join(", ")}.`
            : "Siklus ini belum punya forecast untuk komoditas ini."
        }
        action={
          tersedia.length > 0 ? (
            <div className="flex flex-wrap justify-center gap-2">
              {tersedia.map((k) => (
                <button
                  key={k}
                  type="button"
                  onClick={() => onKabupaten(k)}
                  className="rounded-md border border-border px-2.5 py-1 text-xs hover:bg-secondary"
                >
                  {namaWilayahMl(k)}
                </button>
              ))}
            </div>
          ) : undefined
        }
        className="py-10"
      />
    );
  }
  if (f.jenis === "kosong") {
    return (
      <KosongMl
        pesan={`${f.pesan} Pilih siklus lain untuk melihat proyeksi.`}
      />
    );
  }
  return (
    <KosongMl pesan="Hasil training untuk proyeksi ini belum dapat ditampilkan." />
  );
}

/** Tab Model dari hasil training siklus pilihan. */
export function TabModelMl({
  ml,
  kabupaten,
}: {
  ml: PrediksiMl;
  kabupaten: string;
}) {
  if (ml.model.jenis === "memuat")
    return <Shimmer className="h-72 w-full rounded-lg lg:flex-1" />;
  if (ml.model.jenis === "siklus")
    return <PeringatanSiklusMl pesan={ml.model.pesan} className="shrink-0" />;
  if (ml.model.jenis !== "tampil") {
    return (
      <KosongMl
        pesan={
          ml.model.jenis === "kosong"
            ? ml.model.pesan
            : "Metrik model siklus ini belum dapat ditampilkan."
        }
      />
    );
  }
  const items = ml.ringkas.jenis === "tampil" ? ml.ringkas.blok.items : [];
  return (
    <PanelModelSiklusMl
      model={ml.model.blok}
      items={items}
      wilayah={namaWilayahMl(kabupaten)}
    />
  );
}
