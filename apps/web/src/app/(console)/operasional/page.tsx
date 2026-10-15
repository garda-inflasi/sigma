import { ClipboardCheck, Plus } from "lucide-react";
import { TrendChart } from "@/components/charts/trend-chart";
import { FieldActivity } from "@/components/dashboard/field-activity";
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
  const role = user?.role;
  return (
    <div className="mx-auto max-w-[1600px] space-y-6">
      <PageHeader
        title="Pantauan operasional"
        description="Harga, pasokan, dan aktivitas tim lapangan di seluruh Aceh. Diperbarui otomatis setiap 30 detik."
        actions={
          <>
            {can(role, "harga.validasi") && <SpringLink href="/data/harga?status=DIAJUKAN" variant="soft"><ClipboardCheck /> Antrean validasi</SpringLink>}
            {can(role, "harga.input") && <SpringLink href="/lapangan/input"><Plus /> Input harga</SpringLink>}
          </>
        }
      />
      <KpiGrid />
      <div className="grid gap-6 xl:grid-cols-12">
        <div className="xl:col-span-8"><PriceHeatmap /></div>
        {can(role, "ews.lihat") && <EwsLiveFeed className="xl:col-span-4" />}
      </div>
      <div className="grid gap-6 xl:grid-cols-12">
        <div className="xl:col-span-7"><OpsMap /></div>
        <div className="xl:col-span-5"><TrendChart /></div>
      </div>
      {can(role, "harga.lihat") && (
        <div className="grid gap-6 xl:grid-cols-12">
          <div className="xl:col-span-8"><ValidationQueue /></div>
          <div className="xl:col-span-4 xl:pt-[68px]"><FieldActivity /></div>
        </div>
      )}
    </div>
  );
}
