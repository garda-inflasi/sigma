"use client";

import {
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
import { HalamanPas, IsiPas } from "@/components/garda/halaman-pas";
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

type TabOps = "ringkasan" | "harga" | "pasokan" | "validasi" | "lapangan";

const TAB_PENUH: readonly TabOps[] = [
  "ringkasan",
  "harga",
  "pasokan",
  "validasi",
  "lapangan",
];
const TAB_TANPA_HARGA: readonly TabOps[] = ["ringkasan", "harga", "pasokan"];

const BARIS_TUNGGAL = "lg:grid lg:grid-cols-12 lg:grid-rows-[minmax(0,1fr)]";

function SelBento({
  judul,
  className,
  children,
}: {
  judul: string;
  className?: string;
  children: ReactNode;
}) {
  return (
    <div className={cn("min-w-0 lg:min-h-0", className)}>
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
  // Antrean dan aktivitas lapangan hanya untuk pemegang harga.lihat; tab keduanya tanpa izin kembali ke bawaan.
  const [tab, setTab] = useTabHalaman<TabOps>(
    lihatHarga ? TAB_PENUH : TAB_TANPA_HARGA,
    "ringkasan",
  );

  const daftarTab: ViewTab<TabOps>[] = [
    { value: "ringkasan", label: "Ringkasan", icon: <LayoutDashboard /> },
    { value: "harga", label: "Harga", icon: <LineChart /> },
    { value: "pasokan", label: "Pasokan", icon: <Package /> },
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
          <IsiPas className={BARIS_TUNGGAL}>
            {lihatPeta ? (
              <>
                <SelBento
                  judul="Peta status wilayah"
                  className={lihatEws ? "lg:col-span-5" : "lg:col-span-8"}
                >
                  <OpsMap />
                </SelBento>
                <SelBento
                  judul="Wilayah prioritas"
                  className={lihatEws ? "lg:col-span-3" : "lg:col-span-4"}
                >
                  <OpsWilayahPrioritas className="h-full" />
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
                <EwsLiveFeed className="max-h-[70dvh] p-3.5 sm:p-3.5 lg:max-h-none lg:min-h-0" />
              </SelBento>
            )}
          </IsiPas>
        </>
      )}

      {tab === "harga" && (
        <IsiPas className="lg:grid lg:grid-cols-12 lg:grid-rows-[minmax(0,1.15fr)_minmax(0,1fr)]">
          <SelBento
            judul="Peta panas harga"
            className="lg:col-span-7 lg:row-span-2 xl:col-span-8"
          >
            <PriceHeatmap />
          </SelBento>
          <SelBento judul="Tren harga" className="lg:col-span-5 xl:col-span-4">
            <TrendChart isiPenuh />
          </SelBento>
          <SelBento
            judul="Validasi silang harga"
            className="lg:col-span-5 xl:col-span-4"
          >
            <PembandingHargaCard />
          </SelBento>
        </IsiPas>
      )}

      {tab === "pasokan" && (
        <IsiPas className="lg:grid lg:grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)_minmax(0,1fr)] lg:grid-rows-[minmax(0,1fr)_minmax(0,1fr)]">
          <SelBento judul="Stok cold storage" className="lg:row-span-2">
            <OpsStockCard />
          </SelBento>
          <SelBento judul="Harga produsen & margin" className="lg:row-span-2">
            <OpsProducerCard />
          </SelBento>
          <SelBento judul="Pergerakan logistik">
            <OpsLogisticsCard />
          </SelBento>
          <SelBento judul="Cuaca wilayah">
            <CuacaWilayahCard />
          </SelBento>
        </IsiPas>
      )}

      {tab === "validasi" && lihatHarga && (
        <IsiPas>
          <SelBento
            judul="Antrean validasi"
            className="scrollbar-thin lg:flex-1 lg:overflow-y-auto"
          >
            <ValidationQueue />
          </SelBento>
        </IsiPas>
      )}

      {tab === "lapangan" && lihatHarga && (
        <IsiPas>
          <SelBento judul="Aktivitas tim lapangan" className="lg:flex-1">
            <FieldActivity />
          </SelBento>
        </IsiPas>
      )}
    </HalamanPas>
  );
}
