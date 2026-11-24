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
    <section className="glass flex h-full min-h-0 flex-col rounded-lg p-3.5">
      <PanelTitle
        title="Peta status wilayah"
        description="Klik wilayah untuk melihat status dan pemicu harganya"
        actions={
          <SpringLink href="/peta" size="sm" variant="soft">
            Peta lengkap
          </SpringLink>
        }
      />
      {!isLoading && !data ? (
        <PanelGalat
          judul="Peta status wilayah gagal dimuat"
          error={error}
          onCoba={() => void refetch()}
          className="mt-2 h-72 lg:h-auto lg:min-h-0 lg:flex-1"
        />
      ) : (
        <RegionMap
          markers={data}
          isLoading={isLoading}
          className="mt-2 h-72 lg:h-auto lg:min-h-0 lg:flex-1"
          onSelect={(kode) => router.prefetch(`/peta?wilayah=${kode}`)}
          aksiPopup={{
            label: "Buka detail",
            onClick: (kode) => router.push(`/peta?wilayah=${kode}`),
          }}
        />
      )}
    </section>
  );
}
