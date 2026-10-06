"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Archive, ArrowRightLeft, Download, Rocket } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useMemo, useState } from "react";
import { toast } from "sonner";
import { DataTable } from "@/components/data-table/data-table";
import { XaiPanel } from "@/components/eksekutif/xai-panel";
import { KomoditasIcon } from "@/components/garda/komoditas-icon";
import { PageHeader, PanelTitle } from "@/components/garda/page-header";
import { StatusBadge } from "@/components/garda/status-badge";
import { CountUp } from "@/components/motion/count-up";
import { mockXai } from "@/lib/api/mock";
import { exportCsv } from "@/lib/csv";
import { KOMODITAS } from "@/lib/data/reference";
import { formatPersen, formatRupiah } from "@/lib/format";
import { cn } from "@/lib/utils";

interface BarisProyeksi { id: string; nama: string; kini: number; proyeksi: number; pct: number; lo: number; hi: number; mape: number }
interface BarisModel { id: string; komoditas: string; versi: string; mape: number; drift: number; status: "PRODUKSI" | "KANDIDAT" | "ARSIP" }

function Slider({ label, v, set, min, max, unit }: { label: string; v: number; set: (n: number) => void; min: number; max: number; unit: string }) {
  return (
    <label className="block">
      <span className="flex justify-between text-sm"><span>{label}</span><span className="num text-muted-foreground">{v}{unit}</span></span>
      <input type="range" min={min} max={max} value={v} onChange={(e) => set(Number(e.target.value))} className="mt-2 w-full accent-[var(--color-data-600)]" />
    </label>
  );
}

/** Simulasi what-if: dampak skenario terhadap proyeksi harga (model linear sederhana untuk DSS). */
function SimulasiSkenario({ basePct }: { basePct: number }) {
  const [hujan, setHujan] = useState(100);
  const [pasokan, setPasokan] = useState(0);
  const [subsidi, setSubsidi] = useState(0);
  const hasil = basePct + (hujan - 100) * 0.06 - pasokan * 0.35 - subsidi * 0.18;
  return (
    <section className="glass rounded-xl p-5 sm:p-6">
      <PanelTitle title="Simulasi skenario" description="Cabai merah keriting, Banda Aceh, 14 hari" />
      <div className="mt-5 space-y-4">
        <Slider label="Curah hujan sentra (vs normal)" v={hujan} set={setHujan} min={50} max={250} unit="%" />
        <Slider label="Tambahan pasokan antardaerah" v={pasokan} set={setPasokan} min={0} max={40} unit="%" />
        <Slider label="Subsidi ongkos angkut" v={subsidi} set={setSubsidi} min={0} max={30} unit="%" />
      </div>
      <div className={cn("mt-5 rounded-xl p-4", hasil > 5 ? "bg-defisit-soft/70 dark:bg-defisit/15" : hasil < -3 ? "bg-surplus-soft/70 dark:bg-surplus/15" : "bg-normal-soft/70 dark:bg-normal/15")}>
        <p className="text-xs text-muted-foreground">Proyeksi perubahan harga</p>
        <p className="mt-1 text-3xl font-semibold"><CountUp value={hasil} onView={false} duration={0.5} format={(n) => formatPersen(n, { signed: true })} /></p>
        <p className="mt-1 text-xs text-muted-foreground">Elastisitas dari model v2.3.1. Gunakan sebagai bahan diskusi, bukan keputusan final.</p>
      </div>
    </section>
  );
}

export function PrediksiPage() {
  const params = useSearchParams();
  const router = useRouter();
  const [kode, setKode] = useState(params.get("komoditas") ?? "CBM-KRT");
  const proyeksi = useMemo<BarisProyeksi[]>(() => KOMODITAS.map((k) => {
    const x = mockXai(k.kode);
    return { id: k.kode, nama: k.nama, kini: x.hargaKini, proyeksi: Math.round(x.hargaKini * (1 + x.perubahanPct / 100)), pct: x.perubahanPct, lo: x.interval[0], hi: x.interval[1], mape: x.model.mape };
  }), []);
  const [model, setModel] = useState<BarisModel[]>(() => KOMODITAS.flatMap((k, i) => [
    { id: `${k.kode}-231`, komoditas: k.nama, versi: "v2.3.1", mape: +(k.volatilitas * 180 + 1.5).toFixed(1), drift: +((i % 5) * 0.03).toFixed(2), status: "PRODUKSI" as const },
    { id: `${k.kode}-240`, komoditas: k.nama, versi: "v2.4.0-rc1", mape: +((k.volatilitas * 180 + 1.5) * 0.93).toFixed(1), drift: 0, status: "KANDIDAT" as const },
  ]));

  const kolomProyeksi = useMemo<ColumnDef<BarisProyeksi>[]>(() => [
    { accessorKey: "nama", header: "Komoditas", cell: ({ row: { original: r } }) => <button onClick={() => { setKode(r.id); window.scrollTo({ top: 0, behavior: "smooth" }); }} className="flex items-center gap-2 font-medium hover:text-data-700"><KomoditasIcon kode={r.id} className="text-muted-foreground" />{r.nama}</button> },
    { accessorKey: "kini", header: "Harga kini", cell: ({ getValue }) => <span className="num">{formatRupiah(getValue<number>())}</span> },
    { accessorKey: "proyeksi", header: "Proyeksi 14 hari", cell: ({ getValue }) => <span className="num font-semibold">{formatRupiah(getValue<number>())}</span> },
    { accessorKey: "pct", header: "Perubahan", cell: ({ getValue }) => { const v = getValue<number>(); return <span className={cn("num font-semibold", v > 5 ? "text-defisit-ink" : v < -3 ? "text-surplus-ink" : "text-normal-ink")}>{formatPersen(v, { signed: true })}</span>; } },
    { id: "interval", header: "Rentang 80%", cell: ({ row: { original: r } }) => <span className="num text-xs text-muted-foreground">{formatPersen(r.lo, { signed: true })} s.d. {formatPersen(r.hi, { signed: true })}</span> },
    { accessorKey: "mape", header: "MAPE", cell: ({ getValue }) => <span className="num">{formatPersen(getValue<number>())}</span> },
  ], []);

  const kolomModel = useMemo<ColumnDef<BarisModel>[]>(() => [
    { accessorKey: "komoditas", header: "Komoditas" },
    { accessorKey: "versi", header: "Versi", cell: ({ getValue }) => <span className="num">{getValue<string>()}</span> },
    { accessorKey: "mape", header: "MAPE", cell: ({ getValue }) => <span className="num">{formatPersen(getValue<number>())}</span> },
    { accessorKey: "drift", header: "Drift (PSI)", cell: ({ getValue }) => { const v = getValue<number>(); return <span className={cn("num", v > 0.1 ? "text-waspada-ink" : "text-muted-foreground")}>{v.toFixed(2).replace(".", ",")}</span>; } },
    { accessorKey: "status", header: "Status", filterFn: "equals", cell: ({ getValue }) => { const s = getValue<BarisModel["status"]>(); return <StatusBadge tone={s === "PRODUKSI" ? "normal" : s === "KANDIDAT" ? "data" : "netral"}>{s === "PRODUKSI" ? "Produksi" : s === "KANDIDAT" ? "Kandidat" : "Arsip"}</StatusBadge>; } },
  ], []);

  return (
    <div className="mx-auto max-w-[1600px] space-y-6">
      <PageHeader title="Prediksi & explainable AI" description="Proyeksi harga 14 hari dengan penjelasan faktor, simulasi skenario, dan registri model. Rekomendasi AI selalu melalui validasi manusia." />
      <div className="grid gap-6 xl:grid-cols-12">
        <XaiPanel kode={kode} onKodeChange={setKode} className="xl:col-span-8" />
        <div className="xl:col-span-4"><SimulasiSkenario basePct={mockXai("CBM-KRT").perubahanPct} /></div>
      </div>
      <section className="space-y-3">
        <PanelTitle title="Proyeksi seluruh komoditas" description="Centang untuk meneruskan beberapa proyeksi sekaligus ke kanban intervensi" />
        <DataTable data={proyeksi} columns={kolomProyeksi} getRowId={(r) => r.id} entityLabel="komoditas" pageSize={14} initialSorting={[{ id: "pct", desc: true }]} searchPlaceholder="Cari komoditas"
          rowTone={(r) => (r.pct > 5 ? "defisit" : r.pct < -3 ? "surplus" : undefined)}
          bulkActions={[
            { id: "teruskan", label: "Teruskan ke intervensi", icon: ArrowRightLeft, perm: "intervensi.kelola", tone: "data", run: (r) => { toast.success(`${r.length} kartu intervensi dibuat`); router.push("/intervensi"); } },
            { id: "ekspor", label: "Ekspor CSV", icon: Download, run: (r) => exportCsv(r, [{ header: "Komoditas", value: (x) => x.nama }, { header: "Kini", value: (x) => x.kini }, { header: "Proyeksi", value: (x) => x.proyeksi }, { header: "Perubahan (%)", value: (x) => x.pct }], "proyeksi") },
          ]} />
      </section>
      <section className="space-y-3">
        <PanelTitle title="Registri model" description="Versi, akurasi, dan drift. Promosi kandidat ke produksi butuh izin pengelola model." />
        <DataTable data={model} columns={kolomModel} getRowId={(r) => r.id} entityLabel="model" pageSize={10} searchPlaceholder="Cari model"
          facets={[{ columnId: "status", label: "Status", options: [{ value: "PRODUKSI", label: "Produksi" }, { value: "KANDIDAT", label: "Kandidat" }, { value: "ARSIP", label: "Arsip" }] }]}
          bulkActions={[
            {
              id: "promosi", label: "Promosikan ke produksi", icon: Rocket, perm: "model.kelola", tone: "data", enabled: (r) => r.every((x) => x.status === "KANDIDAT"), confirm: { title: "Promosikan model terpilih?", description: (n) => `${n} model kandidat menggantikan versi produksi. Versi lama otomatis diarsipkan dan dapat dikembalikan.`, actionLabel: "Promosikan" },
              run: (r) => { const ids = new Set(r.map((x) => x.id)); const kom = new Set(r.map((x) => x.komoditas)); setModel((m) => m.map((x) => (ids.has(x.id) ? { ...x, status: "PRODUKSI" } : kom.has(x.komoditas) && x.status === "PRODUKSI" ? { ...x, status: "ARSIP" } : x))); toast.success(`${r.length} model dipromosikan`); }
            },
            { id: "arsip", label: "Arsipkan", icon: Archive, perm: "model.kelola", enabled: (r) => r.every((x) => x.status !== "PRODUKSI"), run: (r) => { const ids = new Set(r.map((x) => x.id)); setModel((m) => m.map((x) => (ids.has(x.id) ? { ...x, status: "ARSIP" } : x))); toast.success(`${r.length} model diarsipkan`); } },
          ]} />
      </section>
    </div>
  );
}
