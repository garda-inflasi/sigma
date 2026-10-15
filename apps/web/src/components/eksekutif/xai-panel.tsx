"use client";

import { motion } from "motion/react";
import { ArrowRightLeft, BrainCircuit, CheckCircle2, MessageSquareWarning } from "lucide-react";
import { useRouter } from "next/navigation";
import { Area, CartesianGrid, ComposedChart, Line, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { toast } from "sonner";
import { GlassTooltip } from "@/components/charts/glass-tooltip";
import { PanelTitle } from "@/components/garda/page-header";
import { Segmented } from "@/components/garda/segmented";
import { PanelSkeleton, Shimmer } from "@/components/garda/shimmer";
import { SpringButton } from "@/components/garda/spring-button";
import { useCan } from "@/lib/auth/session-context";
import { komoditasPilot } from "@/lib/api/mock";
import { garda } from "@/lib/design/tokens";
import { formatPersen, formatRupiah, formatTanggal } from "@/lib/format";
import { ease } from "@/lib/motion/presets";
import { useXai } from "@/lib/query/hooks";
import { cn } from "@/lib/utils";

/**
 * Panel Explainable AI: proyeksi + pita ketidakpastian, kontribusi faktor,
 * narasi, kartu model, dan aksi human-in-the-loop.
 */
export function XaiPanel({ kode, onKodeChange, className }: { kode: string; onKodeChange: (k: string) => void; className?: string }) {
  const { data, isLoading, isFetching } = useXai(kode);
  const router = useRouter();
  const bisaKelola = useCan("intervensi.kelola");
  const bisaSetujui = useCan("intervensi.setujui");

  if (isLoading || !data) return <PanelSkeleton className={className} />;
  const naik = data.perubahanPct >= 0;
  const maxK = Math.max(...data.faktor.map((f) => Math.abs(f.kontribusi)), 0.1);
  const hariIni = data.seri.find((s) => s.proyeksi !== undefined && s.aktual !== undefined)?.tanggal;

  return (
    <section className={cn("glass rounded-xl p-5 sm:p-6", className)}>
      <PanelTitle
        title="Proyeksi harga & penjelasan model"
        description="Mengapa model memperkirakan pergerakan ini"
        actions={
          <Segmented
            layoutId="xai-komoditas"
            size="sm"
            value={kode}
            onChange={onKodeChange}
            options={komoditasPilot.map((k) => ({ value: k.kode, label: k.nama.replace(" keriting", "") }))}
          />
        }
      />

      <div className={cn("mt-5 grid gap-6 transition-opacity lg:grid-cols-[1.25fr_1fr]", isFetching && "opacity-70")}>
        <div>
          <p className="font-display text-display-sm leading-snug font-semibold text-navy-900 dark:text-white">
            {data.nama} diproyeksi {naik ? "naik" : "turun"}{" "}
            <span className={cn("num", naik ? "text-defisit-ink dark:text-rose-300" : "text-surplus-ink dark:text-blue-300")}>{formatPersen(Math.abs(data.perubahanPct))}</span> dalam {data.horizonHari} hari
          </p>
          <p className="mt-1 text-sm text-muted-foreground">
            Rentang keyakinan 80%: <span className="num">{formatPersen(data.interval[0], { signed: true })}</span> hingga <span className="num">{formatPersen(data.interval[1], { signed: true })}</span>, dari harga kini <span className="num">{formatRupiah(data.hargaKini)}</span>/kg
          </p>
          <div className="mt-4 h-[230px]">
            <ResponsiveContainer width="100%" height="100%">
              <ComposedChart data={data.seri} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
                <defs>
                  <linearGradient id="g-band" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor={naik ? garda.status.defisit.base : garda.status.surplus.base} stopOpacity={0.22} />
                    <stop offset="100%" stopColor={naik ? garda.status.defisit.base : garda.status.surplus.base} stopOpacity={0.06} />
                  </linearGradient>
                </defs>
                <CartesianGrid vertical={false} stroke="var(--border)" />
                <XAxis dataKey="tanggal" tickFormatter={(v) => formatTanggal(v, "d MMM")} tick={{ fontSize: 11, fill: "var(--muted-foreground)" }} axisLine={false} tickLine={false} minTickGap={30} />
                <YAxis tickFormatter={(v) => `${Math.round(v / 1000)}rb`} tick={{ fontSize: 11, fill: "var(--muted-foreground)", fontFamily: "var(--font-mono)" }} axisLine={false} tickLine={false} width={44} domain={["auto", "auto"]} />
                <Tooltip content={<GlassTooltip valueFormat={(v) => formatRupiah(v)} />} />
                {hariIni && <ReferenceLine x={hariIni} stroke={garda.navy[300]} strokeDasharray="3 3" label={{ value: "Hari ini", position: "insideTopLeft", fontSize: 10, fill: garda.navy[500] }} />}
                <Area dataKey="rentang" name="Rentang" stroke="none" fill="url(#g-band)" animationDuration={1200} />
                <Line dataKey="aktual" name="Aktual" stroke={garda.navy[700]} strokeWidth={2} dot={false} animationDuration={1100} />
                <Line dataKey="proyeksi" name="Proyeksi" stroke={naik ? garda.status.defisit.base : garda.status.surplus.base} strokeWidth={2} strokeDasharray="5 4" dot={false} animationBegin={900} animationDuration={900} />
              </ComposedChart>
            </ResponsiveContainer>
          </div>
        </div>

        <div className="flex flex-col">
          <p className="text-sm font-medium">Kontribusi faktor (poin persen)</p>
          <ul className="mt-3 space-y-3">
            {data.faktor.map((f, i) => {
              const pos = f.kontribusi >= 0;
              return (
                <li key={f.nama}>
                  <div className="flex items-baseline justify-between gap-2 text-sm">
                    <span className="truncate">{f.nama}</span>
                    <span className={cn("num text-xs font-semibold", pos ? "text-defisit-ink dark:text-rose-300" : "text-surplus-ink dark:text-blue-300")}>{pos ? "+" : "−"}{Math.abs(f.kontribusi).toFixed(1).replace(".", ",")}</span>
                  </div>
                  <div className="relative mt-1.5 h-2 rounded-full bg-mist-100 dark:bg-white/5">
                    <div className="absolute inset-y-0 left-1/2 w-px bg-navy-900/15 dark:bg-white/20" />
                    <motion.div
                      key={`${data.komoditasKode}-${f.nama}`}
                      className={cn("absolute inset-y-0 rounded-full", pos ? "left-1/2 bg-defisit" : "right-1/2 bg-surplus")}
                      initial={{ width: 0 }}
                      animate={{ width: `${(Math.abs(f.kontribusi) / maxK) * 50}%` }}
                      transition={{ duration: 0.8, ease: ease.garda, delay: i * 0.07 }}
                    />
                  </div>
                  <p className="mt-1 text-xs text-muted-foreground">{f.detail}</p>
                </li>
              );
            })}
          </ul>
        </div>
      </div>

      <div className="mt-6 grid gap-4 lg:grid-cols-[1.25fr_1fr]">
        <div className="rounded-xl bg-data-50/70 p-4 dark:bg-data-500/10">
          <p className="flex items-center gap-2 text-sm font-medium text-data-800 dark:text-data-200"><BrainCircuit className="size-4" /> Ringkasan model</p>
          <p className="mt-1.5 text-sm leading-relaxed text-data-900/80 dark:text-data-100/80">{data.narasi}</p>
          <ul className="mt-3 space-y-1 text-sm">
            {data.rekomendasi.map((r) => <li key={r} className="flex gap-2"><CheckCircle2 className="mt-0.5 size-4 shrink-0 text-data-600" /> {r}</li>)}
          </ul>
        </div>
        <div className="flex flex-col justify-between gap-4 rounded-xl border border-border/70 p-4">
          <dl className="grid grid-cols-2 gap-3 text-sm">
            <div><dt className="text-xs text-muted-foreground">Model</dt><dd className="mt-0.5 font-medium">{data.model.nama}</dd></div>
            <div><dt className="text-xs text-muted-foreground">Versi</dt><dd className="num mt-0.5">{data.model.versi}</dd></div>
            <div><dt className="text-xs text-muted-foreground">MAPE 30 hari</dt><dd className="num mt-0.5">{formatPersen(data.model.mape)}</dd></div>
            <div><dt className="text-xs text-muted-foreground">Dilatih ulang</dt><dd className="mt-0.5">{formatTanggal(data.model.dilatih)}</dd></div>
          </dl>
          <div className="flex flex-wrap gap-2">
            {bisaKelola && (
              <SpringButton size="sm" variant="primary" onClick={() => { toast.success("Kartu intervensi dibuat", { description: `${data.nama} masuk ke tahap EWS terpicu.` }); router.push("/intervensi?siklus=tekanan"); }}>
                <ArrowRightLeft /> Teruskan ke intervensi
              </SpringButton>
            )}
            {bisaSetujui && (
              <SpringButton size="sm" variant="data" onClick={() => toast.success("Rekomendasi disetujui", { description: "Keputusan tercatat di audit trail." })}>
                <CheckCircle2 /> Setujui rekomendasi
              </SpringButton>
            )}
            <SpringButton size="sm" variant="soft" onClick={() => toast("Permintaan kajian dikirim ke Unit Data & Statistik")}>
              <MessageSquareWarning /> Minta kajian ulang
            </SpringButton>
          </div>
        </div>
      </div>
    </section>
  );
}

export function XaiPanelSkeleton() {
  return <Shimmer className="h-[640px] w-full rounded-xl" />;
}
