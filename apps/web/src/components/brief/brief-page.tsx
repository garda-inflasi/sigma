"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { AnimatePresence, motion } from "motion/react";
import { CheckCircle2, FileText, Loader2, Send, Sparkles } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DataTable } from "@/components/data-table/data-table";
import { PageHeader } from "@/components/garda/page-header";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import { SpringButton } from "@/components/garda/spring-button";
import { periodeBulanIni } from "@/lib/api/brief";
import { useCan } from "@/lib/auth/session-context";
import type { PolicyBrief } from "@/lib/data/types";
import { formatRelatif } from "@/lib/format";
import { fadeUp, stagger } from "@/lib/motion/presets";
import { useBriefs, useBulkAction, useSusunBrief } from "@/lib/query/hooks";
import { qk } from "@/lib/query/keys";

const STATUS = { DRAF: { tone: "netral", label: "Draf" }, TELAAH: { tone: "waspada", label: "Ditelaah" }, DISETUJUI: { tone: "normal", label: "Disetujui" }, DIDISTRIBUSIKAN: { tone: "data", label: "Didistribusikan" } } as const;
/** Generator policy brief: menyimpan draf kerangka dari templat otomatis (tanpa LLM) lalu menampilkan hasilnya. */
function GeneratorBrief({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const susun = useSusunBrief();
  const periode = periodeBulanIni();
  const label = useMemo(() => new Date(`${periode}-01T00:00:00`).toLocaleDateString("id-ID", { month: "long", year: "numeric" }), [periode]);
  const { reset } = susun;
  useEffect(() => { if (!open) reset(); }, [open, reset]);
  const brief = susun.data;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="glass-strong max-h-[90dvh] overflow-y-auto rounded-xl border-0 sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Draf policy brief {brief?.periode ?? label}</DialogTitle>
          <DialogDescription>Draf otomatis dari templat, bukan hasil LLM. Isi narasi, angka, dan rekomendasi wajib dilengkapi serta ditelaah manusia sebelum diajukan.</DialogDescription>
        </DialogHeader>
        <AnimatePresence mode="wait">
          {susun.isPending ? (
            <motion.div key="s" exit={{ opacity: 0 }} className="space-y-2.5 rounded-xl border border-border/70 p-5">
              <Loader2 className="size-4 animate-spin text-data-600" /><Shimmer className="h-5 w-2/3" /><Shimmer className="h-3.5 w-full" /><Shimmer className="h-3.5 w-4/5" />
            </motion.div>
          ) : brief ? (
            <motion.article key="d" variants={stagger(0.08)} initial="hidden" animate="show" className="space-y-4 rounded-xl border border-border/70 bg-white/60 p-5 text-sm leading-relaxed dark:bg-white/5">
              <motion.div variants={fadeUp} className="flex items-start justify-between gap-3">
                <h3 className="text-display-sm font-semibold">{brief.judul}</h3>
                <StatusBadge tone={STATUS[brief.status].tone}>{STATUS[brief.status].label}</StatusBadge>
              </motion.div>
              <motion.section variants={fadeUp}><p className="font-semibold">Ringkasan</p><p className="text-muted-foreground">{brief.ringkasan}</p></motion.section>
              <motion.p variants={fadeUp} className="text-xs text-muted-foreground">Disusun oleh {brief.disusunOleh}. Draf tersimpan dan tampil di daftar brief.</motion.p>
            </motion.article>
          ) : (
            <div key="i" className="space-y-2 rounded-xl border border-border/70 p-5 text-sm text-muted-foreground">
              <p>Menyusun kerangka draf untuk periode <span className="font-medium text-foreground">{label}</span> atas nama Anda. Satu periode hanya dapat memiliki satu brief.</p>
              {susun.isError && <p role="alert" className="text-defisit-ink">{susun.error instanceof Error ? susun.error.message : "Draf gagal disusun."}</p>}
            </div>
          )}
        </AnimatePresence>
        <div className="flex justify-end gap-2">
          {brief ? (
            <SpringButton onClick={() => onOpenChange(false)}>Selesai</SpringButton>
          ) : (
            <>
              <SpringButton variant="soft" onClick={() => onOpenChange(false)}>Batal</SpringButton>
              <SpringButton disabled={susun.isPending} onClick={() => susun.mutate(periode)}><Sparkles /> {susun.isError ? "Coba lagi" : "Susun draf"}</SpringButton>
            </>
          )}
        </div>
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
          { id: "setujui", label: "Setujui", icon: CheckCircle2, perm: "brief.setujui", tone: "data", enabled: (r) => r.every((x) => x.status === "TELAAH"), confirm: { title: "Setujui policy brief?", description: (n) => `${n} brief akan berstatus disetujui dan siap didistribusikan. Persetujuan tercatat atas nama Anda.`, actionLabel: "Setujui" }, run: (r) => mut.mutateAsync({ aksi: "setujui", ids: ids(r), label: "Brief disetujui" }) },
        ]} />
      <GeneratorBrief open={gen} onOpenChange={setGen} />
    </div>
  );
}
