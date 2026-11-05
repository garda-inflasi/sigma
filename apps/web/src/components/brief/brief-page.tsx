"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { AnimatePresence, motion } from "motion/react";
import { CheckCircle2, FileText, Loader2, Send, Sparkles, TriangleAlert } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DataTable } from "@/components/data-table/data-table";
import { PageHeader } from "@/components/garda/page-header";
import { Shimmer } from "@/components/garda/shimmer";
import { STATUS_BRIEF as STATUS, StatusBadge } from "@/components/garda/status-badge";
import { SpringButton } from "@/components/garda/spring-button";
import { periodeBulanIni, type PolicyBriefDetail } from "@/lib/api/brief";
import { useCan } from "@/lib/auth/session-context";
import type { PolicyBrief } from "@/lib/data/types";
import { formatRelatif } from "@/lib/format";
import { fadeUp, stagger } from "@/lib/motion/presets";
import { useBriefDetail, useBriefs, useBulkAction, useSusunBrief } from "@/lib/query/hooks";
import { qk } from "@/lib/query/keys";

/** Pratinjau draf: label wajib, bagian bernarasi, jejak model, dan sitasi sumber tiap angka. */
function PratinjauBrief({ brief }: { brief: PolicyBriefDetail }) {
  return (
    <motion.article variants={stagger(0.08)} initial="hidden" animate="show" className="space-y-4 rounded-xl border border-border/70 bg-white/60 p-5 text-sm leading-relaxed dark:bg-white/5">
      <motion.div variants={fadeUp} className="flex items-start justify-between gap-3">
        <h3 className="text-display-sm font-semibold">{brief.judul}</h3>
        <StatusBadge tone={STATUS[brief.status].tone}>{STATUS[brief.status].label}</StatusBadge>
      </motion.div>
      <motion.p variants={fadeUp} role="note" className="rounded-lg bg-waspada-soft/60 px-3 py-2 text-xs font-medium text-waspada-ink dark:bg-waspada/10">
        {brief.labelDraf ?? "Draf otomatis — wajib ditelaah manusia"}
      </motion.p>
      {brief.catatan && <motion.p variants={fadeUp} className="text-xs text-muted-foreground">{brief.catatan}</motion.p>}
      {brief.bagian.map((b) => (
        <motion.section key={b.kode} variants={fadeUp}>
          <p className="font-semibold">{b.judul}</p>
          {b.butir.length === 1 ? <p className="text-muted-foreground">{b.butir[0]}</p> : <ul className="list-disc space-y-1 pl-5 text-muted-foreground">{b.butir.map((t) => <li key={t}>{t}</li>)}</ul>}
        </motion.section>
      ))}
      {brief.sitasi.length > 0 && (
        <motion.details variants={fadeUp} className="text-xs text-muted-foreground">
          <summary className="cursor-pointer font-medium text-foreground">Sumber data ({brief.sitasi.length})</summary>
          <ul className="mt-2 space-y-1">{brief.sitasi.map((s) => <li key={s.id}>{s.label}: <span className="num font-medium text-foreground">{s.nilai}</span> — {s.sumber}</li>)}</ul>
        </motion.details>
      )}
      <motion.p variants={fadeUp} className="text-xs text-muted-foreground">
        Disusun oleh {brief.disusunOleh}.{brief.model ? ` Model ${brief.model}, prompt ${brief.promptVersi ?? "-"} (hash ${brief.promptHash?.slice(0, 12) ?? "-"}).` : ""} Draf tersimpan dan tampil di daftar brief.
      </motion.p>
    </motion.article>
  );
}

/**
 * Generator policy brief: server menyusun draf di latar belakang dari data nyata (inflasi, harga, peringatan dini,
 * proyeksi); dialog memantau sampai selesai. Narasi ditulis AI hanya bila LLM aktif di server, selain itu dari
 * templat deterministik; angka selalu dari data dan wajib ditelaah manusia.
 */
function GeneratorBrief({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const susun = useSusunBrief();
  const periode = periodeBulanIni();
  const label = useMemo(() => new Date(`${periode}-01T00:00:00`).toLocaleDateString("id-ID", { month: "long", year: "numeric" }), [periode]);
  const { reset } = susun;
  useEffect(() => { if (!open) reset(); }, [open, reset]);
  const detail = useBriefDetail(susun.data?.id);
  const brief = detail.data;
  const status = brief?.status ?? susun.data?.status;
  // Sesaat setelah "Coba lagi", detail GAGAL lama masih di cache sampai terambil ulang: anggap masih disusun.
  const menyusun =
    susun.isPending ||
    (susun.isSuccess && (status === "DIPROSES" || (!brief && !detail.isError) || (brief?.status === "GAGAL" && detail.isFetching)));
  const qc = useQueryClient();
  // Daftar brief ikut diperbarui begitu penyusunan selesai atau gagal.
  useEffect(() => { if (status && status !== "DIPROSES") qc.invalidateQueries({ queryKey: qk.brief.list() }); }, [status, qc]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="glass-strong max-h-[90dvh] overflow-y-auto rounded-xl border-0 sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Draf policy brief {brief?.periode ?? label}</DialogTitle>
          <DialogDescription>Draf disusun dari data inflasi, harga, peringatan dini, dan proyeksi di sistem. Bila LLM aktif narasinya ditulis AI; semua angka tetap diambil dari data. Isi wajib ditelaah manusia sebelum diajukan.</DialogDescription>
        </DialogHeader>
        <AnimatePresence mode="wait">
          {menyusun ? (
            <motion.div key="s" exit={{ opacity: 0 }} role="status" className="space-y-2.5 rounded-xl border border-border/70 p-5">
              <p className="flex items-center gap-2 text-sm text-muted-foreground"><Loader2 className="size-4 animate-spin text-data-600" /> Menyusun draf dari data terbaru. Anda boleh menutup dialog; draf tetap tersimpan.</p>
              <Shimmer className="h-5 w-2/3" /><Shimmer className="h-3.5 w-full" /><Shimmer className="h-3.5 w-4/5" />
            </motion.div>
          ) : brief && brief.status !== "GAGAL" ? (
            <PratinjauBrief key="d" brief={brief} />
          ) : (
            <div key="i" className="space-y-2 rounded-xl border border-border/70 p-5 text-sm text-muted-foreground">
              {brief?.status === "GAGAL" ? (
                <p role="alert" className="flex items-start gap-2 text-defisit-ink"><TriangleAlert className="mt-0.5 size-4 shrink-0" /> Penyusunan draf gagal{brief.galat ? `: ${brief.galat}` : "."}</p>
              ) : (
                <p>Menyusun draf untuk periode <span className="font-medium text-foreground">{label}</span> atas nama Anda. Satu periode hanya dapat memiliki satu brief.</p>
              )}
              {(susun.isError || detail.isError) && <p role="alert" className="text-defisit-ink">{(susun.error ?? detail.error) instanceof Error ? (susun.error ?? detail.error)!.message : "Draf gagal disusun."}</p>}
            </div>
          )}
        </AnimatePresence>
        <div className="flex justify-end gap-2">
          {brief && brief.status !== "GAGAL" && !menyusun ? (
            <SpringButton onClick={() => onOpenChange(false)}>Selesai</SpringButton>
          ) : (
            <>
              <SpringButton variant="soft" onClick={() => onOpenChange(false)}>{menyusun ? "Tutup" : "Batal"}</SpringButton>
              <SpringButton disabled={menyusun} onClick={() => susun.mutate(periode)}><Sparkles /> {susun.isError || brief?.status === "GAGAL" ? "Coba lagi" : "Susun draf"}</SpringButton>
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
