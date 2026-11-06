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
import { ValidationQueue } from "@/components/harga/validation-queue";
import { getSession } from "@/lib/auth/session";
import { can } from "@/lib/rbac/permissions";

export const metadata = { title: "Dashboard operasional" };

export default async function OperasionalPage() {
  const user = await getSession();
  const lihatPeta = can(user, "spasial.lihat");
  return (
    <div className="mx-auto max-w-[1600px] space-y-6">
      <PageHeader
        title="Pantauan operasional"
        description="Harga, pasokan, dan aktivitas tim lapangan di seluruh Aceh. Diperbarui otomatis setiap 30 detik."
        actions={
          <>
            {can(user, "harga.validasi") && <SpringLink href="/data/harga?status=DIAJUKAN" variant="soft"><ClipboardCheck /> Antrean validasi</SpringLink>}
            {can(user, "harga.input") && <SpringLink href="/lapangan/input"><Plus /> Input harga</SpringLink>}
          </>
        }
      />
      <KpiGrid />
      <div className="grid gap-6 xl:grid-cols-12">
        <div className="xl:col-span-8"><PriceHeatmap /></div>
        {can(user, "ews.lihat") && <EwsLiveFeed className="xl:col-span-4" />}
      </div>
      <div className="grid gap-6 xl:grid-cols-12">
        <div className="xl:col-span-7"><PembandingHargaCard /></div>
        <div className="xl:col-span-5"><CuacaWilayahCard /></div>
      </div>
      <div className="grid gap-6 xl:grid-cols-12">
        {/* Peta memakai data spasial: peran tanpa spasial.lihat (mis. manajer mutu, agregator) tidak memuatnya. */}
        {lihatPeta && <div className="xl:col-span-7"><OpsMap /></div>}
        <div className={lihatPeta ? "xl:col-span-5" : "xl:col-span-12"}><TrendChart /></div>
      </div>
      {can(user, "harga.lihat") && (
        <div className="grid gap-6 xl:grid-cols-12">
          <div className="xl:col-span-8"><ValidationQueue /></div>
          <div className="xl:col-span-4 xl:pt-[68px]"><FieldActivity /></div>
        </div>
      )}
    </div>
  );
}
