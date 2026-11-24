"use client";

import { motion } from "motion/react";
import {
  BellRing,
  CheckCheck,
  ChevronRight,
  CloudDownload,
  MapPin,
  Play,
} from "lucide-react";
import { useMemo, useState } from "react";
import { toast } from "sonner";
import { unduhAcuan } from "@/lib/offline/acuan-server";
import { Checkbox } from "@/components/ui/checkbox";
import {
  BulkActionBar,
  type BulkAction,
} from "@/components/data-table/bulk-action-bar";
import { EmptyState } from "@/components/garda/empty-state";
import { PanelGalat } from "@/components/garda/panel-galat";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import { SpringLink } from "@/components/garda/spring-button";
import { CountUp } from "@/components/motion/count-up";
import { komoditasByKode } from "@/lib/data/reference";
import type { TugasLapangan } from "@/lib/data/types";
import { formatRelatif } from "@/lib/format";
import { ease, fadeUp, stagger } from "@/lib/motion/presets";
import { useBulkAction, useTugasSaya } from "@/lib/query/hooks";
import { qk } from "@/lib/query/keys";
import { cn } from "@/lib/utils";
import { hrefIsiTugas } from "@/lib/api/lapangan";
import { TugasDetailSheet } from "./tugas-detail-sheet";

const STATUS = {
  BELUM: { label: "Belum mulai", tone: "netral" },
  BERJALAN: { label: "Berjalan", tone: "data" },
  SELESAI: { label: "Selesai", tone: "normal" },
  TERLAMBAT: { label: "Terlambat", tone: "defisit" },
} as const;

const JENDELA_MENDESAK_MS = 3 * 3_600_000;

const peringkat = (t: TugasLapangan) =>
  t.status === "TERLAMBAT" ? 0 : t.status === "SELESAI" ? 2 : 1;

function Cincin({ pct }: { pct: number }) {
  const r = 34;
  const c = 2 * Math.PI * r;
  return (
    <svg viewBox="0 0 80 80" className="size-16 -rotate-90" aria-hidden>
      <circle
        cx="40"
        cy="40"
        r={r}
        fill="none"
        strokeWidth="8"
        className="stroke-mist-200 dark:stroke-white/10"
      />
      <motion.circle
        cx="40"
        cy="40"
        r={r}
        fill="none"
        strokeWidth="8"
        strokeLinecap="round"
        className="stroke-data-500"
        strokeDasharray={c}
        initial={{ strokeDashoffset: c }}
        animate={{ strokeDashoffset: c * (1 - pct / 100) }}
        transition={{ duration: 1.1, ease: ease.garda }}
      />
    </svg>
  );
}

/** Daftar tugas survei: yang paling mendesak di atas, progres, tenggat, dan mulai cepat ke form input. */
export function TugasList() {
  const { data, isLoading, isError, error, refetch } = useTugasSaya();
  const tugas = useMemo(() => data ?? [], [data]);
  const [sel, setSel] = useState<string[]>([]);
  const [detailId, setDetailId] = useState<string | null>(null);
  const tandaiSelesai = useBulkAction<TugasLapangan>({
    sumber: "lapangan/tugas",
    queryKey: qk.tugas.saya(),
    invalidate: qk.tugas.saya(),
    patch: (t) => ({ ...t, status: "SELESAI" }),
  });

  const aktif = tugas.filter((t) => t.status !== "SELESAI");
  const terlambat = tugas.filter((t) => t.status === "TERLAMBAT").length;
  const target = tugas.reduce((s, t) => s + t.target, 0);
  const terisi = tugas.reduce((s, t) => s + Math.min(t.terisi, t.target), 0);
  const pct = target ? (terisi / target) * 100 : 0;
  const urut = [...tugas].sort(
    (a, b) =>
      peringkat(a) - peringkat(b) ||
      +new Date(a.tenggat) - +new Date(b.tenggat),
  );
  const utama = urut[0]?.status === "SELESAI" ? undefined : urut[0];
  const sisaMs = utama
    ? new Date(utama.tenggat).getTime() - Date.now()
    : Infinity;
  const labelUtama =
    sisaMs < 0
      ? "Kerjakan dulu, tenggat sudah lewat"
      : sisaMs < JENDELA_MENDESAK_MS
        ? "Kerjakan dulu, tenggat sebentar lagi"
        : "Tugas berikutnya";

  const actions: BulkAction<TugasLapangan>[] = [
    {
      id: "unduh",
      label: "Unduh data acuan",
      icon: CloudDownload,
      tone: "data",
      run: async (r) => {
        await unduhAcuan();
        toast.success(`Data acuan ${r.length} tugas tersimpan offline`, {
          description: "Median harga dan geofence pasar.",
        });
      },
    },
    {
      id: "selesai",
      label: "Tandai selesai",
      icon: CheckCheck,
      enabled: (r) =>
        r.every((t) => t.terisi >= t.target && t.status !== "SELESAI"),
      run: async (r) => {
        await tandaiSelesai.mutateAsync({
          aksi: "tandai_selesai",
          ids: r.map((t) => t.id),
          label: "Tugas ditandai selesai",
        });
        setSel([]);
      },
    },
  ];

  if (isLoading)
    return (
      <div className="space-y-3">
        <Shimmer className="h-24 w-full rounded-xl" />
        {Array.from({ length: 3 }).map((_, i) => (
          <Shimmer key={i} className="h-36 w-full rounded-xl" />
        ))}
      </div>
    );

  if (isError && !data) {
    return (
      <div className="space-y-4">
        <h1 className="text-display-sm font-semibold">Tugas survei</h1>
        <PanelGalat
          judul="Daftar tugas gagal dimuat"
          error={error}
          onCoba={() => void refetch()}
        />
      </div>
    );
  }

  if (tugas.length === 0) {
    return (
      <div className="space-y-4">
        <h1 className="text-display-sm font-semibold">Tugas survei</h1>
        <EmptyState
          title="Belum ada tugas survei"
          description="Tugas dari koordinator akan muncul di sini. Anda tetap bisa mencatat harga."
          action={
            <SpringLink href="/lapangan/input" size="xl">
              Catat harga
            </SpringLink>
          }
          className="glass rounded-xl p-8"
        />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <h1 className="text-display-sm font-semibold">Tugas survei</h1>
      <section
        aria-label="Ringkasan tugas"
        className="glass flex items-center gap-4 rounded-xl p-4"
      >
        <div className="relative shrink-0">
          <Cincin pct={pct} />
          <span className="num absolute inset-0 grid place-items-center text-xs font-semibold">
            {Math.round(pct)}%
          </span>
        </div>
        <div className="min-w-0">
          <p className="text-xl font-semibold">
            <CountUp value={terisi} />{" "}
            <span className="num text-sm font-normal text-muted-foreground">
              dari {target} komoditas
            </span>
          </p>
          <p className="text-[13px] text-muted-foreground">
            <span className="num">{aktif.length}</span> tugas aktif
            {terlambat > 0 && (
              <>
                ,{" "}
                <span className="font-medium text-defisit-ink dark:text-rose-300">
                  <span className="num">{terlambat}</span> terlambat
                </span>
              </>
            )}
          </p>
        </div>
      </section>

      <motion.ul
        variants={stagger(0.06)}
        initial="hidden"
        animate="show"
        className="space-y-3"
      >
        {urut.map((t) => {
          const s = STATUS[t.status];
          const p = Math.min(100, (t.terisi / t.target) * 100);
          const checked = sel.includes(t.id);
          const diUtamakan = t.id === utama?.id;
          const nama = t.komoditas.map((k) => komoditasByKode[k]?.nama ?? k);
          const lanjut = t.status !== "SELESAI" && (
            <SpringLink
              href={hrefIsiTugas(t)}
              size={diUtamakan ? "xl" : "md"}
              className={diUtamakan ? "w-full" : "h-11 shrink-0"}
            >
              <Play /> {t.terisi ? "Lanjutkan" : "Mulai survei"}
            </SpringLink>
          );
          return (
            <motion.li
              key={t.id}
              variants={fadeUp}
              className={cn(
                "glass rounded-xl p-4 transition-shadow",
                diUtamakan && "ring-1 ring-waspada/50",
                checked && "ring-2 ring-data-400",
              )}
            >
              {diUtamakan && (
                <p
                  className={cn(
                    "mb-3 flex items-center gap-1.5 text-xs font-medium",
                    sisaMs < JENDELA_MENDESAK_MS
                      ? "text-waspada-ink dark:text-amber-200"
                      : "text-muted-foreground",
                  )}
                >
                  <BellRing className="size-3.5 shrink-0" aria-hidden />
                  {labelUtama}
                </p>
              )}
              <div className="flex items-start gap-3.5">
                <Checkbox
                  checked={checked}
                  onCheckedChange={() =>
                    setSel((x) =>
                      checked ? x.filter((i) => i !== t.id) : [...x, t.id],
                    )
                  }
                  aria-label={`Pilih ${t.judul}`}
                  className="relative mt-1 rounded-md after:absolute after:-inset-3.5 after:content-['']"
                />
                <div className="min-w-0 flex-1">
                  <div className="flex items-start justify-between gap-2">
                    <button
                      type="button"
                      onClick={() => setDetailId(t.id)}
                      aria-label={`Detail tugas ${t.judul}, ${t.pasarNama}, ${s.label}`}
                      className="-my-2.5 flex min-h-11 min-w-0 items-center gap-0.5 text-left font-medium underline-offset-4 hover:underline"
                    >
                      <span className="min-w-0">{t.judul}</span>
                      <ChevronRight
                        className="size-4 shrink-0 text-muted-foreground"
                        aria-hidden
                      />
                    </button>
                    <StatusBadge tone={s.tone} className="shrink-0">
                      {s.label}
                    </StatusBadge>
                  </div>
                  <p className="mt-1 flex items-center gap-1 text-xs text-muted-foreground">
                    <MapPin className="size-3.5 shrink-0" aria-hidden />
                    <span className="truncate">
                      {t.pasarNama}, harga{" "}
                      {t.tipe === "KONSUMEN" ? "konsumen" : "produsen"}
                    </span>
                  </p>
                  <p
                    className="mt-1 flex min-w-0 gap-1 text-xs text-muted-foreground"
                    title={nama.join(", ")}
                  >
                    <span className="truncate">
                      {nama.slice(0, 3).join(", ")}
                    </span>
                    {nama.length > 3 && (
                      <span className="num shrink-0">+{nama.length - 3}</span>
                    )}
                  </p>
                  <div className="mt-3 flex items-center gap-2">
                    <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-mist-200 dark:bg-white/10">
                      <motion.div
                        className={cn(
                          "h-full rounded-full",
                          t.status === "TERLAMBAT"
                            ? "bg-defisit"
                            : t.status === "SELESAI"
                              ? "bg-normal"
                              : "bg-data-500",
                        )}
                        initial={{ width: 0 }}
                        animate={{ width: `${p}%` }}
                        transition={{ duration: 0.8, ease: ease.garda }}
                      />
                    </div>
                    <span className="num text-xs text-muted-foreground">
                      {t.terisi}/{t.target}
                    </span>
                  </div>
                  <div className="mt-3 flex items-center justify-between gap-2">
                    <span
                      className={cn(
                        "text-xs",
                        t.status === "TERLAMBAT"
                          ? "text-defisit-ink dark:text-rose-300"
                          : "text-muted-foreground",
                      )}
                    >
                      Tenggat {formatRelatif(t.tenggat)}
                    </span>
                    {!diUtamakan && lanjut}
                  </div>
                </div>
              </div>
              {diUtamakan && lanjut && <div className="mt-3">{lanjut}</div>}
            </motion.li>
          );
        })}
      </motion.ul>
      <TugasDetailSheet
        id={detailId}
        daftar={tugas.find((t) => t.id === detailId)}
        onClose={() => setDetailId(null)}
      />
      <BulkActionBar
        rows={tugas.filter((t) => sel.includes(t.id))}
        actions={actions}
        entityLabel="tugas"
        onClear={() => setSel([])}
        placement="inline"
        className="fixed bottom-[calc(max(env(safe-area-inset-bottom),0.75rem)+5.25rem)] left-1/2 max-w-[calc(100vw-2rem)] -translate-x-1/2"
      />
    </div>
  );
}
