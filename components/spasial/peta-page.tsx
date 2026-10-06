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
import { exportCsv } from "@/lib/csv";
import { kabupatenByKode } from "@/lib/data/reference";
import { namaWilayah } from "@/lib/data/generators";
import { formatAngka, formatPersen, formatRupiah } from "@/lib/format";
import { jarakMeter } from "@/lib/offline/acuan";
import { useMarkerWilayah } from "@/lib/query/hooks";
import { cn } from "@/lib/utils";

interface Rute { id: string; asal: string; tujuan: string; komoditas: string; volumeTon: number; jarakKm: number; jamTempuh: number; biaya: number }
const SENTRA = [{ kode: "1106", kom: "Cabai merah keriting" }, { kode: "1117", kom: "Cabai merah keriting" }, { kode: "1109", kom: "Bawang merah" }];
const TARIF_TON_KM = 4200;

export function PetaPage() {
  const params = useSearchParams();
  const [pilih, setPilih] = useState(params.get("wilayah") ?? undefined);
  const { data: markers, isLoading } = useMarkerWilayah();
  const urut = useMemo(() => [...(markers ?? [])].filter((m) => m.punyaPasar).sort((a, b) => b.delta - a.delta), [markers]);

  const rute = useMemo<Rute[]>(() => {
    const defisit = urut.filter((m) => m.delta > 5).slice(0, 5);
    return defisit.flatMap((d, i) => {
      const s = SENTRA[i % SENTRA.length];
      const a = kabupatenByKode[s.kode];
      const km = (jarakMeter(a, d) / 1000) * 1.35; // faktor jalan
      const vol = Math.round(8 + d.delta * 1.6);
      return [{ id: `RT-${s.kode}-${d.kode}`, asal: namaWilayah(s.kode), tujuan: d.nama, komoditas: s.kom, volumeTon: vol, jarakKm: Math.round(km), jamTempuh: +(km / 45).toFixed(1), biaya: Math.round(km * vol * TARIF_TON_KM) }];
    });
  }, [urut]);

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
          <RegionMap markers={markers} isLoading={isLoading} selected={pilih} onSelect={setPilih} className="h-[560px]" />
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
      <div className="grid gap-6 xl:grid-cols-12">
        <section className="space-y-3 xl:col-span-8">
          <PanelTitle title="Rekomendasi rute distribusi" description={`Biaya dihitung Rp${formatAngka(TARIF_TON_KM)}/ton-km dengan faktor jalan 1,35`} />
          <DataTable data={rute} columns={kolom} getRowId={(r) => r.id} entityLabel="rute" pageSize={10} searchPlaceholder="Cari rute"
            bulkActions={[
              { id: "ajukan", label: "Ajukan sebagai intervensi kilat", icon: Send, perm: "logistik.kelola", tone: "data", run: (r) => { toast.success(`${r.length} rute diajukan`, { description: "Masuk ke tahap “Usulan intervensi logistik kilat”." }); } },
              { id: "ekspor", label: "Ekspor CSV", icon: Download, run: (r) => exportCsv(r, [{ header: "Asal", value: (x) => x.asal }, { header: "Tujuan", value: (x) => x.tujuan }, { header: "Komoditas", value: (x) => x.komoditas }, { header: "Volume (ton)", value: (x) => x.volumeTon }, { header: "Jarak (km)", value: (x) => x.jarakKm }, { header: "Biaya", value: (x) => x.biaya }], "rute-distribusi") },
            ]} />
        </section>
        <section className="glass rounded-xl p-5 xl:col-span-4">
          <PanelTitle title="Kandidat lokasi cold storage" description="Skor gabungan surplus musiman, akses jalan, dan permintaan" />
          <ol className="mt-4 space-y-3">
            {[{ w: "Kota Langsa", s: 82, a: "Rencana" }, { w: "Kab. Bireuen", s: 77, a: "Usulan baru" }, { w: "Kab. Pidie", s: 71, a: "Usulan baru" }].map((c, i) => (
              <li key={c.w} className="flex items-center gap-3 rounded-xl bg-white/50 p-3 dark:bg-white/5">
                <span className="num grid size-8 place-items-center rounded-xl bg-navy-800 text-sm font-semibold text-white">{i + 1}</span>
                <span className="min-w-0 flex-1"><span className="block text-sm font-medium">{c.w}</span><StatusBadge tone={c.a === "Rencana" ? "data" : "netral"} className="mt-1 h-5 text-[10px]">{c.a}</StatusBadge></span>
                <span className="text-right"><Warehouse className="ml-auto size-4 text-muted-foreground" /><span className="num text-sm font-semibold">{c.s}</span></span>
              </li>
            ))}
          </ol>
        </section>
      </div>
    </div>
  );
}
