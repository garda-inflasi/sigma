"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Check, ChevronsUp, CircleCheckBig, Download, Save } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useMemo, useState } from "react";
import { toast } from "sonner";
import { Checkbox } from "@/components/ui/checkbox";
import type { BulkAction } from "@/components/data-table/bulk-action-bar";
import { DataTable } from "@/components/data-table/data-table";
import { EwsSummary } from "@/components/eksekutif/exec-side";
import { PageHeader, PanelTitle } from "@/components/garda/page-header";
import { STATUS_EWS, StatusBadge } from "@/components/garda/status-badge";
import { SpringButton } from "@/components/garda/spring-button";
import { PulseRing } from "@/components/motion/pulse-ring";
import { useCan } from "@/lib/auth/session-context";
import { exportCsv } from "@/lib/csv";
import type { EwsAlert, StatusEws } from "@/lib/data/types";
import { severityMeta, toneClass, type Severity } from "@/lib/design/tokens";
import { formatRelatif } from "@/lib/format";
import { useBulkAction, useEwsList } from "@/lib/query/hooks";
import { qk } from "@/lib/query/keys";
import { cn } from "@/lib/utils";
import { JENIS_EWS, JenisIcon, formatNilaiEws } from "./ews-meta";

const KATEGORI = [
  { k: "Pangan pokok", v: [3, 5, 8] }, { k: "Hortikultura", v: [8, 12, 18] }, { k: "Peternakan", v: [5, 8, 12] }, { k: "Perikanan", v: [6, 10, 15] },
];
const KANAL = ["Dashboard", "Push", "Email", "WhatsApp"];

/** Aturan ambang & kanal notifikasi — dapat diubah oleh Unit Data & Statistik. */
function AturanAmbang() {
  const bisa = useCan("ews.atur_ambang");
  const [nilai, setNilai] = useState(KATEGORI.map((x) => x.v));
  const [kanal, setKanal] = useState<string[]>(["Dashboard", "Push", "WhatsApp"]);
  return (
    <section className="glass rounded-xl p-5 sm:p-6">
      <PanelTitle title="Ambang peringatan" description="Δ% harga terhadap rata-rata 7 hari, per kategori" actions={bisa && <SpringButton size="sm" onClick={() => toast.success("Ambang EWS disimpan", { description: "Berlaku untuk deteksi berikutnya; perubahan tercatat di audit trail." })}><Save /> Simpan</SpringButton>} />
      <div className="mt-4 overflow-x-auto">
        <table className="w-full min-w-[420px] text-sm">
          <thead><tr className="text-xs text-muted-foreground"><th className="py-2 text-left font-medium">Kategori</th>{(["WASPADA", "SIAGA", "KRITIS"] as Severity[]).map((s) => <th key={s} className={cn("py-2 font-medium", toneClass[severityMeta[s].tone].text)}>{severityMeta[s].label}</th>)}</tr></thead>
          <tbody>
            {KATEGORI.map((row, i) => (
              <tr key={row.k} className="border-t border-border/60">
                <td className="py-2.5">{row.k}</td>
                {nilai[i].map((v, j) => (
                  <td key={j} className="py-2.5 text-center">
                    <input type="number" disabled={!bisa} value={v} onChange={(e) => setNilai((n) => n.map((r, ri) => (ri === i ? r.map((c, ci) => (ci === j ? Number(e.target.value) : c)) : r)))} className="num h-9 w-16 rounded-lg border border-input bg-white/70 text-center outline-none focus:shadow-glow-data disabled:opacity-60 dark:bg-white/5" aria-label={`${row.k} ambang ${j}`} />
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="mt-5 text-sm font-medium">Kanal notifikasi untuk Siaga & Kritis</p>
      <div className="mt-2 flex flex-wrap gap-2">
        {KANAL.map((k) => (
          <label key={k} className={cn("flex h-9 cursor-pointer items-center gap-2 rounded-xl px-3 text-sm", kanal.includes(k) ? "bg-data-50 text-data-800 dark:bg-data-500/15 dark:text-data-200" : "glass-subtle")}>
            <Checkbox disabled={!bisa} checked={kanal.includes(k)} onCheckedChange={() => setKanal((x) => (x.includes(k) ? x.filter((y) => y !== k) : [...x, k]))} className="rounded-md" />{k}
          </label>
        ))}
      </div>
      {!bisa && <p className="mt-3 text-xs text-muted-foreground">Hanya Unit Data & Statistik yang dapat mengubah ambang.</p>}
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
