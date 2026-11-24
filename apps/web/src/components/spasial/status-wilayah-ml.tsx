"use client";

import { PanelPas } from "@/components/garda/halaman-pas";
import { Shimmer } from "@/components/garda/shimmer";
import {
  BadgeSiklusMl,
  KosongMl,
  PeringatanSiklusMl,
} from "@/components/ml/siklus-ml";
import type { BlokWilayahMl, WilayahMl } from "@/lib/api/ml";
import { KABUPATEN } from "@/lib/data/reference";
import { severityMeta, toneClass, type Severity } from "@/lib/design/tokens";
import {
  alasanBelumAndal,
  keadaanBlok,
  LABEL_BELUM_ANDAL,
  LABEL_WILAYAH_TANPA_DATA,
  labelTargetMl,
  namaWilayahMl,
  sinyalTekanan,
} from "@/lib/ml/label";
import {
  useLampiranMl,
  useMlAktif,
  useSiklusMlTerpilih,
} from "@/lib/query/ml-hooks";
import { cn } from "@/lib/utils";

const TINGKAT: { s: Severity; k: "kritis" | "siaga" | "waspada" | "info" }[] = [
  { s: "KRITIS", k: "kritis" },
  { s: "SIAGA", k: "siaga" },
  { s: "WASPADA", k: "waspada" },
  { s: "INFO", k: "info" },
];

/**
 * Status hasil training per wilayah (blok `wilayah` dari `/v1/spasial/wilayah`) untuk siklus di URL: sinyal EWS
 * aktif, komoditas ber-forecast, dan sinyal indeks pangan (jujur soal kelayakan). Wilayah tanpa hasil tidak diberi
 * angka. Status peta (surplus/defisit) tetap dari data PostgreSQL dan tidak diubah.
 */
export function PanelStatusWilayahMl({
  pilih,
  onPilih,
}: {
  pilih?: string;
  onPilih: (kode: string) => void;
}) {
  const aktif = useMlAktif();
  const [cycleId] = useSiklusMlTerpilih();
  const lampiran = useLampiranMl("/v1/spasial/wilayah", {
    cycleId,
    enabled: aktif,
  });
  const k = keadaanBlok(
    lampiran.data,
    lampiran.data?.wilayah,
    lampiran.isLoading,
  );
  if (k.jenis === "sembunyi") return null;
  return (
    <PanelPas
      judul="Sinyal pipeline per wilayah"
      deskripsi="Hasil training, terpisah dari status peta"
      aksi={<BadgeSiklusMl siklus={lampiran.data?.siklus} />}
      tanpaPadding
      className="lg:shrink-0"
    >
      {k.jenis === "memuat" && <Shimmer className="m-3.5 h-24" />}
      {k.jenis === "siklus" && (
        <PeringatanSiklusMl pesan={k.pesan} className="m-3" />
      )}
      {k.jenis === "kosong" && <KosongMl pesan={k.pesan} className="m-3" />}
      {k.jenis === "tampil" && (
        <IsiWilayah blok={k.blok} pilih={pilih} onPilih={onPilih} />
      )}
    </PanelPas>
  );
}

function IsiWilayah({
  blok,
  pilih,
  onPilih,
}: {
  blok: BlokWilayahMl;
  pilih?: string;
  onPilih: (kode: string) => void;
}) {
  const terpilih = pilih
    ? blok.items.find((w) => w.wilayahKode === pilih)
    : undefined;
  const tanpa =
    KABUPATEN.length -
    blok.items.filter((w) => w.tingkat === "KABUPATEN").length;
  return (
    <div>
      {pilih && (
        <div className="border-b border-border px-3.5 py-2.5">
          <p className="text-[13px] font-medium">{namaWilayahMl(pilih)}</p>
          {terpilih ? (
            <RincianWilayah w={terpilih} />
          ) : (
            <p className="mt-0.5 text-xs text-muted-foreground">
              {LABEL_WILAYAH_TANPA_DATA}
            </p>
          )}
        </div>
      )}
      <ul
        aria-label="Wilayah dengan hasil training"
        className="divide-y divide-border"
      >
        {blok.items.map((w) => (
          <li key={w.wilayahKode}>
            <button
              type="button"
              onClick={() => onPilih(w.wilayahKode)}
              aria-current={pilih === w.wilayahKode ? "true" : undefined}
              className={cn(
                "flex w-full items-center gap-2 px-3.5 py-2 text-left text-xs transition-colors hover:bg-secondary/60",
                pilih === w.wilayahKode && "bg-secondary",
              )}
            >
              <span className="min-w-0 flex-1">
                <span className="block truncate text-[13px] font-medium">
                  {namaWilayahMl(w.wilayahKode)}
                </span>
                <span className="block truncate text-muted-foreground">
                  {w.komoditasForecast.length > 0
                    ? `${w.komoditasForecast.length} komoditas ber-forecast`
                    : LABEL_WILAYAH_TANPA_DATA}
                </span>
              </span>
              <HitungSinyal w={w} />
            </button>
          </li>
        ))}
      </ul>
      {tanpa > 0 && (
        <p className="px-3.5 py-2 text-[11px] text-muted-foreground">
          {tanpa} kabupaten/kota lain: {LABEL_WILAYAH_TANPA_DATA.toLowerCase()}.
        </p>
      )}
    </div>
  );
}

function HitungSinyal({ w }: { w: WilayahMl }) {
  const ada = TINGKAT.filter(({ k }) => w.sinyalAktif[k] > 0);
  if (ada.length === 0)
    return <span className="shrink-0 text-muted-foreground">tanpa sinyal</span>;
  return (
    <span className="flex shrink-0 items-center gap-2">
      {ada.map(({ s, k }) => (
        <span
          key={s}
          title={`${w.sinyalAktif[k]} sinyal ${severityMeta[s].label.toLowerCase()}`}
          className="inline-flex items-center gap-1"
        >
          <span
            aria-hidden
            className={cn(
              "size-1.5 rounded-full",
              toneClass[severityMeta[s].tone].dot,
            )}
          />
          <span className="num">{w.sinyalAktif[k]}</span>
          <span className="sr-only">{severityMeta[s].label}</span>
        </span>
      ))}
    </span>
  );
}

function RincianWilayah({ w }: { w: WilayahMl }) {
  const inf = w.inflasi;
  const s = inf
    ? sinyalTekanan({
        kelayakan: inf.kelayakan,
        alasanKelayakan: null,
        kelas: inf.kelas,
        skorTekanan: inf.skorTekanan,
        siklus: inf.siklus,
      })
    : null;
  return (
    <dl className="mt-1.5 space-y-1 text-xs">
      <div className="flex justify-between gap-2">
        <dt className="text-muted-foreground">Sinyal EWS aktif</dt>
        <dd className="num">{w.sinyalAktif.aktif}</dd>
      </div>
      <div className="flex justify-between gap-2">
        <dt className="text-muted-foreground">Forecast</dt>
        <dd>
          {w.komoditasForecast.length > 0
            ? `${w.komoditasForecast.length} komoditas`
            : LABEL_WILAYAH_TANPA_DATA}
        </dd>
      </div>
      {inf && s && (
        <div>
          <dt className="text-muted-foreground">{labelTargetMl(inf.target)}</dt>
          <dd className="mt-0.5">
            {s.andal ? (
              <>
                <span className="font-medium">{s.judul}</span>{" "}
                <span className="text-muted-foreground">({s.keterangan})</span>
              </>
            ) : (
              <>
                <span className="font-medium">{LABEL_BELUM_ANDAL}.</span>{" "}
                <span className="text-muted-foreground">
                  {alasanBelumAndal(inf.kelayakan)}
                </span>
              </>
            )}
          </dd>
        </div>
      )}
    </dl>
  );
}
