"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { AnimatePresence, motion } from "motion/react";
import { Check, CheckCircle2, Download, FileText, Loader2, Send, Sparkles } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DataTable } from "@/components/data-table/data-table";
import { PageHeader } from "@/components/garda/page-header";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import { SpringButton } from "@/components/garda/spring-button";
import { mockXai } from "@/lib/api/mock";
import { useCan } from "@/lib/auth/session-context";
import type { PolicyBrief } from "@/lib/data/types";
import { formatPersen, formatRelatif } from "@/lib/format";
import { fadeUp, popIn, stagger } from "@/lib/motion/presets";
import { useBriefs, useBulkAction } from "@/lib/query/hooks";
import { qk } from "@/lib/query/keys";

const STATUS = { DRAF: { tone: "netral", label: "Draf" }, TELAAH: { tone: "waspada", label: "Ditelaah" }, DISETUJUI: { tone: "normal", label: "Disetujui" }, DIDISTRIBUSIKAN: { tone: "data", label: "Didistribusikan" } } as const;
const LANGKAH = ["Menghimpun harga tervalidasi & data BPS", "Menjalankan proyeksi dan analisis faktor", "Menyusun narasi & rekomendasi aksi", "Menyiapkan grafik dan lampiran data"];

/** Generator policy brief: stepper beranimasi → pratinjau draf yang dapat diajukan. */
function GeneratorBrief({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const [langkah, setLangkah] = useState(0);
  const xai = mockXai("CBM-KRT");
  useEffect(() => {
    if (!open) return setLangkah(0);
    const t = setInterval(() => setLangkah((l) => (l < LANGKAH.length ? l + 1 : l)), 850);
    return () => clearInterval(t);
  }, [open]);
  const selesai = langkah >= LANGKAH.length;
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="glass-strong max-h-[90dvh] overflow-y-auto rounded-xl border-0 sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Draf policy brief Oktober 2026</DialogTitle>
          <DialogDescription>Disusun otomatis dari data tervalidasi. Wajib ditelaah manusia sebelum diajukan.</DialogDescription>
        </DialogHeader>
        <ol className="space-y-2.5">
          {LANGKAH.map((l, i) => (
            <li key={l} className="flex items-center gap-3 text-sm">
              <span className="grid size-6 place-items-center">
                {i < langkah ? <motion.span variants={popIn} initial="hidden" animate="show" className="grid size-6 place-items-center rounded-full bg-normal text-white"><Check className="size-3.5" /></motion.span>
                  : i === langkah ? <Loader2 className="size-4 animate-spin text-data-600" /> : <span className="size-2 rounded-full bg-mist-300" />}
              </span>
              <span className={i <= langkah ? "text-foreground" : "text-muted-foreground"}>{l}</span>
            </li>
          ))}
        </ol>
        <AnimatePresence mode="wait">
          {!selesai ? (
            <motion.div key="s" exit={{ opacity: 0 }} className="space-y-2.5 rounded-xl border border-border/70 p-5">
              <Shimmer className="h-5 w-2/3" /><Shimmer className="h-3.5 w-full" /><Shimmer className="h-3.5 w-11/12" /><Shimmer className="h-3.5 w-4/5" /><Shimmer className="mt-4 h-28 w-full rounded-xl" />
            </motion.div>
          ) : (
            <motion.article key="d" variants={stagger(0.08)} initial="hidden" animate="show" className="space-y-4 rounded-xl border border-border/70 bg-white/60 p-5 text-sm leading-relaxed dark:bg-white/5">
              <motion.h3 variants={fadeUp} className="text-display-sm font-semibold">Meredam tekanan harga cabai pascacurah hujan ekstrem di dataran tinggi Gayo</motion.h3>
              <motion.section variants={fadeUp}><p className="font-semibold">Ringkasan eksekutif</p><p className="text-muted-foreground">Inflasi tahunan Aceh September 2026 sebesar 2,84% masih dalam koridor sasaran. Namun {xai.narasi.charAt(0).toLowerCase() + xai.narasi.slice(1)}</p></motion.section>
              <motion.section variants={fadeUp}><p className="font-semibold">Proyeksi</p><p className="text-muted-foreground">Kenaikan {formatPersen(xai.perubahanPct)} dalam 14 hari (rentang {formatPersen(xai.interval[0], { signed: true })} s.d. {formatPersen(xai.interval[1], { signed: true })}), model {xai.model.nama} {xai.model.versi}.</p></motion.section>
              <motion.section variants={fadeUp}><p className="font-semibold">Rekomendasi aksi</p><ol className="mt-1 list-decimal space-y-1 pl-5 text-muted-foreground">{xai.rekomendasi.map((r) => <li key={r}>{r}</li>)}</ol></motion.section>
              <motion.section variants={fadeUp}><p className="font-semibold">Lampiran</p><p className="text-muted-foreground">Grafik tren 45 hari, peta panas harga, neraca pangan, dan tabel ruang fiskal per 30 September 2026.</p></motion.section>
            </motion.article>
          )}
        </AnimatePresence>
        {selesai && (
          <div className="flex justify-end gap-2">
            <SpringButton variant="soft" onClick={() => onOpenChange(false)}>Simpan sebagai draf</SpringButton>
            <SpringButton onClick={() => { toast.success("Draf diajukan untuk ditelaah Dewan Pengarah"); onOpenChange(false); }}><Send /> Ajukan telaah</SpringButton>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

export function BriefPage() {
  const params = useSearchParams();
  const bisaSusun = useCan("brief.susun");
  const [gen, setGen] = useState(false);
  const { data, isLoading } = useBriefs();
  const mut = useBulkAction<PolicyBrief>({ sumber: "policy-brief", queryKey: qk.brief.list(), patch: (r, a) => ({ ...r, status: a === "telaah" ? "TELAAH" : a === "setujui" ? "DISETUJUI" : r.status }) });
  useEffect(() => { if (params.get("susun") && bisaSusun) setGen(true); }, [params, bisaSusun]);

  const kolom = useMemo<ColumnDef<PolicyBrief>[]>(() => [
    { accessorKey: "periode", header: "Periode", cell: ({ getValue }) => <span className="whitespace-nowrap font-medium">{getValue<string>()}</span> },
    { accessorKey: "judul", header: "Judul", cell: ({ getValue }) => <span className="flex max-w-lg items-start gap-2"><FileText className="mt-0.5 size-4 shrink-0 text-muted-foreground" />{getValue<string>()}</span> },
    { accessorKey: "jumlahRekomendasi", header: "Rekomendasi", cell: ({ getValue }) => <span className="num">{getValue<number>()}</span> },
    { accessorKey: "disusunOleh", header: "Penyusun" },
    { accessorKey: "diperbarui", header: "Diperbarui", cell: ({ getValue }) => <span className="whitespace-nowrap text-muted-foreground">{formatRelatif(getValue<string>())}</span> },
    { accessorKey: "status", header: "Status", filterFn: "equals", cell: ({ getValue }) => { const s = STATUS[getValue<PolicyBrief["status"]>()]; return <StatusBadge tone={s.tone}>{s.label}</StatusBadge>; } },
  ], []);
  const ids = (r: PolicyBrief[]) => r.map((x) => x.id);

  return (
    <div className="mx-auto max-w-[1600px] space-y-6">
      <PageHeader title="Policy brief" description="Draf bulanan disusun otomatis dari temuan AI, ditelaah Unit Hubungan Pemerintah & Kebijakan, lalu disetujui Dewan Pengarah sebelum didistribusikan ke mitra daerah."
        actions={bisaSusun && <SpringButton onClick={() => setGen(true)}><Sparkles /> Susun draf bulan ini</SpringButton>} />
      <DataTable data={data} isLoading={isLoading} columns={kolom} getRowId={(r) => r.id} entityLabel="brief" searchPlaceholder="Cari judul atau periode"
        facets={[{ columnId: "status", label: "Status", options: Object.entries(STATUS).map(([value, v]) => ({ value, label: v.label })) }]}
        bulkActions={[
          { id: "telaah", label: "Ajukan telaah", icon: Send, perm: "brief.susun", enabled: (r) => r.every((x) => x.status === "DRAF"), run: (r) => mut.mutateAsync({ aksi: "telaah", ids: ids(r), label: "Diajukan untuk ditelaah" }) },
          { id: "setujui", label: "Setujui", icon: CheckCircle2, perm: "brief.setujui", tone: "data", enabled: (r) => r.every((x) => x.status === "TELAAH" || x.status === "DRAF"), confirm: { title: "Setujui policy brief?", description: (n) => `${n} brief akan berstatus disetujui dan siap didistribusikan. Persetujuan tercatat atas nama Anda.`, actionLabel: "Setujui" }, run: (r) => mut.mutateAsync({ aksi: "setujui", ids: ids(r), label: "Brief disetujui" }) },
          { id: "unduh", label: "Unduh PDF", icon: Download, run: (r) => { toast.success(`${r.length} PDF sedang disiapkan`, { description: "Tautan unduhan dikirim ke email Anda." }); } },
        ]} />
      <GeneratorBrief open={gen} onOpenChange={setGen} />
    </div>
  );
}
