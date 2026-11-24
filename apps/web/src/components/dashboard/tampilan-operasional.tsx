"use client";

import {
  BrainCircuit,
  ClipboardCheck,
  LayoutDashboard,
  LineChart,
  Package,
  Plus,
  Users,
} from "lucide-react";
import type { ReactNode } from "react";
import { TrendChart } from "@/components/charts/trend-chart";
import { EwsLiveFeed } from "@/components/ews/ews-live-feed";
import { HalamanPas, IsiPas, KOLOM_PAS } from "@/components/garda/halaman-pas";
import { PageHeader } from "@/components/garda/page-header";
import { SpringLink } from "@/components/garda/spring-button";
import { ViewTabs, type ViewTab } from "@/components/garda/view-tabs";
import { WidgetErrorBoundary } from "@/components/garda/widget-error-boundary";
import { ValidationQueue } from "@/components/harga/validation-queue";
import { useTabHalaman } from "@/lib/hooks/use-tab-halaman";
import { cn } from "@/lib/utils";
import { FieldActivity } from "./field-activity";
import { CuacaWilayahCard, PembandingHargaCard } from "./hasil-ingestion";
import { KpiGrid } from "./kpi-grid";
import { OpsFilterBar } from "./ops-filter-bar";
import { OpsLogisticsCard } from "./ops-logistics-card";
import { OpsMap } from "./ops-map";
import { OpsProducerCard } from "./ops-producer-card";
import { OpsStockCard } from "./ops-stock-card";
import { OpsWilayahPrioritas } from "./ops-wilayah-prioritas";
import { PriceHeatmap } from "./price-heatmap";
import {
  adaProyeksiOps,
  TabProyeksiOpsMl,
  useOperasionalMl,
} from "./proyeksi-wilayah-ml";

type TabOps =
  | "ringkasan"
  | "harga"
  | "pasokan"
  | "proyeksi"
  | "validasi"
  | "lapangan";

const TAB_PENUH: readonly TabOps[] = [
  "ringkasan",
  "harga",
  "pasokan",
  "validasi",
  "lapangan",
];
const TAB_TANPA_HARGA: readonly TabOps[] = ["ringkasan", "harga", "pasokan"];

/** Sel bento mengikuti tinggi isinya; `penuh` hanya untuk visualisasi yang harus mengisi baris. */
function SelBento({
  judul,
  className,
  penuh,
  children,
}: {
  judul: string;
  className?: string;
  penuh?: boolean;
  children: ReactNode;
}) {
  return (
    <div
      className={cn(
        "flex w-full min-w-0 flex-col lg:max-h-full lg:min-h-0",
        penuh && "lg:h-full",
        className,
      )}
    >
      <WidgetErrorBoundary fallbackTitle={judul} className="h-full">
        {children}
      </WidgetErrorBoundary>
    </div>
  );
}

export function TampilanOperasional({
  lihatPeta,
  lihatEws,
  lihatHarga,
  bisaValidasi,
  bisaInput,
}: {
  lihatPeta: boolean;
  lihatEws: boolean;
  lihatHarga: boolean;
  bisaValidasi: boolean;
  bisaInput: boolean;
}) {
  // Hasil training (mode Prod): tab Proyeksi hanya ada bila lampiran `ml` operasional tersedia untuk pengguna ini.
  const lampiranMl = useOperasionalMl();
  const adaMl = adaProyeksiOps(lampiranMl.data);
  // Antrean dan aktivitas lapangan hanya untuk pemegang harga.lihat; tab keduanya tanpa izin kembali ke bawaan.
  const [tab, setTab] = useTabHalaman<TabOps>(
    [
      ...(lihatHarga ? TAB_PENUH : TAB_TANPA_HARGA),
      ...(adaMl ? (["proyeksi"] as const) : []),
    ],
    "ringkasan",
  );

  const daftarTab: ViewTab<TabOps>[] = [
    { value: "ringkasan", label: "Ringkasan", icon: <LayoutDashboard /> },
    { value: "harga", label: "Harga", icon: <LineChart /> },
    { value: "pasokan", label: "Pasokan", icon: <Package /> },
    ...(adaMl
      ? [
          {
            value: "proyeksi" as const,
            label: "Proyeksi",
            icon: <BrainCircuit />,
          },
        ]
      : []),
    ...(lihatHarga
      ? [
          {
            value: "validasi" as const,
            label: "Validasi",
            icon: <ClipboardCheck />,
          },
          {
            value: "lapangan" as const,
            label: "Lapangan",
            icon: <Users />,
          },
        ]
      : []),
  ];

  return (
    <HalamanPas>
      <PageHeader
        title="Pantauan operasional"
        description="Harga, pasokan, dan tim lapangan di seluruh Aceh. Diperbarui tiap 30 detik."
        tabs={
          <ViewTabs
            layoutId="tab-operasional"
            label="Tampilan operasional"
            value={tab}
            onChange={setTab}
            tabs={daftarTab}
          />
        }
        actions={
          <>
            {bisaValidasi && (
              <SpringLink href="/data/harga?status=DIAJUKAN" variant="soft">
                <ClipboardCheck /> Antrean validasi
              </SpringLink>
            )}
            {bisaInput && (
              <SpringLink href="/lapangan/input">
                <Plus /> Input harga
              </SpringLink>
            )}
          </>
        }
      />

      {/* Antrean dan aktivitas lapangan tidak mengikuti filter, jadi bilahnya tidak ditampilkan di sana. */}
      {tab !== "validasi" && tab !== "lapangan" && <OpsFilterBar />}

      {tab === "ringkasan" && (
        <>
          <WidgetErrorBoundary fallbackTitle="Indikator KPI operasional">
            <KpiGrid />
          </WidgetErrorBoundary>
          <IsiPas kolom="lg:grid-cols-12">
            {lihatPeta ? (
              <>
                <SelBento
                  judul="Peta status wilayah"
                  penuh
                  className={lihatEws ? "lg:col-span-5" : "lg:col-span-8"}
                >
                  <OpsMap />
                </SelBento>
                <SelBento
                  judul="Wilayah prioritas"
                  className={lihatEws ? "lg:col-span-3" : "lg:col-span-4"}
                >
                  <OpsWilayahPrioritas />
                </SelBento>
              </>
            ) : (
              <SelBento
                judul="Peta panas harga"
                className={lihatEws ? "lg:col-span-8" : "lg:col-span-12"}
              >
                <PriceHeatmap />
              </SelBento>
            )}
            {lihatEws && (
              <SelBento judul="Peringatan dini (EWS)" className="lg:col-span-4">
                <EwsLiveFeed className="max-h-[70dvh] p-3.5 sm:p-3.5 lg:max-h-full lg:min-h-0" />
              </SelBento>
            )}
          </IsiPas>
        </>
      )}

      {tab === "harga" && (
        <IsiPas kolom="lg:grid-cols-12">
          <SelBento
            judul="Peta panas harga"
            className="lg:col-span-7 xl:col-span-8"
          >
            <PriceHeatmap />
          </SelBento>
          <div
            className={cn(KOLOM_PAS, "lg:col-span-5 lg:h-full xl:col-span-4")}
          >
            <SelBento judul="Tren harga" className="lg:min-h-56 lg:flex-1">
              <TrendChart isiPenuh />
            </SelBento>
            <SelBento judul="Validasi silang harga" className="lg:max-h-[45%]">
              <PembandingHargaCard />
            </SelBento>
          </div>
        </IsiPas>
      )}

      {tab === "pasokan" && (
        <IsiPas kolom="lg:grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)_minmax(0,1fr)]">
          <SelBento judul="Stok cold storage">
            <OpsStockCard />
          </SelBento>
          <SelBento judul="Harga produsen & margin">
            <OpsProducerCard />
          </SelBento>
          <div className={KOLOM_PAS}>
            <SelBento judul="Pergerakan logistik">
              <OpsLogisticsCard />
            </SelBento>
            <SelBento judul="Cuaca wilayah">
              <CuacaWilayahCard />
            </SelBento>
          </div>
        </IsiPas>
      )}

      {tab === "proyeksi" && adaMl && (
        <TabProyeksiOpsMl
          lampiran={lampiranMl.data}
          memuat={lampiranMl.isLoading}
        />
      )}

      {tab === "validasi" && lihatHarga && (
        <IsiPas>
          <SelBento
            judul="Antrean validasi"
            className="scrollbar-thin lg:overflow-y-auto"
          >
            <ValidationQueue />
          </SelBento>
        </IsiPas>
      )}

      {tab === "lapangan" && lihatHarga && (
        <IsiPas>
          <SelBento judul="Aktivitas tim lapangan">
            <FieldActivity />
          </SelBento>
        </IsiPas>
      )}
    </HalamanPas>
  );
}
