"use client";

import { motion } from "motion/react";
import { Boxes, ChevronRight, Warehouse } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { EmptyState } from "@/components/garda/empty-state";
import {
  HalamanPas,
  IsiPas,
  PanelPas,
  RingkasanKeputusan,
  type ItemRingkas,
} from "@/components/garda/halaman-pas";
import { PageHeader } from "@/components/garda/page-header";
import { PanelGalat } from "@/components/garda/panel-galat";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import { ViewTabs } from "@/components/garda/view-tabs";
import { Button } from "@/components/ui/button";
import { utilisasiPersen } from "@/lib/api/cold-storage";
import type { ColdStorageParams } from "@/lib/api/cold-storage-query";
import type { ColdStorageView, FasilitasColdStorage } from "@/lib/data/types";
import type { Tone } from "@/lib/design/tokens";
import { formatAngka } from "@/lib/format";
import { GLOSARIUM } from "@/lib/glosarium";
import { useTabHalaman } from "@/lib/hooks/use-tab-halaman";
import { stagger } from "@/lib/motion/presets";
import { useColdStorage } from "@/lib/query/hooks";
import { useDebouncedValue } from "@/lib/use-debounced-value";
import { FacilityCard, suhuMenyimpang } from "./facility-card";
import { FacilityDetailSheet } from "./facility-detail-sheet";
import { StockTable } from "./stock-table";

const TAB = ["fasilitas", "stok"] as const;

/** Sisa masa simpan terpendek lebih dulu: stok yang paling mendesak untuk disalurkan. */
const KRITERIA_AWAL: ColdStorageParams = {
  sort: "sisa_masa_simpan",
  desc: false,
  page: 1,
  limit: 10,
};

interface Masalah {
  f: FasilitasColdStorage;
  label: string;
  tone: Tone;
  bobot: number;
}

/** Masalah terberat satu fasilitas; fasilitas rencana belum beroperasi sehingga tidak dinilai. */
function masalahFasilitas(f: FasilitasColdStorage): Masalah | null {
  if (f.status === "RENCANA") return null;
  if (suhuMenyimpang(f))
    return { f, label: "Suhu di luar batas", tone: "defisit", bobot: 0 };
  if (f.status === "PERAWATAN")
    return { f, label: "Sedang perawatan", tone: "waspada", bobot: 1 };
  if (utilisasiPersen(f) > 85)
    return { f, label: "Hampir penuh", tone: "defisit", bobot: 2 };
  return null;
}

function KartuSkeleton() {
  return (
    <div className="min-h-[11rem] rounded-lg border border-border bg-card p-3.5">
      <Shimmer className="h-5 w-24 rounded-full" />
      <Shimmer className="mt-3 h-4 w-40 max-w-full" />
      <Shimmer className="mt-2 h-3 w-28" />
      <Shimmer className="mt-4 h-7 w-20" />
      <Shimmer className="mt-2 h-1.5 w-full rounded-full" />
    </div>
  );
}

export function ColdStoragePage() {
  const [tab, setTab] = useTabHalaman(TAB, "fasilitas");
  const [params, setParams] = useState<ColdStorageParams>(KRITERIA_AWAL);
  const [search, setSearch] = useState("");
  const [detailId, setDetailId] = useState<string>();
  const [hanyaMasalah, setHanyaMasalah] = useState(false);
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

  const fasilitas = useMemo(() => data?.fasilitas ?? [], [data]);
  const masalah = useMemo(
    () =>
      fasilitas
        .map(masalahFasilitas)
        .filter((m): m is Masalah => m !== null)
        .sort((a, b) => a.bobot - b.bobot),
    [fasilitas],
  );
  const kapasitas = useMemo(() => {
    const berjalan = fasilitas.filter((f) => f.status !== "RENCANA");
    const terisi = berjalan.reduce((n, f) => n + f.terisiTon, 0);
    const total = berjalan.reduce((n, f) => n + f.kapasitasTon, 0);
    return { terisi, total, persen: total > 0 ? (terisi / total) * 100 : 0 };
  }, [fasilitas]);
  const idMasalah = useMemo(
    () => new Set(masalah.map((m) => m.f.id)),
    [masalah],
  );
  const kartu = hanyaMasalah
    ? fasilitas.filter((f) => idMasalah.has(f.id))
    : fasilitas;

  const ringkasan: ItemRingkas[] = [
    {
      label: "Kapasitas terpakai",
      nilai: data ? `${Math.round(kapasitas.persen)}%` : "–",
      keterangan: data
        ? `${formatAngka(kapasitas.terisi)} dari ${formatAngka(kapasitas.total)} ton`
        : undefined,
      tone: !data ? "netral" : kapasitas.persen > 85 ? "defisit" : "data",
      arti: "Total stok dibanding kapasitas fasilitas yang sudah beroperasi.",
    },
    {
      label: "Fasilitas bermasalah",
      nilai: data ? `${masalah.length} fasilitas` : "–",
      keterangan: !data
        ? undefined
        : masalah[0]
          ? `${masalah[0].label}: ${masalah[0].f.nama}`
          : "Semua berjalan normal",
      tone: !data ? "netral" : masalah.length > 0 ? "defisit" : "normal",
      arti: "Suhu di luar batas, sedang perawatan, atau terisi di atas 85%. Klik untuk menyaring kartu.",
      aktif: hanyaMasalah && tab === "fasilitas",
      onClick: () => {
        setHanyaMasalah((v) => (tab === "fasilitas" ? !v : true));
        setTab("fasilitas");
      },
    },
    {
      label: "Berisiko food loss",
      nilai: data
        ? `${formatAngka(data.ringkasan.risikoTinggiTon, 1)} ton`
        : "–",
      keterangan: data
        ? `${data.ringkasan.risikoTinggiLot} lot perlu disalurkan`
        : undefined,
      tone: !data
        ? "netral"
        : data.ringkasan.risikoTinggiLot > 0
          ? "waspada"
          : "normal",
      arti: GLOSARIUM.foodLoss,
      onClick: () => {
        ubah({ sort: "risiko", desc: true });
        setTab("stok");
      },
    },
    {
      label: "Total stok",
      nilai: data
        ? `${formatAngka(data.ringkasan.totalVolumeTon, 1)} ton`
        : "–",
      keterangan: data ? `${data.ringkasan.totalLot} lot tersimpan` : undefined,
      tone: "netral",
      arti: "Buka daftar stok, urut dari sisa masa simpan terpendek.",
      onClick: () => setTab("stok"),
    },
  ];

  return (
    <HalamanPas>
      <PageHeader
        title="Cold storage & food loss"
        description="Pantau kapasitas dan suhu, lalu salurkan stok yang paling berisiko."
        tabs={
          <ViewTabs
            layoutId="tab-cold-storage"
            label="Tampilan cold storage"
            value={tab}
            onChange={setTab}
            tabs={[
              {
                value: "fasilitas",
                label: "Fasilitas",
                icon: <Warehouse />,
                count: data?.fasilitas.length,
              },
              {
                value: "stok",
                label: "Stok",
                icon: <Boxes />,
                count: data?.ringkasan.totalLot,
              },
            ]}
          />
        }
      />

      <RingkasanKeputusan items={ringkasan} label="Ringkasan cold storage" />

      {/* Tab stok punya banner galat sendiri di dalam tabel. */}
      {tab === "fasilitas" && error && data && (
        <p
          role="alert"
          className="flex shrink-0 flex-wrap items-center gap-x-3 rounded-lg bg-defisit-soft px-3 py-1.5 text-[13px] text-defisit-ink dark:bg-defisit/10 dark:text-rose-300"
        >
          Gagal memperbarui data; menampilkan data terakhir.
          <button
            type="button"
            onClick={() => void refetch()}
            className="font-semibold underline-offset-4 hover:underline"
          >
            Coba lagi
          </button>
        </p>
      )}

      {tab === "fasilitas" && (
        <IsiPas className="lg:grid lg:grid-cols-[minmax(0,1fr)_19rem]">
          <PanelPas
            judul="Fasilitas"
            deskripsi={
              hanyaMasalah
                ? "Hanya fasilitas bermasalah"
                : "Klik kartu untuk melihat stoknya"
            }
            aksi={
              hanyaMasalah && (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setHanyaMasalah(false)}
                >
                  Reset filter
                </Button>
              )
            }
          >
            <div className="@container">
              {isLoading && !data ? (
                <div className="grid grid-cols-1 gap-3 @md:grid-cols-2 @[38rem]:grid-cols-3">
                  {Array.from({ length: 5 }).map((_, i) => (
                    <KartuSkeleton key={i} />
                  ))}
                </div>
              ) : error && !data ? (
                <PanelGalat
                  judul="Fasilitas belum dapat dimuat."
                  error={error}
                  onCoba={() => void refetch()}
                />
              ) : kartu.length === 0 ? (
                <EmptyState
                  title={
                    hanyaMasalah
                      ? "Tidak ada fasilitas bermasalah"
                      : "Belum ada fasilitas"
                  }
                  description={
                    hanyaMasalah
                      ? "Semua fasilitas berjalan normal saat ini."
                      : "Fasilitas tampil di sini setelah didaftarkan."
                  }
                  className="py-10"
                />
              ) : (
                <motion.div
                  key={hanyaMasalah ? "masalah" : "semua"}
                  variants={stagger(0.07)}
                  initial="hidden"
                  animate="show"
                  className="grid grid-cols-1 gap-3 @md:grid-cols-2 @[38rem]:grid-cols-3"
                >
                  {kartu.map((f) => (
                    <FacilityCard
                      key={f.id}
                      f={f}
                      active={params.fasilitasId === f.id}
                      onDetail={() => setDetailId(f.id)}
                      onSelect={() => {
                        if (params.fasilitasId === f.id) {
                          ubah({ fasilitasId: undefined });
                          return;
                        }
                        ubah({ fasilitasId: f.id });
                        setTab("stok");
                      }}
                    />
                  ))}
                </motion.div>
              )}
            </div>
          </PanelPas>

          <PanelPas
            judul="Perlu tindakan"
            deskripsi="Urut dari yang paling mendesak"
            tanpaPadding
            className="order-first lg:order-none"
            isiClassName="max-h-[40dvh] overflow-y-auto lg:max-h-none"
          >
            {!data ? (
              isLoading ? (
                <div role="status" aria-label="Memuat" className="space-y-2 p-3.5">
                  <Shimmer className="h-10 w-full" />
                  <Shimmer className="h-10 w-full" />
                  <Shimmer className="h-10 w-full" />
                </div>
              ) : (
                <EmptyState
                  title="Belum ada data"
                  description="Muat ulang untuk melihat fasilitas yang perlu ditinjau."
                  className="py-10"
                />
              )
            ) : masalah.length === 0 ? (
              <EmptyState
                title="Tidak ada masalah"
                description="Suhu dan kapasitas semua fasilitas dalam batas."
                className="py-10"
              />
            ) : (
              <ul className="divide-y divide-border">
                {masalah.map((m) => (
                  <li key={m.f.id}>
                    <button
                      type="button"
                      onClick={() => setDetailId(m.f.id)}
                      aria-label={`Tinjau ${m.f.nama}: ${m.label}`}
                      className="flex w-full items-center gap-2.5 px-3.5 py-2 text-left transition-colors hover:bg-secondary/60"
                    >
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-[13px] font-medium">
                          {m.f.nama}
                        </span>
                        <span className="mt-0.5 flex items-center gap-1.5">
                          <StatusBadge
                            tone={m.tone}
                            className="h-5 text-[10px]"
                          >
                            {m.label}
                          </StatusBadge>
                          <span className="num text-xs text-muted-foreground">
                            {Math.round(utilisasiPersen(m.f))}% terisi
                          </span>
                        </span>
                      </span>
                      <ChevronRight
                        className="size-4 shrink-0 text-muted-foreground"
                        aria-hidden
                      />
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </PanelPas>
        </IsiPas>
      )}

      {tab === "stok" && (
        <IsiPas>
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
        </IsiPas>
      )}

      <FacilityDetailSheet
        id={detailId}
        fasilitas={fasilitas}
        onClose={() => setDetailId(undefined)}
      />
    </HalamanPas>
  );
}
