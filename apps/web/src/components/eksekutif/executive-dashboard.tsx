"use client";

import { BrainCircuit, Download, LayoutDashboard, Scale } from "lucide-react";
import { useState } from "react";
import {
  HalamanPas,
  IsiPas,
  KOLOM_PAS,
  RingkasanKeputusan,
  type ItemRingkas,
} from "@/components/garda/halaman-pas";
import { PageHeader, PanelTitle } from "@/components/garda/page-header";
import { PanelSkeleton, Shimmer } from "@/components/garda/shimmer";
import { SpringLink } from "@/components/garda/spring-button";
import { ViewTabs, type ViewTab } from "@/components/garda/view-tabs";
import { useCan } from "@/lib/auth/session-context";
import type { RingkasanEksekutif, SumberAngka } from "@/lib/data/types";
import { severityMeta } from "@/lib/design/tokens";
import { GLOSARIUM } from "@/lib/glosarium";
import { formatAngka, formatPersen, formatRupiah } from "@/lib/format";
import { useTabHalaman } from "@/lib/hooks/use-tab-halaman";
import { useRingkasanEksekutif } from "@/lib/query/hooks";
import { useUi } from "@/lib/stores/ui-store";
import { cn } from "@/lib/utils";
import {
  EwsSummary,
  InterventionSummary,
  LatestBrief,
  PenyumbangUtama,
} from "./exec-side";
import { Segmented } from "@/components/garda/segmented";
import { PemilihSiklusMl } from "@/components/ml/siklus-ml";
import {
  TabProyeksiMl,
  usePrediksiMl,
  KABUPATEN_ML_BAWAAN,
} from "@/components/prediksi/prediksi-ml";
import { KOMODITAS } from "@/lib/data/reference";
import { keadaanBlok } from "@/lib/ml/label";
import {
  useLampiranMl,
  useMlAktif,
  useSiklusMlTerpilih,
} from "@/lib/query/ml-hooks";
import { FiscalSpace } from "./fiscal-space";
import { FoodBalance } from "./food-balance";
import { InflationHero } from "./inflation-hero";
import { PanelTekananMl } from "./tekanan-ml";
import { XaiPanel } from "./xai-panel";

const KOMODITAS_PILOT = KOMODITAS.filter((k) => k.pilot);

type TabEksekutif = "ringkasan" | "proyeksi" | "neraca";

const TAB_PENUH: readonly TabEksekutif[] = ["ringkasan", "proyeksi", "neraca"];
const TAB_TANPA_PROYEKSI: readonly TabEksekutif[] = ["ringkasan", "neraca"];

/** Panel kosong: blok yang datanya belum ada, bukan galat. */
function Kosong({ judul, pesan }: { judul: string; pesan: string }) {
  return (
    <section className="glass w-full rounded-lg p-3.5">
      <PanelTitle title={judul} description={pesan} />
    </section>
  );
}

/** Blok ringkasan beserta nama tampilnya; sumber DEMO = contoh, selain itu angka resmi. */
function blokBerSumber(d: RingkasanEksekutif) {
  return [
    { nama: "inflasi", sumber: d.inflasi?.sumber },
    { nama: "neraca pangan", sumber: d.neraca?.sumber },
    { nama: "ruang fiskal", sumber: d.fiskal?.sumber },
  ].filter((b): b is { nama: string; sumber: SumberAngka } => !!b.sumber);
}
const blokDemo = (d: RingkasanEksekutif) =>
  blokBerSumber(d)
    .filter((b) => b.sumber === "DEMO")
    .map((b) => `angka ${b.nama}`);
const blokResmi = (d: RingkasanEksekutif) =>
  blokBerSumber(d)
    .filter((b) => b.sumber !== "DEMO")
    .map((b) => `${b.nama} (${b.sumber})`);

const LABEL_FISKAL = {
  MEMADAI: "Memadai",
  TERBATAS: "Terbatas",
  KRITIS: "Kritis",
} as const;
const TONE_FISKAL = {
  MEMADAI: "normal",
  TERBATAS: "waspada",
  KRITIS: "defisit",
} as const;

/** Jawaban satu pandangan, seluruhnya dihitung dari data ringkasan yang sudah dimuat. */
function itemKeputusan(
  d: RingkasanEksekutif | undefined,
  {
    lihatFiskal,
    lihatEws,
    bukaNeraca,
  }: { lihatFiskal: boolean; lihatEws: boolean; bukaNeraca: () => void },
): ItemRingkas[] {
  const inf = d?.inflasi;
  const dalam =
    !!inf && inf.yoy >= inf.sasaran.bawah && inf.yoy <= inf.sasaran.atas;
  const pendorong = inf?.penyumbang
    .filter((p) => p.andil > 0)
    .sort((a, b) => b.andil - a.andil)[0];
  const terburuk = d?.neraca?.items
    .map((i) => ({
      nama: i.nama,
      rasio: i.produksi / i.kebutuhan - 1,
      selisih: i.produksi - i.kebutuhan,
    }))
    .sort((a, b) => a.rasio - b.rasio)[0];
  const adaDefisit = !!terburuk && terburuk.selisih < 0;
  const mendesak = d ? d.ews.KRITIS + d.ews.SIAGA : 0;

  const items: ItemRingkas[] = [
    {
      label: "Inflasi vs sasaran",
      nilai: !d
        ? "–"
        : !inf
          ? "Belum ada"
          : dalam
            ? "Dalam koridor"
            : "Di luar koridor",
      keterangan: inf
        ? `${formatPersen(inf.yoy, { digits: 2 })}, sasaran ${formatAngka(inf.sasaran.bawah, 1)}–${formatAngka(inf.sasaran.atas, 1)}%`
        : "Menunggu impor dari BPS",
      tone: !inf ? "netral" : dalam ? "normal" : "defisit",
      arti: GLOSARIUM.inflasi,
    },
    {
      label: "Pendorong terbesar",
      nilai: !d ? "–" : (pendorong?.nama ?? "Belum ada"),
      keterangan: pendorong
        ? `+${pendorong.andil.toFixed(2).replace(".", ",")} poin ke inflasi`
        : "Belum ada andil positif",
      tone: pendorong ? "waspada" : "netral",
      arti: "Komoditas dengan andil terbesar terhadap kenaikan inflasi bulan ini.",
    },
    {
      label: "Defisit terbesar",
      nilai: !d
        ? "–"
        : !terburuk
          ? "Belum ada"
          : adaDefisit
            ? terburuk.nama
            : "Tidak ada",
      keterangan: adaDefisit
        ? `Kurang ${formatAngka(Math.abs(terburuk.selisih))} ton/bulan`
        : terburuk
          ? "Semua komoditas mencukupi"
          : "Belum ada data neraca",
      tone: !terburuk ? "netral" : adaDefisit ? "defisit" : "normal",
      arti: GLOSARIUM.neraca,
      onClick: bukaNeraca,
    },
    {
      label: "Peringatan mendesak",
      nilai: !d ? "–" : `${formatAngka(mendesak)} peringatan`,
      keterangan: d
        ? `${formatAngka(d.ews.KRITIS)} kritis, ${formatAngka(d.ews.SIAGA)} siaga`
        : undefined,
      tone: !d
        ? "netral"
        : d.ews.KRITIS > 0
          ? severityMeta.KRITIS.tone
          : d.ews.SIAGA > 0
            ? severityMeta.SIAGA.tone
            : "normal",
      arti: GLOSARIUM.kritis,
      href: lihatEws ? "/ews" : undefined,
    },
  ];

  if (lihatFiskal && d?.fiskal) {
    const f = d.fiskal;
    items.push({
      label: "Sisa ruang fiskal",
      nilai: formatRupiah(f.pagu - f.realisasi, { compact: true }),
      keterangan: `${LABEL_FISKAL[f.status]}, ${formatPersen((f.realisasi / f.pagu) * 100, { digits: 0 })} terealisasi`,
      tone: TONE_FISKAL[f.status],
      arti: "Sisa pagu belanja pengendalian inflasi yang masih bisa dipakai.",
      onClick: bukaNeraca,
    });
  }
  return items;
}

export function ExecutiveDashboard() {
  const { data, isLoading } = useRingkasanEksekutif();
  const lingkunganData = useUi((s) => s.lingkunganData);
  const mlInfo = useUi((s) => s.mlInfo);
  const [kode, setKode] = useState("CBM-KRT");
  const lihatFiskal = useCan("fiskal.lihat");
  const lihatPrediksi = useCan("prediksi.lihat");
  const lihatBrief = useCan("brief.lihat");
  const lihatEws = useCan("ews.lihat");
  const [tab, setTab] = useTabHalaman<TabEksekutif>(
    lihatPrediksi ? TAB_PENUH : TAB_TANPA_PROYEKSI,
    "ringkasan",
  );
  // Hasil training (mode Prod): sinyal tekanan dan kesegaran dari lampiran `ml` endpoint ini, untuk siklus pilihan.
  const mlAktif = useMlAktif();
  const [cycleId] = useSiklusMlTerpilih();
  const lampiran = useLampiranMl("/v1/dashboard/eksekutif", {
    cycleId,
    enabled: mlAktif,
  });
  const kInflasi = keadaanBlok(
    lampiran.data,
    lampiran.data?.inflasi,
    lampiran.isLoading,
  );
  const kMakro = keadaanBlok(lampiran.data, lampiran.data?.makro);
  const kWilayah = keadaanBlok(lampiran.data, lampiran.data?.wilayah);
  const kKesegaran = keadaanBlok(lampiran.data, lampiran.data?.kesegaran);
  const [kabupatenMl, setKabupatenMl] = useState(KABUPATEN_ML_BAWAAN);
  const proyeksiMl = usePrediksiMl(kode, kabupatenMl, tab === "proyeksi");

  const daftarTab: ViewTab<TabEksekutif>[] = [
    { value: "ringkasan", label: "Ringkasan", icon: <LayoutDashboard /> },
    ...(lihatPrediksi
      ? [
          {
            value: "proyeksi" as const,
            label: "Proyeksi",
            icon: <BrainCircuit />,
          },
        ]
      : []),
    {
      value: "neraca",
      label: lihatFiskal ? "Neraca & fiskal" : "Neraca",
      icon: <Scale />,
    },
  ];

  const pesanDemo =
    lingkunganData === "staging" && data?.demo
      ? `Data demo: ${blokDemo(data).join(", ")} di halaman ini adalah contoh, bukan angka resmi BPS, BI, atau DJPb.${blokResmi(data).length > 0 ? ` Angka resmi: ${blokResmi(data).join(", ")}.` : ""}`
      : null;
  const pesanProdKosong =
    lingkunganData === "prod" &&
    !isLoading &&
    kInflasi.jenis === "sembunyi" &&
    kMakro.jenis !== "tampil" &&
    kWilayah.jenis !== "tampil" &&
    (!data?.inflasi || mlInfo?.tersedia === false)
      ? "Mode Prod (Real) aktif — belum ada hasil training MongoDB (ml_predictions) untuk ditampilkan pada blok yang kosong."
      : null;

  return (
    <HalamanPas>
      <PageHeader
        title="Ringkasan Dewan Pengarah"
        description="Kondisi inflasi, komoditas bermasalah, dan keputusan pekan ini."
        tabs={
          <ViewTabs
            layoutId="tab-eksekutif"
            label="Tampilan ringkasan eksekutif"
            value={tab}
            onChange={setTab}
            tabs={daftarTab}
          />
        }
        actions={
          <>
            <PemilihSiklusMl />
            {lihatBrief && (
              <SpringLink href="/policy-brief" variant="soft">
                <Download /> Policy brief terbaru
              </SpringLink>
            )}
          </>
        }
      />

      {pesanDemo && (
        <p
          role="status"
          title={pesanDemo}
          className="shrink-0 rounded-md border border-dashed border-waspada/50 bg-waspada/10 px-3 py-1.5 text-md text-waspada-ink lg:truncate dark:text-amber-300"
        >
          {pesanDemo}
        </p>
      )}
      {pesanProdKosong && (
        <p
          role="status"
          title={pesanProdKosong}
          className="shrink-0 rounded-md border border-dashed border-data-500/40 bg-data-50 px-3 py-1.5 text-xs text-data-800 lg:truncate dark:bg-data-950/40 dark:text-data-200"
        >
          {pesanProdKosong}
        </p>
      )}

      {tab === "ringkasan" && (
        <>
          <RingkasanKeputusan
            label="Ringkasan keputusan"
            className="max-lg:[&>li:last-child:nth-child(odd)]:col-span-2"
            items={itemKeputusan(data, {
              lihatFiskal,
              lihatEws,
              bukaNeraca: () => setTab("neraca"),
            })}
          />
          <IsiPas kolom="lg:grid-cols-12">
            <div className="scrollbar-thin flex min-w-0 flex-col gap-2.5 lg:col-span-8 lg:max-h-full lg:min-h-0 lg:overflow-y-auto">
              {isLoading || !data ? (
                <Shimmer className="h-80 w-full rounded-lg" />
              ) : data.inflasi ? (
                <InflationHero data={data.inflasi} />
              ) : (
                <Kosong
                  judul="Inflasi tahunan"
                  pesan="Belum ada data inflasi bulanan. Angka resmi masuk lewat impor dari BPS."
                />
              )}
              <PanelTekananMl
                inflasi={kInflasi}
                kesegaran={kKesegaran}
                makro={kMakro}
                wilayah={kWilayah}
                cycleId={cycleId}
              />
              {lihatBrief && <LatestBrief />}
            </div>
            <div className="scrollbar-thin flex min-w-0 flex-col gap-2.5 lg:col-span-4 lg:max-h-full lg:min-h-0 lg:overflow-y-auto">
              {data && <EwsSummary data={data.ews} />}
              {data && (
                <InterventionSummary
                  data={data.intervensi}
                  dominan={data.siklusDominan}
                />
              )}
              {data?.inflasi && (
                <PenyumbangUtama data={data.inflasi.penyumbang} />
              )}
            </div>
          </IsiPas>
        </>
      )}

      {tab === "proyeksi" && lihatPrediksi && proyeksiMl.aktif && (
        <TabProyeksiMl
          ml={proyeksiMl}
          kode={kode}
          kabupaten={kabupatenMl}
          onKabupaten={setKabupatenMl}
          onKomoditas={setKode}
          pilihKomoditas={
            <Segmented
              layoutId="xai-komoditas-ml"
              size="sm"
              value={kode}
              onChange={setKode}
              options={KOMODITAS_PILOT.map((k) => ({
                value: k.kode,
                label: k.nama.replace(" keriting", ""),
              }))}
            />
          }
        />
      )}

      {tab === "proyeksi" && lihatPrediksi && !proyeksiMl.aktif && (
        <IsiPas>
          <XaiPanel kode={kode} onKodeChange={setKode} className="lg:flex-1" />
        </IsiPas>
      )}

      {tab === "neraca" && (
        <IsiPas kolom={lihatFiskal ? "lg:grid-cols-12" : undefined}>
          <div className={cn(KOLOM_PAS, lihatFiskal && "lg:col-span-7")}>
            {!data ? (
              <PanelSkeleton />
            ) : data.neraca ? (
              <FoodBalance
                data={data.neraca.items}
                sumber={data.neraca.sumber}
              />
            ) : (
              <Kosong
                judul="Neraca pangan"
                pesan="Belum ada data neraca pangan."
              />
            )}
          </div>
          {lihatFiskal && (
            <div className={cn(KOLOM_PAS, "lg:col-span-5")}>
              {!data ? (
                <PanelSkeleton />
              ) : data.fiskal ? (
                <FiscalSpace data={data.fiskal} />
              ) : (
                <Kosong
                  judul="Ruang fiskal"
                  pesan="Belum ada data ruang fiskal. Angka resmi masuk lewat impor dari DJPb."
                />
              )}
            </div>
          )}
        </IsiPas>
      )}
    </HalamanPas>
  );
}
