"use client";

import { motion } from "motion/react";
import { BrainCircuit } from "lucide-react";
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
import { GlassTooltip } from "@/components/charts/glass-tooltip";
import { PanelPas } from "@/components/garda/halaman-pas";
import { Istilah } from "@/components/garda/istilah";
import { StatusBadge } from "@/components/garda/status-badge";
import type {
  BlokModelMl,
  ForecastDetailMl,
  ForecastMl,
  PenjelasanMl,
} from "@/lib/api/ml";
import { garda } from "@/lib/design/tokens";
import { formatAngka, formatPersen, formatRupiah } from "@/lib/format";
import {
  akhiranSatuan,
  HORIZON_ML,
  seriGrafikForecast,
  titikHorizon,
} from "@/lib/ml/forecast";
import {
  alasanBelumAndal,
  dapatDiandalkan,
  LABEL_BELUM_ANDAL,
  LABEL_PERSISTENSI,
  labelModelForecast,
  labelSatuanKontribusi,
  namaKomoditasMl,
  namaModel,
  narasiJujur,
  ringkasPemenang,
  tanggalBisnis,
} from "@/lib/ml/label";
import { ease } from "@/lib/motion/presets";
import { cn } from "@/lib/utils";
import { CatatanBelumAndal, LabelTargetMl } from "./siklus-ml";

const fmtPct = (v: number | null | undefined, signed = true) =>
  v === null || v === undefined ? "–" : formatPersen(v, { signed });
const fmtRp = (v: number | null | undefined) =>
  v === null || v === undefined ? "–" : formatRupiah(v);

/** Grafik harga asli + proyeksi P50 dengan pita P10–P90 pada H+7/14/30. */
export function GrafikForecastMl({
  item,
  className,
}: {
  item: ForecastDetailMl;
  className?: string;
}) {
  const seri = useMemo(() => seriGrafikForecast(item), [item]);
  const t14 = titikHorizon(item.kurva, 14) ?? item.kurva[0];
  const naik = (t14?.perubahanP50Pct ?? 0) >= 0;
  const warna = naik ? garda.status.defisit.base : garda.status.surplus.base;
  const jangkar = item.tanggalTerakhir
    ? Date.parse(`${item.tanggalTerakhir}T00:00:00Z`)
    : undefined;
  const satuan = akhiranSatuan(item.satuan);
  return (
    <div className={cn("min-w-0", className)}>
      <ResponsiveContainer width="100%" height="100%">
        <ComposedChart
          data={seri}
          margin={{ top: 8, right: 8, bottom: 0, left: 0 }}
        >
          <defs>
            <linearGradient id="g-band-ml" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={warna} stopOpacity={0.24} />
              <stop offset="100%" stopColor={warna} stopOpacity={0.08} />
            </linearGradient>
          </defs>
          <CartesianGrid vertical={false} stroke="var(--border)" />
          <XAxis
            dataKey="ts"
            type="number"
            scale="time"
            domain={["dataMin", "dataMax"]}
            tickFormatter={(v: number) =>
              tanggalBisnis(new Date(v).toISOString()).replace(/ \d{4}$/, "")
            }
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
                valueFormat={(v) => `${formatRupiah(v)}${satuan}`}
                labelFormat={(l) =>
                  tanggalBisnis(new Date(Number(l)).toISOString())
                }
              />
            }
          />
          {jangkar !== undefined && (
            <ReferenceLine
              x={jangkar}
              stroke={garda.navy[300]}
              strokeDasharray="3 3"
              label={{
                value: "Harga terakhir",
                position: "insideTopLeft",
                fontSize: 10,
                fill: "var(--muted-foreground)",
              }}
            />
          )}
          <Area
            dataKey="rentang"
            name="Rentang P10–P90"
            stroke="none"
            fill="url(#g-band-ml)"
            isAnimationActive={false}
          />
          <Line
            dataKey="aktual"
            name="Harga asli"
            stroke="var(--foreground)"
            strokeWidth={2}
            dot={false}
            isAnimationActive={false}
          />
          <Line
            dataKey="p50"
            name="Proyeksi P50"
            stroke={warna}
            strokeWidth={2}
            strokeDasharray="5 4"
            dot={{ r: 2.5, strokeWidth: 0, fill: warna }}
            isAnimationActive={false}
          />
        </ComposedChart>
      </ResponsiveContainer>
    </div>
  );
}

/** Kurva ringkas: harga terakhir lalu P50 dan pita P10–P90 pada H+7/14/30, tanpa sumbu. */
export function KurvaMiniMl({
  forecast,
  className,
}: {
  forecast: ForecastMl;
  className?: string;
}) {
  const seri = useMemo(() => {
    const awal = forecast.hargaTerakhir;
    const titik = forecast.kurva
      .filter((t) => t.p50 !== null)
      .map((t) => ({
        h: t.horizon,
        p50: t.p50 as number,
        rentang:
          t.p10 !== null && t.p90 !== null
            ? ([t.p10, t.p90] as [number, number])
            : undefined,
      }));
    return awal === null
      ? titik
      : [
          { h: 0, p50: awal, rentang: [awal, awal] as [number, number] },
          ...titik,
        ];
  }, [forecast]);
  const akhir = titikHorizon(forecast.kurva, 30)?.perubahanP50Pct ?? 0;
  const warna =
    akhir > 0
      ? garda.status.defisit.base
      : akhir < 0
        ? garda.status.surplus.base
        : garda.navy[500];
  const id = `g-mini-${forecast.komoditasKode}-${forecast.kabupatenKode}`;
  return (
    <div aria-hidden className={cn("h-10 min-w-0", className)}>
      <ResponsiveContainer width="100%" height="100%">
        <ComposedChart
          data={seri}
          margin={{ top: 2, right: 2, bottom: 2, left: 2 }}
        >
          <defs>
            <linearGradient id={id} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={warna} stopOpacity={0.28} />
              <stop offset="100%" stopColor={warna} stopOpacity={0.08} />
            </linearGradient>
          </defs>
          <XAxis dataKey="h" type="number" domain={[0, 30]} hide />
          <YAxis domain={["dataMin", "dataMax"]} hide />
          <Area
            dataKey="rentang"
            stroke="none"
            fill={`url(#${id})`}
            isAnimationActive={false}
          />
          <Line
            dataKey="p50"
            stroke={warna}
            strokeWidth={1.75}
            dot={false}
            isAnimationActive={false}
          />
        </ComposedChart>
      </ResponsiveContainer>
    </div>
  );
}

/** P10/P50/P90 per horizon wajib (7, 14, 30 hari), dengan tanggal dan perubahan P50 terhadap harga terakhir. */
export function TabelHorizonMl({
  item,
  className,
}: {
  item: ForecastMl;
  className?: string;
}) {
  const satuan = akhiranSatuan(item.satuan);
  return (
    <div className={cn("overflow-x-auto", className)}>
      <table className="w-full min-w-[320px] text-[13px]">
        <caption className="sr-only">
          Proyeksi kuantil {namaKomoditasMl(item.komoditasKode)} per horizon,
          satuan {item.satuan}
        </caption>
        <thead>
          <tr className="text-left text-xs text-muted-foreground">
            <th className="py-1 pr-2 font-medium">
              <Istilah kunci="rentangKuantil">Kuantil</Istilah>
            </th>
            {HORIZON_ML.map((h) => {
              const t = titikHorizon(item.kurva, h);
              return (
                <th key={h} className="py-1 pl-2 text-right font-medium">
                  H+{h}
                  <span className="block text-[11px] font-normal">
                    {t ? tanggalBisnis(t.tanggal) : "–"}
                  </span>
                </th>
              );
            })}
          </tr>
        </thead>
        <tbody className="num">
          {(["p90", "p50", "p10"] as const).map((k) => (
            <tr key={k} className="border-t border-border">
              <th
                scope="row"
                className={cn(
                  "py-1 pr-2 text-left text-xs font-medium",
                  k === "p50" ? "text-foreground" : "text-muted-foreground",
                )}
              >
                {k.toUpperCase()}
              </th>
              {HORIZON_ML.map((h) => {
                const v = titikHorizon(item.kurva, h)?.[k];
                return (
                  <td
                    key={h}
                    className={cn(
                      "py-1 pl-2 text-right whitespace-nowrap",
                      k === "p50" ? "font-semibold" : "text-muted-foreground",
                    )}
                  >
                    {v === null || v === undefined
                      ? "–"
                      : `${formatRupiah(v)}${satuan}`}
                  </td>
                );
              })}
            </tr>
          ))}
          <tr className="border-t border-border text-xs">
            <th
              scope="row"
              className="py-1 pr-2 text-left font-medium text-muted-foreground"
            >
              Δ P50
            </th>
            {HORIZON_ML.map((h) => (
              <td key={h} className="py-1 pl-2 text-right">
                {fmtPct(titikHorizon(item.kurva, h)?.perubahanP50Pct)}
              </td>
            ))}
          </tr>
        </tbody>
      </table>
    </div>
  );
}

/**
 * Proyeksi harga satu komoditas × kabupaten dari siklus pilihan: kalimat utama (P50 H+14), grafik pita kuantil, dan
 * tabel H+7/14/30. Model baseline ditandai persistensi di samping kalimat utamanya.
 */
export function PanelForecastMl({
  item,
  wilayah,
  memuatUlang,
  aksi,
  kaki,
}: {
  item: ForecastDetailMl;
  wilayah: string;
  memuatUlang?: boolean;
  aksi?: ReactNode;
  kaki?: ReactNode;
}) {
  const nama = namaKomoditasMl(item.komoditasKode);
  const t14 = titikHorizon(item.kurva, 14);
  const pct = t14?.perubahanP50Pct ?? null;
  const naik = (pct ?? 0) >= 0;
  const satuan = akhiranSatuan(item.satuan);
  return (
    <PanelPas
      judul={`Proyeksi harga ${nama}`}
      deskripsi={`${wilayah}, kuantil P10–P50–P90 untuk 7, 14, dan 30 hari`}
      aksi={aksi}
      penuh
      aria-busy={memuatUlang || undefined}
      isiClassName="flex flex-col gap-2 lg:overflow-hidden"
    >
      <div className="shrink-0">
        <p className="font-display text-display-sm leading-snug font-semibold text-navy-900 dark:text-white">
          {pct === null ? (
            <>Proyeksi P50 14 hari belum tersedia</>
          ) : (
            <>
              P50 14 hari {pct === 0 ? "tetap" : naik ? "naik" : "turun"}{" "}
              <span
                className={cn(
                  "num",
                  pct === 0
                    ? ""
                    : naik
                      ? "text-defisit-ink dark:text-rose-300"
                      : "text-surplus-ink dark:text-blue-300",
                )}
              >
                {formatPersen(Math.abs(pct))}
              </span>{" "}
              menjadi <span className="num">{fmtRp(t14?.p50)}</span>
              {satuan}
            </>
          )}
        </p>
        <p className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-[13px] text-muted-foreground">
          <span>
            Harga asli terakhir{" "}
            <span className="num">{fmtRp(item.hargaTerakhir)}</span>
            {satuan} ({tanggalBisnis(item.tanggalTerakhir)})
          </span>
          {item.model.adalahBaseline && (
            <StatusBadge tone="netral" className="h-auto min-h-5 py-0.5 text-[11px] whitespace-normal">
              <Istilah kunci="persistensi">{LABEL_PERSISTENSI}</Istilah>
            </StatusBadge>
          )}
        </p>
      </div>
      <GrafikForecastMl
        item={item}
        className={cn(
          "h-56 transition-opacity lg:h-auto lg:min-h-0 lg:flex-1",
          memuatUlang && "opacity-60",
        )}
      />
      <TabelHorizonMl
        item={item}
        className={cn("shrink-0", memuatUlang && "opacity-60")}
      />
      {kaki}
    </PanelPas>
  );
}

/** Kontribusi faktor XAI dengan satuannya. Format lama tanpa `satuan_kontribusi` diberi tanda, bukan ditebak. */
export function DaftarFaktorMl({
  penjelasan,
  kunci,
}: {
  penjelasan: PenjelasanMl;
  kunci: string;
}) {
  const satuan = labelSatuanKontribusi(penjelasan.satuanKontribusi);
  const maks = Math.max(
    ...penjelasan.faktor.map((f) => Math.abs(f.kontribusi ?? 0)),
    1e-9,
  );
  if (penjelasan.faktor.length === 0) {
    return (
      <p className="text-xs text-muted-foreground">
        Tidak ada faktor yang dijelaskan: baseline persistensi tidak memakai
        fitur.
      </p>
    );
  }
  return (
    <div>
      <p className="text-[13px] font-medium">
        <Istilah kunci="kontribusiFaktor">Kontribusi faktor</Istilah>{" "}
        <span className="text-xs font-normal text-muted-foreground">
          ({satuan})
        </span>
      </p>
      <ul aria-label="Faktor penjelas" className="mt-1.5 space-y-2.5">
        {penjelasan.faktor.map((f, i) => {
          const v = f.kontribusi ?? 0;
          const pos = v >= 0;
          return (
            <li key={`${f.kelompok}-${i}`}>
              <div className="flex items-baseline justify-between gap-2 text-[13px]">
                <span className="min-w-0 truncate" title={f.label}>
                  {f.label}
                </span>
                <span
                  className={cn(
                    "num shrink-0 text-xs font-semibold",
                    pos
                      ? "text-defisit-ink dark:text-rose-300"
                      : "text-surplus-ink dark:text-blue-300",
                  )}
                >
                  {f.kontribusi === null
                    ? "–"
                    : `${pos ? "+" : "−"}${formatAngka(Math.abs(v), 2)}`}
                </span>
              </div>
              <div className="relative mt-1 h-1.5 rounded-full bg-mist-100 dark:bg-white/5">
                <div className="absolute inset-y-0 left-1/2 w-px bg-navy-900/15 dark:bg-white/20" />
                <motion.div
                  key={`${kunci}-${f.kelompok}`}
                  className={cn(
                    "absolute inset-y-0 rounded-full",
                    pos ? "left-1/2 bg-defisit" : "right-1/2 bg-surplus",
                  )}
                  initial={{ width: 0 }}
                  animate={{ width: `${(Math.abs(v) / maks) * 50}%` }}
                  transition={{
                    duration: 0.7,
                    ease: ease.garda,
                    delay: i * 0.06,
                  }}
                />
              </div>
              {f.fitur.length > 0 && (
                <p
                  className="mt-1 truncate text-xs text-muted-foreground"
                  title={f.fitur.map((ft) => ft.nama).join(", ")}
                >
                  {f.fitur
                    .slice(0, 3)
                    .map((ft) => ft.nama.replaceAll("_", " "))
                    .join(", ")}
                </p>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}

function Metrik({
  label,
  children,
  arti,
}: {
  label: string;
  children: ReactNode;
  arti?: "mape" | "rmse";
}) {
  return (
    <div className="min-w-0">
      <dt className="text-xs text-muted-foreground">
        {arti ? <Istilah kunci={arti}>{label}</Istilah> : label}
      </dt>
      <dd className="num mt-0.5 text-[13px] font-medium">{children}</dd>
    </div>
  );
}

const labelBanding = (v: boolean | null) =>
  v === null
    ? "Belum dievaluasi (tanpa holdout)"
    : v
      ? "Ya, lebih akurat"
      : "Tidak";

/** Model pemenang, metrik vs baseline, faktor XAI, dan narasi satu forecast. */
export function PanelPenjelasanMl({
  item,
  cycleId,
}: {
  item: ForecastDetailMl;
  cycleId: string | null;
}) {
  const m = item.metrik;
  return (
    <PanelPas
      judul="Model dan penjelasannya"
      deskripsi={
        cycleId ? `Hasil training siklus ${cycleId}` : "Hasil training"
      }
    >
      <div className="space-y-4">
        <div>
          <p className="text-xs text-muted-foreground">Model pemenang</p>
          <p className="mt-0.5 text-[13px] font-semibold">
            {item.model.adalahBaseline ? (
              <Istilah kunci="persistensi">
                {labelModelForecast(item.model)}
              </Istilah>
            ) : (
              labelModelForecast(item.model)
            )}
          </p>
          {item.model.alasan && (
            <p className="mt-0.5 text-xs leading-snug text-muted-foreground">
              {item.model.alasan}
            </p>
          )}
          {item.model.nKandidat !== null && (
            <p className="mt-0.5 text-xs text-muted-foreground">
              {formatAngka(item.model.nKandidat)} kandidat diuji walk-forward
              {m.nLipatanSeleksi !== null
                ? `, ${m.nLipatanSeleksi} lipatan seleksi`
                : ""}
            </p>
          )}
        </div>
        <dl className="grid grid-cols-2 gap-x-3 gap-y-2.5 border-t border-border pt-3">
          <Metrik label="MAPE model" arti="mape">
            {fmtPct(m.mape, false)}
          </Metrik>
          <Metrik
            label={`MAPE baseline${m.baseline ? ` (${namaModel(m.baseline).toLowerCase()})` : ""}`}
          >
            {fmtPct(m.mapeBaseline, false)}
          </Metrik>
          <Metrik label="RMSE" arti="rmse">
            {fmtRp(m.rmse)}
          </Metrik>
          <Metrik label="Mengalahkan baseline">
            {labelBanding(m.mengalahkanBaseline)}
          </Metrik>
        </dl>
        {m.evaluasi && (
          <p className="-mt-2 text-[11px] text-muted-foreground">
            Dievaluasi pada {m.evaluasi.replaceAll("_", " ")}, bukan jaminan
            akurasi ke depan.
          </p>
        )}
        <div className="border-t border-border pt-3">
          {item.penjelasan ? (
            <div className="space-y-3">
              <DaftarFaktorMl
                penjelasan={item.penjelasan}
                kunci={`${item.komoditasKode}-${item.kabupatenKode}`}
              />
              {item.penjelasan.narasi && (
                <p className="rounded-lg bg-data-50/70 p-2.5 text-[13px] leading-snug text-data-900/85 dark:bg-data-500/10 dark:text-data-100/85">
                  <BrainCircuit
                    className="mr-1 inline size-3.5 align-[-2px]"
                    aria-hidden
                  />
                  {narasiJujur(item.penjelasan.narasi, item.model)}
                </p>
              )}
              <p className="text-[11px] text-muted-foreground">
                Metode:{" "}
                {item.penjelasan.metode === "shap" ? "SHAP" : "dekomposisi"}
              </p>
            </div>
          ) : (
            <p className="text-xs text-muted-foreground">
              Penjelasan XAI tidak diterbitkan untuk forecast ini.
            </p>
          )}
        </div>
      </div>
    </PanelPas>
  );
}

/** Metrik model siklus (blok `model`) dan perbandingan per komoditas di satu kabupaten. */
export function PanelModelSiklusMl({
  model,
  items,
  wilayah,
}: {
  model: BlokModelMl;
  items: ForecastMl[];
  wilayah: string;
}) {
  const k = model.klasifikasi;
  const r = model.regresi;
  const pemenang = ringkasPemenang(r);
  return (
    <div className="flex min-w-0 flex-col gap-3 lg:min-h-0 lg:flex-1 lg:flex-row">
      <PanelPas
        judul="Metrik siklus"
        deskripsi={model.cycleId ? `Siklus ${model.cycleId}` : undefined}
        className="lg:w-[22rem] lg:shrink-0"
      >
        <div className="space-y-4">
          <section aria-label="Klasifikasi">
            <p className="text-xs text-muted-foreground">Klasifikasi arah</p>
            <p className="mt-0.5 text-[13px] font-semibold">
              <LabelTargetMl target={model.targetInflasi} />
            </p>
            {k && !dapatDiandalkan(k.kelayakan) ? (
              <CatatanBelumAndal
                judul={LABEL_BELUM_ANDAL}
                alasan={alasanBelumAndal(k.kelayakan, k.alasan)}
                className="mt-2"
              />
            ) : k ? (
              <StatusBadge tone="normal" className="mt-2">
                Layak dipakai
              </StatusBadge>
            ) : null}
            {k && (
              <dl className="mt-2 grid grid-cols-2 gap-x-3 gap-y-2">
                <Metrik label="ROC-AUC uji">
                  {k.rocAuc === null ? "–" : formatAngka(k.rocAuc, 3)}
                </Metrik>
                <Metrik label="Brier vs dasar">
                  {k.brier === null ? "–" : formatAngka(k.brier, 3)} /{" "}
                  {k.brierBaseline === null
                    ? "tidak dicatat"
                    : formatAngka(k.brierBaseline, 3)}
                </Metrik>
                <Metrik label="Model">{namaModel(k.model)}</Metrik>
                <Metrik label="Data uji">
                  {k.nUji === null ? "–" : `${formatAngka(k.nUji)} titik`}
                </Metrik>
              </dl>
            )}
          </section>
          <section
            aria-label="Regresi harga"
            className="border-t border-border pt-3"
          >
            <p className="text-xs text-muted-foreground">Proyeksi harga</p>
            <p className="mt-0.5 text-[13px] font-semibold">
              {pemenang?.label ?? "Siklus ini tanpa forecast"}
            </p>
            {r && (
              <>
                <dl className="mt-2 grid grid-cols-2 gap-x-3 gap-y-2">
                  <Metrik label="MAPE median" arti="mape">
                    {fmtPct(r.mapeMedian, false)}
                  </Metrik>
                  <Metrik label="MAPE baseline">
                    {fmtPct(r.mapeBaselineMedian, false)}
                  </Metrik>
                  <Metrik label="RMSE median" arti="rmse">
                    {fmtRp(r.rmseMedian)}
                  </Metrik>
                  <Metrik label="Dievaluasi holdout">
                    {r.nDievaluasiHoldout === null
                      ? "–"
                      : `${r.nDievaluasiHoldout} seri`}
                  </Metrik>
                </dl>
                <ul className="mt-2 space-y-0.5 text-xs text-muted-foreground">
                  {Object.entries(r.pemenang)
                    .filter(([, n]) => n > 0)
                    .sort((a, b) => b[1] - a[1])
                    .map(([nama, n]) => (
                      <li key={nama} className="flex justify-between gap-2">
                        <span>{namaModel(nama)}</span>
                        <span className="num">{n} seri</span>
                      </li>
                    ))}
                </ul>
              </>
            )}
          </section>
        </div>
      </PanelPas>
      <PanelPas
        judul="Model per komoditas"
        deskripsi={`${wilayah}, pemenang walk-forward tiap seri`}
        tanpaPadding
        className="lg:min-w-0 lg:flex-1"
        isiClassName="overflow-x-auto"
      >
        {items.length === 0 ? (
          <p className="p-3.5 text-xs text-muted-foreground">
            Siklus ini tidak punya forecast untuk wilayah ini.
          </p>
        ) : (
          <table className="w-full min-w-[560px] border-separate border-spacing-0 text-[13px]">
            <thead>
              <tr className="text-left text-xs text-muted-foreground">
                {[
                  "Komoditas",
                  "Model pemenang",
                  "MAPE",
                  "MAPE baseline",
                  "RMSE",
                  "Lebih akurat dari baseline",
                ].map((h, i) => (
                  <th
                    key={h}
                    className={cn(
                      "sticky top-0 border-b border-border bg-secondary px-3 py-2 font-medium",
                      i >= 2 && "text-right",
                    )}
                  >
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {items.map((f) => (
                <tr key={f.komoditasKode} className="hover:bg-secondary/60">
                  <td className="border-b border-border px-3 py-2 font-medium">
                    {namaKomoditasMl(f.komoditasKode)}
                  </td>
                  <td className="border-b border-border px-3 py-2 text-xs">
                    {labelModelForecast(f.model)}
                  </td>
                  <td className="num border-b border-border px-3 py-2 text-right">
                    {fmtPct(f.metrik.mape, false)}
                  </td>
                  <td className="num border-b border-border px-3 py-2 text-right text-muted-foreground">
                    {fmtPct(f.metrik.mapeBaseline, false)}
                  </td>
                  <td className="num border-b border-border px-3 py-2 text-right text-muted-foreground">
                    {fmtRp(f.metrik.rmse)}
                  </td>
                  <td className="border-b border-border px-3 py-2 text-right text-xs">
                    {labelBanding(f.metrik.mengalahkanBaseline)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </PanelPas>
    </div>
  );
}
