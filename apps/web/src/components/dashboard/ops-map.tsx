"use client";

import { useRouter } from "next/navigation";
import { RegionMap } from "@/components/map/region-map";
import { PanelTitle } from "@/components/garda/page-header";
import { PanelGalat } from "@/components/garda/panel-galat";
import { SpringLink } from "@/components/garda/spring-button";
import { useMarkerWilayah } from "@/lib/query/hooks";

export function OpsMap() {
  const { data, isLoading, error, refetch } = useMarkerWilayah();
  const router = useRouter();
  return (
    <section className="glass flex h-full flex-col rounded-xl p-5 sm:p-6">
      <PanelTitle
        title="Peta status wilayah"
        description="Status harga per kabupaten/kota berdasarkan pergerakan terbesar"
        actions={<SpringLink href="/peta" size="sm" variant="soft">Peta lengkap</SpringLink>}
      />
      {!isLoading && !data ? (
        <PanelGalat judul="Peta status wilayah gagal dimuat" error={error} onCoba={() => void refetch()} className="mt-4 h-[380px] flex-1" />
      ) : (
        <RegionMap markers={data} isLoading={isLoading} className="mt-4 h-[380px] flex-1" onSelect={(kode) => router.prefetch(`/peta?wilayah=${kode}`)} />
      )}
    </section>
  );
}
