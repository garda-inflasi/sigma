"use client";

import { motion } from "motion/react";
import {
  ArrowRightLeft,
  CheckCircle2,
  Download,
  MessageSquareWarning,
} from "lucide-react";
import { useRouter } from "next/navigation";
import { useMemo, type ReactNode } from "react";
import {
  Area,
  CartesianGrid,
  ComposedChart,
  Line,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { toast } from "sonner";
import { GlassTooltip } from "@/components/charts/glass-tooltip";
import { PanelPas } from "@/components/garda/halaman-pas";
import { Istilah } from "@/components/garda/istilah";
import { MetaGrid, MetaItem } from "@/components/garda/inspector";
import { SpringButton } from "@/components/garda/spring-button";
import { StatusBadge } from "@/components/garda/status-badge";
import { useCan } from "@/lib/auth/session-context";
import type { ProyeksiXai } from "@/lib/data/types";
import { garda } from "@/lib/design/tokens";
import { formatPersen, formatRupiah, formatTanggal } from "@/lib/format";
import { ease } from "@/lib/motion/presets";
import { useUnduhProyeksi } from "@/lib/query/hooks";
import { cn } from "@/lib/utils";

/** Grafik proyeksi utama dengan pita ketidakpastian dan aksi tindak lanjut (human-in-the-loop). */
export function PanelProyeksi({
  data,
  memuatUlang,
  aksiModel,
  catatan,
}: {
  data: ProyeksiXai;
  memuatUlang?: boolean;
  /** Tombol pengelola model; hanya diisi untuk peran `model.kelola`. */
  aksiModel?: ReactNode;
  catatan?: string | null;
}) {
  const router = useRouter();
  const bisaKelola = useCan("intervensi.kelola");
  const bisaSetujui = useCan("intervensi.setujui");
  const bisaUnduh = useCan("prediksi.lihat");
  const unduh = useUnduhProyeksi();
  const naik = data.perubahanPct >= 0;
  const warna = naik ? garda.status.defisit.base : garda.status.surplus.base;
  // Titik proyeksi berjarak mingguan: sumbu waktu menjaga 14 hari ke depan tampil sesuai skalanya.
  const seri = useMemo(
    () =>
      data.seri.map((s) => ({
        ...s,
        ts: new Date(`${s.tanggal}T00:00:00`).getTime(),
      })),
    [data.seri],
  );
  const hariIni = seri.find(
    (s) => s.proyeksi !== undefined && s.aktual !== undefined,
  )?.ts;

  return (
    <PanelPas
      judul={`Proyeksi harga ${data.nama}`}
      deskripsi={`Kota Banda Aceh, ${data.horizonHari} hari ke depan`}
      aksi={aksiModel}
      penuh
      isiClassName="flex flex-col gap-2 lg:overflow-hidden"
    >
      <div className="shrink-0">
        <p className="font-display text-display-sm leading-snug font-semibold text-navy-900 dark:text-white">
          {data.nama} diproyeksi {naik ? "naik" : "turun"}{" "}
          <span
            className={cn(
              "num",
              naik
                ? "text-defisit-ink dark:text-rose-300"
                : "text-surplus-ink dark:text-blue-300",
            )}
          >
            {formatPersen(Math.abs(data.perubahanPct))}
          </span>{" "}
          dalam {data.horizonHari} hari
        </p>
        <p className="mt-0.5 text-[13px] text-muted-foreground">
          Rentang keyakinan 80%:{" "}
          <span className="num">
            {formatPersen(data.interval[0], { signed: true })}
          </span>{" "}
          hingga{" "}
          <span className="num">
            {formatPersen(data.interval[1], { signed: true })}
          </span>
          , dari harga kini{" "}
          <span className="num">{formatRupiah(data.hargaKini)}</span>/kg
        </p>
      </div>

      <div
        className={cn(
          "h-64 min-w-0 transition-opacity lg:h-auto lg:min-h-0 lg:flex-1",
          memuatUlang && "opacity-70",
        )}
      >
        <ResponsiveContainer width="100%" height="100%">
          <ComposedChart
            data={seri}
            margin={{ top: 8, right: 8, bottom: 0, left: 0 }}
          >
            <defs>
              <linearGradient id="g-band-prediksi" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor={warna} stopOpacity={0.22} />
                <stop offset="100%" stopColor={warna} stopOpacity={0.06} />
              </linearGradient>
            </defs>
            <CartesianGrid vertical={false} stroke="var(--border)" />
            <XAxis
              dataKey="ts"
              type="number"
              scale="time"
              domain={["dataMin", "dataMax"]}
              tickFormatter={(v: number) => formatTanggal(new Date(v), "d MMM")}
              tick={{ fontSize: 11, fill: "var(--muted-foreground)" }}
              axisLine={false}
              tickLine={false}
              minTickGap={30}
            />
            <YAxis
              tickFormatter={(v) => `${Math.round(v / 1000)}rb`}
              tick={{
                fontSize: 11,
                fill: "var(--muted-foreground)",
                fontFamily: "var(--font-mono)",
              }}
              axisLine={false}
              tickLine={false}
              width={44}
              domain={["auto", "auto"]}
            />
            <Tooltip
              content={
                <GlassTooltip
                  valueFormat={(v) => formatRupiah(v)}
                  labelFormat={(l) =>
                    formatTanggal(new Date(Number(l)), "EEEE, d MMM")
                  }
                />
              }
            />
            {hariIni !== undefined && (
              <ReferenceLine
                x={hariIni}
                stroke={garda.navy[300]}
                strokeDasharray="3 3"
                label={{
                  value: "Hari ini",
                  position: "insideTopLeft",
                  fontSize: 10,
                  fill: "var(--muted-foreground)",
                }}
              />
            )}
            <Area
              dataKey="rentang"
              name="Rentang"
              stroke="none"
              fill="url(#g-band-prediksi)"
              isAnimationActive={false}
            />
            <Line
              dataKey="aktual"
              name="Aktual"
              stroke="var(--foreground)"
              strokeWidth={2}
              dot={false}
              isAnimationActive={false}
            />
            <Line
              dataKey="proyeksi"
              name="Proyeksi"
              stroke={warna}
              strokeWidth={2}
              strokeDasharray="5 4"
              dot={false}
              isAnimationActive={false}
            />
          </ComposedChart>
        </ResponsiveContainer>
      </div>

      {catatan && (
        <p
          title={catatan}
          className="shrink-0 text-xs text-muted-foreground lg:truncate"
        >
          {catatan}
        </p>
      )}
      <div className="flex shrink-0 flex-wrap gap-2 border-t border-border pt-2.5">
        {bisaKelola && (
          <SpringButton
            size="sm"
            variant="primary"
            onClick={() => {
              toast.success("Kartu intervensi dibuat", {
                description: `${data.nama} masuk ke tahap EWS terpicu.`,
              });
              router.push("/intervensi?siklus=tekanan");
            }}
          >
            <ArrowRightLeft /> Teruskan ke intervensi
          </SpringButton>
        )}
        {bisaSetujui && (
          <SpringButton
            size="sm"
            variant="data"
            onClick={() =>
              toast.success("Rekomendasi disetujui", {
                description: "Keputusan tercatat di audit trail.",
              })
            }
          >
            <CheckCircle2 /> Setujui rekomendasi
          </SpringButton>
        )}
        {bisaUnduh && (
          <SpringButton
            size="sm"
            variant="soft"
            disabled={unduh.isPending}
            onClick={() => unduh.mutate(data.komoditasKode)}
          >
            <Download /> Unduh CSV
          </SpringButton>
        )}
        <SpringButton
          size="sm"
          variant="soft"
          onClick={() =>
            toast("Permintaan kajian dikirim ke Unit Data & Statistik")
          }
        >
          <MessageSquareWarning /> Minta kajian ulang
        </SpringButton>
      </div>
    </PanelPas>
  );
}

/** Penjelasan model dalam bahasa sederhana: ringkasan, faktor pendorong, saran, dan identitas model. */
export function PanelPenjelasan({ data }: { data: ProyeksiXai }) {
  const maks = Math.max(...data.faktor.map((f) => Math.abs(f.kontribusi)), 0.1);
  return (
    <PanelPas
      judul="Mengapa harga bergerak?"
      deskripsi="Faktor utama proyeksi ini"
      aksi={data.demo && <StatusBadge tone="waspada">Data demo</StatusBadge>}
    >
      <div className="space-y-4">
        <p className="text-[13px] leading-relaxed">{data.narasi}</p>

        <ul aria-label="Faktor proyeksi" className="space-y-3">
          {data.faktor.map((f, i) => {
            const naik = f.kontribusi >= 0;
            return (
              <li key={f.nama}>
                <div className="flex items-baseline justify-between gap-2 text-[13px]">
                  <span className="min-w-0 truncate font-medium" title={f.nama}>
                    {f.nama}
                  </span>
                  <span
                    className={cn(
                      "num shrink-0 text-xs font-semibold",
                      naik
                        ? "text-defisit-ink dark:text-rose-300"
                        : "text-surplus-ink dark:text-blue-300",
                    )}
                  >
                    {naik ? "+" : "−"}
                    {Math.abs(f.kontribusi).toFixed(1).replace(".", ",")} poin
                  </span>
                </div>
                <div className="relative mt-1 h-1.5 rounded-full bg-mist-100 dark:bg-white/5">
                  <div className="absolute inset-y-0 left-1/2 w-px bg-navy-900/15 dark:bg-white/20" />
                  <motion.div
                    key={`${data.komoditasKode}-${f.nama}`}
                    className={cn(
                      "absolute inset-y-0 rounded-full",
                      naik ? "left-1/2 bg-defisit" : "right-1/2 bg-surplus",
                    )}
                    initial={{ width: 0 }}
                    animate={{
                      width: `${(Math.abs(f.kontribusi) / maks) * 50}%`,
                    }}
                    transition={{
                      duration: 0.8,
                      ease: ease.garda,
                      delay: i * 0.07,
                    }}
                  />
                </div>
                <p className="mt-1 text-xs leading-snug text-muted-foreground">
                  {naik ? "Mendorong harga naik. " : "Menahan harga. "}
                  {f.detail}
                </p>
              </li>
            );
          })}
        </ul>

        {data.rekomendasi.length > 0 && (
          <div>
            <p className="text-[13px] font-medium">Saran langkah</p>
            <ul className="mt-1.5 space-y-1 text-[13px]">
              {data.rekomendasi.map((r) => (
                <li key={r} className="flex gap-2">
                  <CheckCircle2 className="mt-0.5 size-3.5 shrink-0 text-data-600" />
                  {r}
                </li>
              ))}
            </ul>
          </div>
        )}

        <div className="border-t border-border pt-3">
          <MetaGrid>
            <MetaItem label="Model">{data.model.nama}</MetaItem>
            <MetaItem label="Versi">
              <span className="num">{data.model.versi}</span>
            </MetaItem>
            <MetaItem label="Rata-rata meleset">
              <Istilah kunci="mape">
                <span className="num">{formatPersen(data.model.mape)}</span>
              </Istilah>
            </MetaItem>
            <MetaItem label="Dilatih ulang">
              {formatTanggal(data.model.dilatih)}
            </MetaItem>
          </MetaGrid>
        </div>
      </div>
    </PanelPas>
  );
}
