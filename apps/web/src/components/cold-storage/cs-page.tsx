"use client";

import { motion } from "motion/react";
import { useCallback, useEffect, useState } from "react";
import { PageHeader, PanelTitle } from "@/components/garda/page-header";
import { Shimmer } from "@/components/garda/shimmer";
import type { ColdStorageParams } from "@/lib/api/cold-storage-query";
import type { ColdStorageView } from "@/lib/data/types";
import { formatAngka } from "@/lib/format";
import { stagger } from "@/lib/motion/presets";
import { useColdStorage } from "@/lib/query/hooks";
import { useDebouncedValue } from "@/lib/use-debounced-value";
import { FacilityCard } from "./facility-card";
import { StockTable } from "./stock-table";

/** Sisa masa simpan terpendek lebih dulu: stok yang paling mendesak untuk disalurkan. */
const KRITERIA_AWAL: ColdStorageParams = {
  sort: "sisa_masa_simpan",
  desc: false,
  page: 1,
  limit: 10,
};

function KartuSkeleton() {
  return (
    <div className="glass min-h-[11.5rem] w-[17rem] shrink-0 snap-start rounded-xl p-5 @xl:w-auto">
      <Shimmer className="h-6 w-24 rounded-full" />
      <Shimmer className="mt-4 h-4 w-40 max-w-full" />
      <Shimmer className="mt-2 h-3 w-28" />
      <Shimmer className="mt-6 h-8 w-20" />
      <Shimmer className="mt-3 h-2 w-full rounded-full" />
    </div>
  );
}

export function ColdStoragePage() {
  const [params, setParams] = useState<ColdStorageParams>(KRITERIA_AWAL);
  const [search, setSearch] = useState("");
  const q = useDebouncedValue(search, 300).trim();

  // Mengetik tidak boleh memicu satu permintaan per huruf: kata kunci masuk ke
  // kriteria setelah jeda, bersama kembali ke halaman 1 dalam satu pembaruan.
  useEffect(() => {
    setParams((p) =>
      (p.q ?? "") === q ? p : { ...p, q: q || undefined, page: 1 },
    );
  }, [q]);

  const {
    data: terbaru,
    isLoading,
    isFetching,
    error,
    refetch,
  } = useColdStorage(params);

  // TanStack Query membuang data placeholder begitu permintaan untuk kriteria
  // baru gagal. Data sukses terakhir disimpan di sini agar kartu dan tabel tetap
  // tampil, dengan banner galat, bukan digantikan layar kosong.
  const [terakhirBaik, setTerakhirBaik] = useState<ColdStorageView>();
  useEffect(() => {
    if (terbaru) setTerakhirBaik(terbaru);
  }, [terbaru]);
  const data = terbaru ?? terakhirBaik;

  // Mengubah kriteria apa pun selain halaman mengembalikan tabel ke halaman 1.
  const ubah = useCallback((patch: Partial<ColdStorageParams>) => {
    setParams((p) => ({ ...p, ...patch, page: patch.page ?? 1 }));
  }, []);

  const hapusFilter = useCallback(() => {
    setSearch("");
    setParams((p) => ({ ...p, q: undefined, fasilitasId: undefined, page: 1 }));
  }, []);

  // Data menyusut (mis. stok keluar) hingga halaman aktif melewati halaman terakhir.
  const totalPages = data?.halaman.totalPages ?? 0;
  useEffect(() => {
    if (totalPages > 0 && (params.page ?? 1) > totalPages)
      setParams((p) => ({ ...p, page: totalPages }));
  }, [totalPages, params.page]);

  const risikoTinggiTon = data?.ringkasan.risikoTinggiTon;

  return (
    <div className="mx-auto max-w-[1600px] space-y-6">
      <PageHeader
        title="Cold storage & food loss"
        description="Utilisasi fasilitas, suhu, dan masa simpan stok. Kepemilikan komoditas tetap di tangan produsen dan agregator."
      />

      {/* Kartu fasilitas: baris geser di layar sempit, grid menyesuaikan lebar kontainer (bukan layar) */}
      <div className="@container">
        {isLoading && !data ? (
          <div className="-mx-3 flex snap-x gap-3 overflow-hidden px-3 @xl:mx-0 @xl:grid @xl:grid-cols-2 @xl:gap-4 @xl:px-0 @3xl:grid-cols-3 @[70rem]:grid-cols-5">
            {Array.from({ length: 5 }).map((_, i) => (
              <KartuSkeleton key={i} />
            ))}
          </div>
        ) : data ? (
          <motion.div
            variants={stagger(0.07)}
            initial="hidden"
            animate="show"
            className="scrollbar-thin -mx-3 flex snap-x snap-mandatory gap-3 overflow-x-auto px-3 pb-2 @xl:mx-0 @xl:grid @xl:grid-cols-2 @xl:gap-4 @xl:overflow-visible @xl:px-0 @xl:pb-0 @3xl:grid-cols-3 @[70rem]:grid-cols-5"
          >
            {data.fasilitas.map((f) => (
              <div
                key={f.id}
                className="w-[17rem] shrink-0 snap-start @xl:w-auto"
              >
                <FacilityCard
                  f={f}
                  active={params.fasilitasId === f.id}
                  onSelect={() =>
                    ubah({
                      fasilitasId:
                        params.fasilitasId === f.id ? undefined : f.id,
                    })
                  }
                />
              </div>
            ))}
          </motion.div>
        ) : null}
      </div>

      <section className="space-y-3">
        <PanelTitle
          title="Stok tersimpan"
          description={
            risikoTinggiTon === undefined ? (
              "Urutkan menurut sisa masa simpan untuk menentukan prioritas penyaluran."
            ) : (
              <>
                <span className="num font-medium text-foreground">
                  {formatAngka(risikoTinggiTon, 1)} ton
                </span>{" "}
                berisiko tinggi food loss. Urutkan menurut sisa masa simpan
                untuk menentukan prioritas penyaluran.
              </>
            )
          }
        />
        <StockTable
          view={data}
          isLoading={isLoading && !data}
          isFetching={isFetching}
          error={error}
          onRetry={() => void refetch()}
          params={params}
          onParams={ubah}
          search={search}
          onSearch={setSearch}
          onResetFilters={hapusFilter}
        />
      </section>
    </div>
  );
}
