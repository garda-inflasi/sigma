import { Suspense } from "react";
import { ClipboardCheck, Plus } from "lucide-react";
import { TrendChart } from "@/components/charts/trend-chart";
import { FieldActivity } from "@/components/dashboard/field-activity";
import { CuacaWilayahCard, PembandingHargaCard } from "@/components/dashboard/hasil-ingestion";
import { KpiGrid } from "@/components/dashboard/kpi-grid";
import { OpsMap } from "@/components/dashboard/ops-map";
import { PriceHeatmap } from "@/components/dashboard/price-heatmap";
import { EwsLiveFeed } from "@/components/ews/ews-live-feed";
import { PageHeader } from "@/components/garda/page-header";
import { SpringLink } from "@/components/garda/spring-button";
import { WidgetErrorBoundary } from "@/components/garda/widget-error-boundary";
import { ValidationQueue } from "@/components/harga/validation-queue";
import { getSession } from "@/lib/auth/session";
import { can } from "@/lib/rbac/permissions";

export const metadata = { title: "Dashboard operasional" };

function OperasionalSkeleton() {
  return (
    <div className="mx-auto max-w-[1600px] space-y-6">
      <div className="h-16 w-1/3 rounded-xl bg-mist-100 dark:bg-white/5 animate-pulse" />
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="h-28 rounded-xl bg-mist-100 dark:bg-white/5 animate-pulse" />
        ))}
      </div>
      <div className="grid gap-6 xl:grid-cols-12">
        <div className="h-96 rounded-xl bg-mist-100 dark:bg-white/5 animate-pulse xl:col-span-8" />
        <div className="h-96 rounded-xl bg-mist-100 dark:bg-white/5 animate-pulse xl:col-span-4" />
      </div>
    </div>
  );
}

export default async function OperasionalPage() {
  const user = await getSession();
  const lihatPeta = can(user, "spasial.lihat");
  const lihatEws = can(user, "ews.lihat");

  return (
    <Suspense fallback={<OperasionalSkeleton />}>
      <div className="mx-auto max-w-[1600px] space-y-6">
        <PageHeader
          title="Pantauan operasional"
          description="Harga, pasokan, dan aktivitas tim lapangan di seluruh Aceh. Diperbarui otomatis setiap 30 detik."
          actions={
            <>
              {can(user, "harga.validasi") && (
                <SpringLink href="/data/harga?status=DIAJUKAN" variant="soft">
                  <ClipboardCheck /> Antrean validasi
                </SpringLink>
              )}
              {can(user, "harga.input") && (
                <SpringLink href="/lapangan/input">
                  <Plus /> Input harga
                </SpringLink>
              )}
            </>
          }
        />

        <WidgetErrorBoundary fallbackTitle="Indikator KPI operasional">
          <KpiGrid />
        </WidgetErrorBoundary>

        <div className="grid grid-cols-1 xl:grid-cols-12 items-stretch gap-4">
          <div className={lihatEws ? "xl:col-span-7" : "xl:col-span-12"}>
            <WidgetErrorBoundary fallbackTitle="Peta panas harga">
              <PriceHeatmap />
            </WidgetErrorBoundary>
          </div>
          {lihatEws && (
            <div className="xl:col-span-5 relative min-h-[500px]">
              <div className="h-full xl:absolute xl:inset-0 flex flex-col">
                <WidgetErrorBoundary fallbackTitle="Peringatan dini (EWS)" className="h-full">
                  <EwsLiveFeed className="h-full flex flex-col" />
                </WidgetErrorBoundary>
              </div>
            </div>
          )}
        </div>

        <div className="grid gap-6 xl:grid-cols-12 [content-visibility:auto] [contain-intrinsic-size:auto_320px]">
          <div className="xl:col-span-7">
            <WidgetErrorBoundary fallbackTitle="Validasi silang harga">
              <PembandingHargaCard />
            </WidgetErrorBoundary>
          </div>
          <div className="xl:col-span-5">
            <WidgetErrorBoundary fallbackTitle="Cuaca wilayah">
              <CuacaWilayahCard />
            </WidgetErrorBoundary>
          </div>
        </div>

        <div className="grid gap-6 xl:grid-cols-12 [content-visibility:auto] [contain-intrinsic-size:auto_420px]">
          {/* Peta memakai data spasial: peran tanpa spasial.lihat (mis. manajer mutu, agregator) tidak memuatnya. */}
          {lihatPeta && (
            <div className="xl:col-span-7">
              <WidgetErrorBoundary fallbackTitle="Peta status wilayah">
                <OpsMap />
              </WidgetErrorBoundary>
            </div>
          )}
          <div className={lihatPeta ? "xl:col-span-5" : "xl:col-span-12"}>
            <WidgetErrorBoundary fallbackTitle="Tren harga">
              <TrendChart />
            </WidgetErrorBoundary>
          </div>
        </div>

        {can(user, "harga.lihat") && (
          <div className="grid gap-6 xl:grid-cols-12 [content-visibility:auto] [contain-intrinsic-size:auto_500px]">
            <div className="xl:col-span-8">
              <WidgetErrorBoundary fallbackTitle="Antrean validasi">
                <ValidationQueue />
              </WidgetErrorBoundary>
            </div>
            <div className="xl:col-span-4 xl:pt-[68px]">
              <WidgetErrorBoundary fallbackTitle="Aktivitas tim lapangan">
                <FieldActivity />
              </WidgetErrorBoundary>
            </div>
          </div>
        )}
      </div>
    </Suspense>
  );
}
