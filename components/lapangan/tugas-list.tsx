"use client";

import { motion } from "motion/react";
import { BellRing, CheckCheck, CloudDownload, MapPin, Play } from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Checkbox } from "@/components/ui/checkbox";
import { BulkActionBar, type BulkAction } from "@/components/data-table/bulk-action-bar";
import { KomoditasIcon } from "@/components/garda/komoditas-icon";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import { SpringLink } from "@/components/garda/spring-button";
import { CountUp } from "@/components/motion/count-up";
import type { TugasLapangan } from "@/lib/data/types";
import { formatRelatif } from "@/lib/format";
import { ease, fadeUp, stagger } from "@/lib/motion/presets";
import { useTugasSaya } from "@/lib/query/hooks";
import { cn } from "@/lib/utils";

const STATUS = {
  BELUM: { label: "Belum mulai", tone: "netral" },
  BERJALAN: { label: "Berjalan", tone: "data" },
  SELESAI: { label: "Selesai", tone: "normal" },
  TERLAMBAT: { label: "Terlambat", tone: "defisit" },
} as const;

function Cincin({ pct }: { pct: number }) {
  const r = 34;
  const c = 2 * Math.PI * r;
  return (
    <svg viewBox="0 0 80 80" className="size-20 -rotate-90" aria-hidden>
      <circle cx="40" cy="40" r={r} fill="none" strokeWidth="8" className="stroke-mist-200 dark:stroke-white/10" />
      <motion.circle cx="40" cy="40" r={r} fill="none" strokeWidth="8" strokeLinecap="round" className="stroke-data-500" strokeDasharray={c} initial={{ strokeDashoffset: c }} animate={{ strokeDashoffset: c * (1 - pct / 100) }} transition={{ duration: 1.1, ease: ease.garda }} />
    </svg>
  );
}

/** Daftar tugas survei: progres, tenggat, pengingat, mulai cepat ke form input. */
export function TugasList() {
  const { data, isLoading } = useTugasSaya();
  const [tugas, setTugas] = useState<TugasLapangan[]>([]);
  const [sel, setSel] = useState<string[]>([]);
  useEffect(() => { if (data) setTugas(data); }, [data]);

  const aktif = tugas.filter((t) => t.status !== "SELESAI");
  const target = tugas.reduce((s, t) => s + t.target, 0);
  const terisi = tugas.reduce((s, t) => s + Math.min(t.terisi, t.target), 0);
  const pct = target ? (terisi / target) * 100 : 0;
  const mendesak = aktif.filter((t) => new Date(t.tenggat).getTime() - Date.now() < 3 * 3_600_000).sort((a, b) => +new Date(a.tenggat) - +new Date(b.tenggat))[0];

  const actions: BulkAction<TugasLapangan>[] = [
    { id: "unduh", label: "Unduh data acuan", icon: CloudDownload, tone: "data", run: async (r) => { await new Promise((ok) => setTimeout(ok, 700)); toast.success(`Data acuan ${r.length} tugas tersimpan offline`, { description: "Median harga, daftar pedagang, dan geofence pasar." }); } },
    {
      id: "selesai", label: "Tandai selesai", icon: CheckCheck,
      enabled: (r) => r.every((t) => t.terisi >= t.target && t.status !== "SELESAI"),
      run: (r) => { setTugas((ts) => ts.map((t) => (r.some((x) => x.id === t.id) ? { ...t, status: "SELESAI" } : t))); toast.success(`${r.length} tugas ditandai selesai`); },
    },
  ];

  if (isLoading) return <div className="space-y-3"><Shimmer className="h-28 w-full rounded-xl" />{Array.from({ length: 3 }).map((_, i) => <Shimmer key={i} className="h-36 w-full rounded-xl" />)}</div>;

  return (
    <div className="space-y-4">
      <h1 className="text-display-sm font-semibold">Tugas survei</h1>
      <section className="glass flex items-center gap-4 rounded-xl p-4">
        <div className="relative">
          <Cincin pct={pct} />
          <span className="num absolute inset-0 grid place-items-center text-sm font-semibold">{Math.round(pct)}%</span>
        </div>
        <div>
          <p className="text-sm text-muted-foreground">Komoditas tercatat</p>
          <p className="text-2xl font-semibold"><CountUp value={terisi} /> <span className="num text-base text-muted-foreground">/ {target}</span></p>
          <p className="text-xs text-muted-foreground"><span className="num">{aktif.length}</span> tugas aktif</p>
        </div>
      </section>

      {mendesak && (
        <div className="flex gap-3 rounded-xl bg-waspada-soft/80 p-3.5 text-sm text-waspada-ink dark:bg-waspada/15 dark:text-amber-200">
          <BellRing className="mt-0.5 size-4 shrink-0" />
          <p><b>{mendesak.judul}</b> di {mendesak.pasarNama} berakhir {formatRelatif(mendesak.tenggat)}.</p>
        </div>
      )}

      <motion.ul variants={stagger(0.06)} initial="hidden" animate="show" className="space-y-3">
        {tugas.map((t) => {
          const s = STATUS[t.status];
          const p = Math.min(100, (t.terisi / t.target) * 100);
          const checked = sel.includes(t.id);
          return (
            <motion.li key={t.id} variants={fadeUp} className={cn("glass rounded-xl p-4 transition-shadow", checked && "ring-2 ring-data-400")}>
              <div className="flex items-start gap-3">
                <Checkbox checked={checked} onCheckedChange={() => setSel((x) => (checked ? x.filter((i) => i !== t.id) : [...x, t.id]))} aria-label={`Pilih ${t.judul}`} className="mt-1 rounded-md" />
                <div className="min-w-0 flex-1">
                  <div className="flex items-center justify-between gap-2">
                    <p className="font-medium">{t.judul}</p>
                    <StatusBadge tone={s.tone}>{s.label}</StatusBadge>
                  </div>
                  <p className="mt-1 flex items-center gap-1 text-xs text-muted-foreground"><MapPin className="size-3.5" />{t.pasarNama}, harga {t.tipe === "KONSUMEN" ? "konsumen" : "produsen"}</p>
                  <div className="mt-3 flex flex-wrap gap-1.5">
                    {t.komoditas.map((k) => <span key={k} className="grid size-7 place-items-center rounded-lg bg-navy-50 text-navy-700 dark:bg-white/10 dark:text-navy-200"><KomoditasIcon kode={k} className="size-3.5" /></span>)}
                  </div>
                  <div className="mt-3 flex items-center gap-2">
                    <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-mist-200 dark:bg-white/10">
                      <motion.div className={cn("h-full rounded-full", t.status === "TERLAMBAT" ? "bg-defisit" : t.status === "SELESAI" ? "bg-normal" : "bg-data-500")} initial={{ width: 0 }} animate={{ width: `${p}%` }} transition={{ duration: 0.8, ease: ease.garda }} />
                    </div>
                    <span className="num text-xs text-muted-foreground">{t.terisi}/{t.target}</span>
                  </div>
                  <div className="mt-3 flex items-center justify-between">
                    <span className={cn("text-xs", t.status === "TERLAMBAT" ? "text-defisit-ink" : "text-muted-foreground")}>Tenggat {formatRelatif(t.tenggat)}</span>
                    {t.status !== "SELESAI" && (
                      <SpringLink size="sm" href={`/lapangan/input?pasar=${t.pasarId}&komoditas=${t.komoditas.join(",")}&tipe=${t.tipe}&tugas=${t.id}`}>
                        <Play /> {t.terisi ? "Lanjutkan" : "Mulai survei"}
                      </SpringLink>
                    )}
                  </div>
                </div>
              </div>
            </motion.li>
          );
        })}
      </motion.ul>
      <BulkActionBar rows={tugas.filter((t) => sel.includes(t.id))} actions={actions} entityLabel="tugas" onClear={() => setSel([])} placement="inline" className="fixed bottom-[96px] left-1/2 -translate-x-1/2" />
    </div>
  );
}
