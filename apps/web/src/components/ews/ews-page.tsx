"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Check, ChevronsUp, CircleCheckBig, Download, Pencil, Play } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useMemo, useState } from "react";
import type { BulkAction } from "@/components/data-table/bulk-action-bar";
import { DataTable } from "@/components/data-table/data-table";
import { EwsSummary } from "@/components/eksekutif/exec-side";
import { PageHeader, PanelTitle } from "@/components/garda/page-header";
import { STATUS_EWS, StatusBadge } from "@/components/garda/status-badge";
import { PulseRing } from "@/components/motion/pulse-ring";
import { exportCsv } from "@/lib/csv";
import type { EwsAlert, StatusEws } from "@/lib/data/types";
import { severityMeta, toneClass, type Severity } from "@/lib/design/tokens";
import { satuanAmbang } from "@/lib/api/ews";
import { formatAngka, formatRelatif } from "@/lib/format";
import { SpringButton } from "@/components/garda/spring-button";
import { useCan } from "@/lib/auth/session-context";
import type { EwsAturan } from "@/lib/api/ews";
import { useBulkAction, useEvaluasiEws, useEwsAturan, useEwsList } from "@/lib/query/hooks";
import { qk } from "@/lib/query/keys";
import { cn } from "@/lib/utils";
import { AturanDialog } from "./aturan-dialog";
import { JENIS_EWS, JenisIcon, formatNilaiEws } from "./ews-meta";

const KANAL_LABEL: Record<string, string> = { DASHBOARD: "Dashboard", PUSH: "Push", EMAIL: "Email", WHATSAPP: "WhatsApp" };

/** Aturan ambang & kanal notifikasi dari basis data; dapat diubah oleh peran dengan `ews.atur_ambang`. */
function AturanAmbang() {
  const { data, isLoading } = useEwsAturan();
  const bisaAtur = useCan("ews.atur_ambang");
  const evaluasi = useEvaluasiEws();
  const [diubah, setDiubah] = useState<EwsAturan | null>(null);
  const aktif = useMemo(() => (data ?? []).filter((a) => a.aktif), [data]);
  const kanal = useMemo(() => [...new Set(aktif.flatMap((a) => a.kanal))], [aktif]);
  return (
    <section className="glass rounded-xl p-5 sm:p-6">
      <PanelTitle title="Ambang peringatan" description="Per komoditas dan jenis; satuan menurut jenis (Δ% harga/stok terhadap rata-rata jendela, atau mm hujan)" />
      <div className="mt-4 overflow-x-auto">
        <table className="w-full min-w-[420px] text-sm">
          <thead><tr className="text-xs text-muted-foreground"><th className="py-2 text-left font-medium">Aturan</th>{(["WASPADA", "SIAGA", "KRITIS"] as Severity[]).map((s) => <th key={s} className={cn("py-2 font-medium", toneClass[severityMeta[s].tone].text)}>{severityMeta[s].label}</th>)}{bisaAtur && <th className="w-8" />}</tr></thead>
          <tbody>
            {isLoading && <tr><td colSpan={bisaAtur ? 5 : 4} className="py-4 text-center text-xs text-muted-foreground">Memuat ambang…</td></tr>}
            {!isLoading && aktif.length === 0 && <tr><td colSpan={bisaAtur ? 5 : 4} className="py-4 text-center text-xs text-muted-foreground">Belum ada aturan ambang aktif.</td></tr>}
            {aktif.map((a) => (
              <tr key={a.id} className="border-t border-border/60">
                <td className="py-2.5"><span className="block">{a.komoditasNama ?? "Semua komoditas"}</span><span className="block text-xs text-muted-foreground">{JENIS_EWS[a.jenis].label} · {a.jendelaHari} hari</span></td>
                {[a.waspada, a.siaga, a.kritis].map((v, j) => <td key={j} className="num py-2.5 text-center">{formatAngka(v)}{satuanAmbang(a.jenis)}</td>)}
                {bisaAtur && <td className="py-2.5 text-right"><button type="button" aria-label={`Ubah ambang ${a.komoditasNama ?? "semua komoditas"} ${JENIS_EWS[a.jenis].label}`} onClick={() => setDiubah(a)} className="rounded-md p-1 text-muted-foreground hover:bg-black/5 hover:text-foreground dark:hover:bg-white/10"><Pencil className="size-3.5" /></button></td>}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {aktif[0] && <p className="mt-3 text-xs text-muted-foreground">{aktif[0].keterangan}</p>}
      {bisaAtur && (
        <SpringButton variant="soft" size="sm" className="mt-4" disabled={evaluasi.isPending} onClick={() => evaluasi.mutate()}>
          <Play /> {evaluasi.isPending ? "Mengevaluasi…" : "Evaluasi sekarang"}
        </SpringButton>
      )}
      <AturanDialog aturan={diubah} onClose={() => setDiubah(null)} />
      <p className="mt-5 text-sm font-medium">Kanal notifikasi</p>
      <div className="mt-2 flex flex-wrap gap-2">
        {kanal.map((k) => <span key={k} className="glass-subtle flex h-9 items-center rounded-xl px-3 text-sm">{KANAL_LABEL[k] ?? k}</span>)}
      </div>
    </section>
  );
}

export function EwsPage() {
  const params = useSearchParams();
  const sev = params.get("severity");
  const sorot = params.get("id");
  const { data, isLoading } = useEwsList();
  const patch = (aksi: string) => (aksi === "akui" ? "DIAKUI" : aksi === "eskalasi" ? "DIESKALASI" : "SELESAI") as StatusEws;
  const mut = useBulkAction<EwsAlert>({ sumber: "ews", queryKey: qk.ews.list(), patch: (r, a) => ({ ...r, status: patch(a) }) });
  const ringkas = useMemo(() => {
    const aktif = (data ?? []).filter((a) => a.status !== "SELESAI");
    return { INFO: 0, WASPADA: 0, SIAGA: 0, KRITIS: 0, ...Object.fromEntries((["INFO", "WASPADA", "SIAGA", "KRITIS"] as Severity[]).map((s) => [s, aktif.filter((a) => a.severity === s).length])) } as Record<Severity, number>;
  }, [data]);

  const columns = useMemo<ColumnDef<EwsAlert>[]>(() => [
    {
      accessorKey: "severity", header: "Tingkat", filterFn: "equals",
      sortingFn: (a, b) => severityMeta[a.original.severity].rank - severityMeta[b.original.severity].rank,
      cell: ({ row: { original: a } }) => {
        const m = severityMeta[a.severity];
        return (
          <span className="inline-flex items-center gap-2">
            <PulseRing tone={m.tone} rings={1} active={a.status === "BARU" && m.rank >= 2} className="size-2.5"><span className={cn("relative size-2.5 rounded-full", toneClass[m.tone].bg)} /></PulseRing>
            <span className={cn("text-xs font-medium", toneClass[m.tone].text)}>{m.label}</span>
          </span>
        );
      },
    },
    {
      id: "alert", accessorFn: (a) => `${a.judul} ${a.pesan}`, header: "Peringatan",
      cell: ({ row: { original: a } }) => (
        <div className="flex max-w-md items-start gap-2.5">
          <JenisIcon jenis={a.jenis} className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
          <div className="min-w-0"><p className={cn("font-medium", a.id === sorot && "text-data-700 dark:text-data-300")}>{a.judul}</p><p className="truncate text-xs text-muted-foreground">{a.pesan}</p></div>
        </div>
      ),
    },
    { accessorKey: "jenis", header: "Jenis", filterFn: "equals", cell: ({ getValue }) => <span className="whitespace-nowrap text-xs text-muted-foreground">{JENIS_EWS[getValue<EwsAlert["jenis"]>()].label}</span> },
    { accessorKey: "wilayah", header: "Wilayah", cell: ({ getValue }) => <span className="whitespace-nowrap">{getValue<string>()}</span> },
    { accessorKey: "nilai", header: "Perubahan", cell: ({ row: { original: a } }) => <span className={cn("num font-semibold", toneClass[severityMeta[a.severity].tone].text)}>{formatNilaiEws(a)}</span> },
    {
      accessorKey: "skorDampak", header: "Dampak",
      cell: ({ getValue }) => { const v = getValue<number>(); return <span className="flex items-center gap-2"><span className="h-1.5 w-16 overflow-hidden rounded-full bg-mist-200 dark:bg-white/10"><span className={cn("block h-full rounded-full", v > 75 ? "bg-defisit" : v > 50 ? "bg-waspada" : "bg-data-500")} style={{ width: `${v}%` }} /></span><span className="num text-xs">{v}</span></span>; },
    },
    { accessorKey: "waktu", header: "Waktu", cell: ({ getValue }) => <span className="whitespace-nowrap text-muted-foreground">{formatRelatif(getValue<string>())}</span> },
    { accessorKey: "status", header: "Status", filterFn: "equals", cell: ({ getValue }) => { const s = STATUS_EWS[getValue<string>()]; return <StatusBadge tone={s.tone}>{s.label}</StatusBadge>; } },
  ], [sorot]);

  const ids = (r: EwsAlert[]) => r.map((x) => x.id);
  const actions: BulkAction<EwsAlert>[] = [
    { id: "akui", label: "Akui", icon: Check, perm: "ews.akui", tone: "data", enabled: (r) => r.every((x) => x.status === "BARU"), run: (r) => mut.mutateAsync({ aksi: "akui", ids: ids(r), label: "Alert diakui" }) },
    { id: "eskalasi", label: "Eskalasi", icon: ChevronsUp, perm: "ews.eskalasi", run: (r) => mut.mutateAsync({ aksi: "eskalasi", ids: ids(r), label: "Alert dieskalasi ke Dewan Pengarah" }) },
    { id: "selesai", label: "Tandai selesai", icon: CircleCheckBig, perm: "ews.akui", confirm: { title: "Tutup alert terpilih?", description: (n) => `${n} alert ditandai selesai dan keluar dari feed peringatan aktif.`, actionLabel: "Tandai selesai" }, run: (r) => mut.mutateAsync({ aksi: "selesai", ids: ids(r), label: "Alert diselesaikan" }) },
    { id: "ekspor", label: "Ekspor CSV", icon: Download, run: (r) => exportCsv(r, [{ header: "ID", value: (x) => x.id }, { header: "Tingkat", value: (x) => x.severity }, { header: "Judul", value: (x) => x.judul }, { header: "Wilayah", value: (x) => x.wilayah }, { header: "Nilai", value: (x) => x.nilai }, { header: "Waktu", value: (x) => x.waktu }, { header: "Status", value: (x) => x.status }], "ews") },
  ];

  return (
    <div className="mx-auto max-w-[1600px] space-y-6">
      <PageHeader title="Peringatan dini" description="Alarm otomatis dari model EWS: lonjakan harga, kelangkaan stok, anomali cuaca, over-supply, dan anomali data. Diprioritaskan menurut skor dampak." />
      <div className="grid gap-6 xl:grid-cols-12">
        <div className="space-y-6 xl:col-span-8">
          <EwsSummary data={ringkas} />
          <DataTable
            data={data}
            isLoading={isLoading}
            columns={columns}
            getRowId={(r) => r.id}
            entityLabel="alert"
            bulkActions={actions}
            facets={[
              { columnId: "severity", label: "Tingkat", options: (["KRITIS", "SIAGA", "WASPADA", "INFO"] as Severity[]).map((s) => ({ value: s, label: severityMeta[s].label })) },
              { columnId: "status", label: "Status", options: Object.entries(STATUS_EWS).map(([value, v]) => ({ value, label: v.label })) },
              { columnId: "jenis", label: "Jenis", options: Object.entries(JENIS_EWS).map(([value, v]) => ({ value, label: v.label })) },
            ]}
            initialFilters={sev ? [{ id: "severity", value: sev }] : []}
            initialSorting={[{ id: "skorDampak", desc: true }]}
            searchPlaceholder="Cari peringatan atau wilayah"
            rowTone={(a) => (a.id === sorot ? "data" : undefined)}
          />
        </div>
        <div className="xl:col-span-4"><AturanAmbang /></div>
      </div>
    </div>
  );
}
