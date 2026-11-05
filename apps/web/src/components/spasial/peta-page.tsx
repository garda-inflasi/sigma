"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { ArrowRight, Download, Send, Warehouse } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useMemo, useState } from "react";
import { toast } from "sonner";
import { DataTable } from "@/components/data-table/data-table";
import { PageHeader, PanelTitle } from "@/components/garda/page-header";
import { StatusBadge } from "@/components/garda/status-badge";
import { RegionMap } from "@/components/map/region-map";
import { LokasiPanel, type TitikLokasi } from "./lokasi-panel";
import { NeracaPanel } from "./neraca-panel";
import { OptimasiPanel } from "./optimasi-panel";
import { exportCsv } from "@/lib/csv";
import { formatAngka, formatPersen, formatRupiah } from "@/lib/format";
import { useCariLokasi, useGeometriWilayah, useKandidatColdStorage, useMarkerWilayah, useRuteDistribusi } from "@/lib/query/hooks";
import { ZOOM_GEOMETRI, type Rute } from "@/lib/api/spasial";
import { cn } from "@/lib/utils";

export function PetaPage() {
  const params = useSearchParams();
  const [pilih, setPilih] = useState(params.get("wilayah") ?? undefined);
  const { data: markers, isLoading } = useMarkerWilayah();
  const { data: geometri } = useGeometriWilayah(ZOOM_GEOMETRI);
  const urut = useMemo(() => [...(markers ?? [])].filter((m) => m.punyaPasar).sort((a, b) => b.delta - a.delta), [markers]);

  const { data: rute, isLoading: ruteMemuat } = useRuteDistribusi();
  const { data: kandidat } = useKandidatColdStorage();

  // Lokasi dari geolokasi atau klik peta; wilayah hasilnya langsung dipilih di peta.
  const cariLokasi = useCariLokasi();
  const [titik, setTitik] = useState<TitikLokasi>();
  const cariTitik = (t: TitikLokasi) => {
    setTitik(t);
    cariLokasi.mutate(t, { onSuccess: (l) => setPilih(l.kode) });
  };

  const kolom = useMemo<ColumnDef<Rute>[]>(() => [
    { id: "rute", accessorFn: (r) => `${r.asal} ${r.tujuan}`, header: "Rute", cell: ({ row: { original: r } }) => <span className="flex items-center gap-2 whitespace-nowrap font-medium">{r.asal}<ArrowRight className="size-3.5 text-muted-foreground" />{r.tujuan}</span> },
    { accessorKey: "komoditas", header: "Komoditas" },
    { accessorKey: "volumeTon", header: "Volume", cell: ({ getValue }) => <span className="num">{getValue<number>()} ton</span> },
    { accessorKey: "jarakKm", header: "Jarak", cell: ({ getValue }) => <span className="num">{formatAngka(getValue<number>())} km</span> },
    { accessorKey: "jamTempuh", header: "Tempuh", cell: ({ getValue }) => <span className="num">{formatAngka(getValue<number>(), 1)} jam</span> },
    { accessorKey: "biaya", header: "Estimasi biaya", cell: ({ getValue }) => <span className="num font-semibold">{formatRupiah(getValue<number>(), { compact: true })}</span> },
  ], []);

  return (
    <div className="mx-auto max-w-[1600px] space-y-6">
      <PageHeader title="Peta surplus–defisit" description="Status harga per wilayah, rekomendasi rute dari sentra surplus ke wilayah tertekan, dan kandidat lokasi cold storage." />
      <div className="grid gap-6 xl:grid-cols-12">
        <section className="glass rounded-xl p-4 xl:col-span-8">
          <LokasiPanel titik={titik} hasil={cariLokasi.data} galat={cariLokasi.error instanceof Error ? cariLokasi.error.message : undefined} mencari={cariLokasi.isPending} onCari={cariTitik} />
          <RegionMap markers={markers} geometri={geometri} isLoading={isLoading} selected={pilih} onSelect={setPilih} onKlikPeta={(lat, lng) => cariTitik({ lat, lng })} className="h-[560px]" />
        </section>
        <section className="glass flex max-h-[600px] flex-col rounded-xl p-5 xl:col-span-4">
          <PanelTitle title="Wilayah" description="Diurutkan dari tekanan tertinggi" />
          <ul className="scrollbar-thin mt-4 -mr-2 flex-1 space-y-1 overflow-y-auto pr-2">
            {urut.map((m) => (
              <li key={m.kode}>
                <button onClick={() => setPilih(m.kode)} className={cn("flex w-full items-center gap-3 rounded-xl p-3 text-left transition-colors hover:bg-white/60 dark:hover:bg-white/5", pilih === m.kode && "bg-white shadow-ambient-sm dark:bg-white/10")}>
                  <span className={cn("size-2.5 rounded-full", { defisit: "bg-defisit", waspada: "bg-waspada", normal: "bg-normal", surplus: "bg-surplus" }[m.status])} />
                  <span className="min-w-0 flex-1"><span className="block truncate text-sm font-medium">{m.nama}</span><span className="block truncate text-xs text-muted-foreground">{m.komoditas}</span></span>
                  <span className="num text-sm font-semibold">{formatPersen(m.delta, { signed: true })}</span>
                </button>
              </li>
            ))}
          </ul>
        </section>
      </div>
      <NeracaPanel pilih={pilih} onPilih={setPilih} />
      <OptimasiPanel markers={markers ?? []} />
      <div className="grid gap-6 xl:grid-cols-12">
        <section className="space-y-3 xl:col-span-8">
          <PanelTitle title="Rekomendasi rute distribusi" description="Biaya dihitung Rp4.200/ton-km dengan faktor jalan 1,35" />
          <DataTable data={rute} isLoading={ruteMemuat} columns={kolom} getRowId={(r) => r.id} entityLabel="rute" pageSize={10} searchPlaceholder="Cari rute"
            bulkActions={[
              { id: "ajukan", label: "Ajukan sebagai intervensi kilat", icon: Send, perm: "logistik.kelola", tone: "data", run: (r) => { toast.success(`${r.length} rute diajukan`, { description: "Masuk ke tahap “Usulan intervensi logistik kilat”." }); } },
              { id: "ekspor", label: "Ekspor CSV", icon: Download, run: (r) => exportCsv(r, [{ header: "Asal", value: (x) => x.asal }, { header: "Tujuan", value: (x) => x.tujuan }, { header: "Komoditas", value: (x) => x.komoditas }, { header: "Volume (ton)", value: (x) => x.volumeTon }, { header: "Jarak (km)", value: (x) => x.jarakKm }, { header: "Biaya", value: (x) => x.biaya }], "rute-distribusi") },
            ]} />
        </section>
        <section className="glass rounded-xl p-5 xl:col-span-4">
          <PanelTitle title="Kandidat lokasi cold storage" description="Skor gabungan surplus musiman, akses jalan, dan permintaan" />
          <ol className="mt-4 space-y-3">
            {(kandidat ?? []).map((c, i) => (
              <li key={c.id} className="flex items-center gap-3 rounded-xl bg-white/50 p-3 dark:bg-white/5">
                <span className="num grid size-8 place-items-center rounded-xl bg-navy-800 text-sm font-semibold text-white">{i + 1}</span>
                <span className="min-w-0 flex-1"><span className="block text-sm font-medium">{c.lokasi}</span><StatusBadge tone={c.rencana ? "data" : "netral"} className="mt-1 h-5 text-[10px]">{c.rencana ? "Rencana" : "Usulan baru"}</StatusBadge></span>
                <span className="text-right"><Warehouse className="ml-auto size-4 text-muted-foreground" /><span className="num text-sm font-semibold">{c.skor}</span></span>
              </li>
            ))}
          </ol>
        </section>
      </div>
    </div>
  );
}
