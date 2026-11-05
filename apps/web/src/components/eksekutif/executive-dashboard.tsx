"use client";

import { Download } from "lucide-react";
import { useState } from "react";
import { PageHeader, PanelTitle } from "@/components/garda/page-header";
import { PanelSkeleton, Shimmer } from "@/components/garda/shimmer";
import { SpringLink } from "@/components/garda/spring-button";
import { useCan } from "@/lib/auth/session-context";
import { useRingkasanEksekutif } from "@/lib/query/hooks";
import { EwsSummary, InterventionSummary, LatestBrief } from "./exec-side";
import { FiscalSpace } from "./fiscal-space";
import { FoodBalance } from "./food-balance";
import { InflationHero } from "./inflation-hero";
import { XaiPanel } from "./xai-panel";

/** Panel kosong: blok yang datanya belum ada, bukan galat. */
function Kosong({ judul, pesan }: { judul: string; pesan: string }) {
  return (
    <section className="glass rounded-xl p-5 sm:p-6">
      <PanelTitle title={judul} description={pesan} />
    </section>
  );
}

export function ExecutiveDashboard() {
  const { data, isLoading } = useRingkasanEksekutif();
  const [kode, setKode] = useState("CBM-KRT");
  const lihatFiskal = useCan("fiskal.lihat");
  const lihatPrediksi = useCan("prediksi.lihat");
  const lihatBrief = useCan("brief.lihat");

  return (
    <div className="mx-auto max-w-[1600px] space-y-6">
      <PageHeader
        title="Ringkasan Dewan Pengarah"
        description="Kondisi inflasi, neraca pangan, ruang fiskal, dan rekomendasi AI untuk keputusan pekan ini."
        actions={lihatBrief && <SpringLink href="/policy-brief" variant="soft"><Download /> Policy brief terbaru</SpringLink>}
      />
      {data?.demo && (
        <p role="status" className="rounded-xl border border-dashed border-waspada/50 bg-waspada/10 px-4 py-3 text-sm text-waspada-ink">
          Data demo: angka inflasi, neraca pangan, dan ruang fiskal di halaman ini adalah contoh, bukan angka resmi BPS, BI, atau DJPb.
        </p>
      )}
      {isLoading || !data ? (
        <Shimmer className="h-[420px] w-full rounded-xl" />
      ) : data.inflasi ? (
        <InflationHero data={data.inflasi} />
      ) : (
        <Kosong judul="Inflasi tahunan" pesan="Belum ada data inflasi bulanan. Angka resmi masuk lewat impor dari BPS." />
      )}
      <div className="grid gap-6 xl:grid-cols-12">
        <div className={lihatFiskal ? "xl:col-span-7" : "xl:col-span-12"}>
          {!data ? <PanelSkeleton /> : data.neraca ? <FoodBalance data={data.neraca.items} /> : <Kosong judul="Neraca pangan" pesan="Belum ada data neraca pangan." />}
        </div>
        {lihatFiskal && (
          <div className="xl:col-span-5">
            {!data ? <PanelSkeleton /> : data.fiskal ? <FiscalSpace data={data.fiskal} /> : <Kosong judul="Ruang fiskal" pesan="Belum ada data ruang fiskal. Angka resmi masuk lewat impor dari DJPb." />}
          </div>
        )}
      </div>
      <div className="grid gap-6 xl:grid-cols-12">
        {lihatPrediksi && <XaiPanel kode={kode} onKodeChange={setKode} className="xl:col-span-8" />}
        <div className={lihatPrediksi ? "space-y-6 xl:col-span-4" : "grid gap-6 md:grid-cols-3 xl:col-span-12"}>
          {data && <EwsSummary data={data.ews} />}
          {data && <InterventionSummary data={data.intervensi} dominan={data.siklusDominan} />}
          {lihatBrief && <LatestBrief />}
        </div>
      </div>
    </div>
  );
}
