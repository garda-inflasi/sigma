"use client";

import type { ColumnDef } from "@tanstack/react-table";
import {
  ArrowRight,
  Download,
  Map as IkonPeta,
  Route,
  Scale,
  Send,
  Sparkles,
  Warehouse,
} from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useMemo, useState } from "react";
import { toast } from "sonner";
import { DataTable, SelTeks } from "@/components/data-table/data-table";
import { EmptyState } from "@/components/garda/empty-state";
import {
  HalamanPas,
  IsiPas,
  PanelPas,
  RingkasanKeputusan,
  type ItemRingkas,
} from "@/components/garda/halaman-pas";
import { PageHeader } from "@/components/garda/page-header";
import { StatusBadge } from "@/components/garda/status-badge";
import { ViewTabs } from "@/components/garda/view-tabs";
import { RegionMap } from "@/components/map/region-map";
import { Button } from "@/components/ui/button";
import { LokasiPanel, type TitikLokasi } from "./lokasi-panel";
import { NeracaPanel } from "./neraca-panel";
import { OptimasiPanel } from "./optimasi-panel";
import { ZOOM_GEOMETRI, type Rute } from "@/lib/api/spasial";
import { useCan } from "@/lib/auth/session-context";
import { exportCsv } from "@/lib/csv";
import { formatAngka, formatPersen, formatRupiah } from "@/lib/format";
import { useTabHalaman } from "@/lib/hooks/use-tab-halaman";
import {
  META_STATUS,
  type StatusWilayah,
  type TitikPeta,
} from "@/lib/peta/aceh";
import {
  useCariLokasi,
  useColdStorage,
  useGeometriWilayah,
  useKandidatColdStorage,
  useMarkerWilayah,
  useMasterPasar,
  useRuteDistribusi,
} from "@/lib/query/hooks";
import { cn } from "@/lib/utils";

const TAB = ["peta", "neraca", "rute", "optimasi", "kandidat"] as const;
const PARAM_CS = {};

const KETERANGAN_STATUS: Record<StatusWilayah, string> = {
  defisit: "Perlu tindakan segera",
  waspada: "Pantau kenaikan harga",
  normal: "Harga stabil",
  surplus: "Bisa jadi sumber pasokan",
};
const TONE_STATUS = {
  defisit: "defisit",
  waspada: "waspada",
  normal: "normal",
  surplus: "surplus",
} as const;

export function PetaPage() {
  const params = useSearchParams();
  const [tab, setTab] = useTabHalaman(TAB, "peta");
  const [pilih, setPilih] = useState(params.get("wilayah") ?? undefined);
  const [saringStatus, setSaringStatus] = useState<StatusWilayah>();
  const { data: markers, isLoading } = useMarkerWilayah();
  const { data: geometri } = useGeometriWilayah(ZOOM_GEOMETRI);
  const { data: rute, isLoading: ruteMemuat } = useRuteDistribusi();
  const { data: kandidat } = useKandidatColdStorage();
  const { data: pasar } = useMasterPasar(useCan("master.lihat"));
  const { data: cold } = useColdStorage(PARAM_CS, useCan("coldstorage.lihat"));

  const titikLain = useMemo<TitikPeta[]>(
    () => [
      ...(pasar ?? [])
        .filter((p) => p.aktif)
        .map((p) => ({
          id: `pasar-${p.id}`,
          jenis: "pasar" as const,
          nama: p.nama,
          keterangan: `Pasar · ${p.kabupaten}`,
          lat: p.lat,
          lng: p.lng,
        })),
      ...(cold?.fasilitas ?? []).map((f) => ({
        id: `cold-${f.id}`,
        jenis: "cold-storage" as const,
        nama: f.nama,
        keterangan: `Cold storage · ${f.wilayah}`,
        lat: f.lat,
        lng: f.lng,
      })),
    ],
    [pasar, cold],
  );

  const jumlahStatus = useMemo(() => {
    const n: Record<StatusWilayah, number> = {
      defisit: 0,
      waspada: 0,
      normal: 0,
      surplus: 0,
    };
    for (const m of markers ?? []) n[m.status]++;
    return n;
  }, [markers]);

  const urut = useMemo(
    () =>
      [...(markers ?? [])]
        .filter(
          (m) => m.punyaPasar && (!saringStatus || m.status === saringStatus),
        )
        .sort(
          (a, b) =>
            META_STATUS[a.status].urutan - META_STATUS[b.status].urutan ||
            b.delta - a.delta,
        ),
    [markers, saringStatus],
  );

  // Lokasi dari geolokasi atau klik peta; wilayah hasilnya langsung dipilih di peta.
  const cariLokasi = useCariLokasi();
  const [titik, setTitik] = useState<TitikLokasi>();
  const [fokusPilihan, setFokusPilihan] = useState(true);
  const pilihWilayah = (kode: string) => {
    setFokusPilihan(true);
    setPilih(kode);
  };
  const cariTitik = (t: TitikLokasi, dariKlikPeta = false) => {
    setFokusPilihan(!dariKlikPeta);
    setTitik(t);
    cariLokasi.mutate(t, { onSuccess: (l) => setPilih(l.kode) });
  };

  const ringkasan: ItemRingkas[] = (
    ["defisit", "waspada", "normal", "surplus"] as const
  ).map((s) => ({
    label: META_STATUS[s].label,
    nilai: isLoading ? "–" : `${jumlahStatus[s]} wilayah`,
    keterangan: KETERANGAN_STATUS[s],
    tone: TONE_STATUS[s],
    arti: META_STATUS[s].arti,
    aktif: saringStatus === s,
    onClick: () => setSaringStatus((v) => (v === s ? undefined : s)),
  }));

  const kolom = useMemo<ColumnDef<Rute>[]>(
    () => [
      {
        id: "rute",
        accessorFn: (r) => `${r.asal} ${r.tujuan}`,
        header: "Rute",
        cell: ({ row: { original: r } }) => (
          <span className="flex items-center gap-2 font-medium whitespace-nowrap">
            {r.asal}
            <ArrowRight className="size-3.5 text-muted-foreground" />
            {r.tujuan}
          </span>
        ),
      },
      {
        accessorKey: "komoditas",
        header: "Komoditas",
        cell: ({ getValue }) => <SelTeks>{getValue<string>()}</SelTeks>,
      },
      {
        accessorKey: "volumeTon",
        header: "Volume",
        cell: ({ getValue }) => (
          <span className="num">{getValue<number>()} ton</span>
        ),
      },
      {
        accessorKey: "jarakKm",
        header: "Jarak",
        cell: ({ getValue }) => (
          <span className="num">{formatAngka(getValue<number>())} km</span>
        ),
      },
      {
        accessorKey: "jamTempuh",
        header: "Tempuh",
        cell: ({ getValue }) => (
          <span className="num">{formatAngka(getValue<number>(), 1)} jam</span>
        ),
      },
      {
        accessorKey: "biaya",
        header: "Estimasi biaya",
        cell: ({ getValue }) => (
          <span className="num font-semibold">
            {formatRupiah(getValue<number>(), { compact: true })}
          </span>
        ),
      },
    ],
    [],
  );

  return (
    <HalamanPas>
      <PageHeader
        title="Peta surplus–defisit"
        description="Lihat wilayah yang tertekan, cari sumber pasokan terdekat, lalu rencanakan distribusi."
        tabs={
          <ViewTabs
            layoutId="tab-peta"
            label="Tampilan peta"
            value={tab}
            onChange={setTab}
            tabs={[
              { value: "peta", label: "Peta", icon: <IkonPeta /> },
              { value: "neraca", label: "Neraca", icon: <Scale /> },
              {
                value: "rute",
                label: "Rute",
                icon: <Route />,
                count: rute?.length,
              },
              { value: "optimasi", label: "Optimasi", icon: <Sparkles /> },
              {
                value: "kandidat",
                label: "Lokasi baru",
                icon: <Warehouse />,
                count: kandidat?.length,
              },
            ]}
          />
        }
      />

      {tab === "peta" && (
        <>
          <RingkasanKeputusan
            items={ringkasan}
            label="Ringkasan status wilayah"
          />
          <IsiPas kolom="lg:grid-cols-[minmax(0,1fr)_19rem]">
            <section className="glass flex min-w-0 flex-col overflow-hidden rounded-lg lg:h-full lg:min-h-0">
              <div className="shrink-0 border-b border-border px-3 py-2">
                <LokasiPanel
                  titik={titik}
                  hasil={cariLokasi.data}
                  galat={
                    cariLokasi.error instanceof Error
                      ? cariLokasi.error.message
                      : undefined
                  }
                  mencari={cariLokasi.isPending}
                  onCari={(t) => cariTitik(t)}
                />
              </div>
              <RegionMap
                markers={markers}
                geometri={geometri}
                titikLain={titikLain}
                titikCari={titik}
                isLoading={isLoading}
                selected={pilih}
                onSelect={setPilih}
                onKlikPeta={(lat, lng) => cariTitik({ lat, lng }, true)}
                fokusPilihan={fokusPilihan}
                className="h-[62dvh] rounded-none border-0 lg:h-auto lg:min-h-0 lg:flex-1"
              />
            </section>
            <PanelPas
              judul="Wilayah"
              deskripsi={
                saringStatus
                  ? `Hanya status ${META_STATUS[saringStatus].label}`
                  : "Urut dari yang paling tertekan"
              }
              aksi={
                saringStatus && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setSaringStatus(undefined)}
                  >
                    Reset filter
                  </Button>
                )
              }
              tanpaPadding
              isiClassName="max-h-[50dvh] overflow-y-auto lg:max-h-none"
            >
              {urut.length === 0 && !isLoading ? (
                <EmptyState
                  title="Tidak ada wilayah"
                  description="Tidak ada wilayah dengan status ini."
                  className="py-10"
                />
              ) : (
                <ul className="divide-y divide-border">
                  {urut.map((m) => (
                    <li key={m.kode}>
                      <button
                        type="button"
                        onClick={() => pilihWilayah(m.kode)}
                        aria-current={pilih === m.kode ? "true" : undefined}
                        className={cn(
                          "flex w-full items-center gap-2.5 px-3.5 py-2 text-left transition-colors hover:bg-secondary/60",
                          pilih === m.kode && "bg-secondary",
                        )}
                      >
                        <span
                          aria-hidden
                          className="size-2 shrink-0 rounded-full"
                          style={{
                            backgroundColor: META_STATUS[m.status].warna,
                          }}
                        />
                        <span className="min-w-0 flex-1">
                          <span className="block truncate text-[13px] font-medium">
                            {m.nama}
                          </span>
                          <span className="block truncate text-xs text-muted-foreground">
                            {META_STATUS[m.status].label} · {m.komoditas}
                          </span>
                        </span>
                        <span className="num text-[13px] font-semibold">
                          {formatPersen(m.delta, { signed: true })}
                        </span>
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </PanelPas>
          </IsiPas>
        </>
      )}

      {tab === "neraca" && (
        <div className="scrollbar-thin min-w-0 lg:min-h-0 lg:flex-1 lg:overflow-y-auto">
          <NeracaPanel pilih={pilih} onPilih={setPilih} />
        </div>
      )}

      {tab === "optimasi" && (
        <div className="scrollbar-thin min-w-0 lg:min-h-0 lg:flex-1 lg:overflow-y-auto">
          <OptimasiPanel markers={markers ?? []} />
        </div>
      )}

      {tab === "rute" && (
        <IsiPas>
          <p className="shrink-0 text-[13px] text-muted-foreground">
            Rekomendasi pengiriman dari sentra surplus ke wilayah tertekan.
            Biaya dihitung Rp4.200/ton-km dengan faktor jalan 1,35.
          </p>
          <DataTable
            isiPenuh
            data={rute}
            isLoading={ruteMemuat}
            columns={kolom}
            getRowId={(r) => r.id}
            entityLabel="rute"
            pageSize={25}
            searchPlaceholder="Cari rute atau komoditas"
            bulkActions={[
              {
                id: "ajukan",
                label: "Ajukan sebagai intervensi kilat",
                icon: Send,
                perm: "logistik.kelola",
                tone: "data",
                run: (r) => {
                  toast.success(`${r.length} rute diajukan`, {
                    description:
                      "Masuk ke tahap “Usulan intervensi logistik kilat”.",
                  });
                },
              },
              {
                id: "ekspor",
                label: "Ekspor CSV",
                icon: Download,
                run: (r) =>
                  exportCsv(
                    r,
                    [
                      { header: "Asal", value: (x) => x.asal },
                      { header: "Tujuan", value: (x) => x.tujuan },
                      { header: "Komoditas", value: (x) => x.komoditas },
                      { header: "Volume (ton)", value: (x) => x.volumeTon },
                      { header: "Jarak (km)", value: (x) => x.jarakKm },
                      { header: "Biaya", value: (x) => x.biaya },
                    ],
                    "rute-distribusi",
                  ),
              },
            ]}
          />
        </IsiPas>
      )}

      {tab === "kandidat" && (
        <PanelPas
          judul="Kandidat lokasi cold storage"
          deskripsi="Skor gabungan surplus musiman, akses jalan, dan permintaan. Makin tinggi makin layak."
        >
          {(kandidat ?? []).length === 0 ? (
            <EmptyState
              title="Belum ada kandidat"
              description="Kandidat muncul setelah analisis surplus musiman tersedia."
              className="py-10"
            />
          ) : (
            <ol className="grid grid-cols-1 gap-2 md:grid-cols-2 xl:grid-cols-3">
              {(kandidat ?? []).map((c, i) => (
                <li
                  key={c.id}
                  className="flex items-center gap-3 rounded-lg border border-border bg-secondary/60 p-3"
                >
                  <span className="num grid size-8 shrink-0 place-items-center rounded-md bg-navy-800 text-sm font-semibold text-white dark:bg-white dark:text-navy-900">
                    {i + 1}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-[13px] font-medium">
                      {c.lokasi}
                    </span>
                    <StatusBadge
                      tone={c.rencana ? "data" : "netral"}
                      className="mt-1 h-5 text-[10px]"
                    >
                      {c.rencana ? "Rencana" : "Usulan baru"}
                    </StatusBadge>
                  </span>
                  <span className="text-right">
                    <span className="block text-[11px] text-muted-foreground">
                      Skor
                    </span>
                    <span className="num text-sm font-semibold">{c.skor}</span>
                  </span>
                </li>
              ))}
            </ol>
          )}
        </PanelPas>
      )}
    </HalamanPas>
  );
}
