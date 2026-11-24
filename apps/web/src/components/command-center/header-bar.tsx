"use client";

import { memo, useEffect, useState } from "react";
import Link from "next/link";
import {
  ArrowLeft,
  Maximize2,
  Minimize2,
  Moon,
  Pause,
  Play,
  Radio,
  RefreshCw,
  Sun,
} from "lucide-react";
import { useTheme } from "next-themes";
import { PulseRing } from "@/components/motion/pulse-ring";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { SiklusAktif } from "@/lib/api/siklus";
import { siklusMeta, toneClass } from "@/lib/design/tokens";
import type { StreamStatus } from "@/lib/stores/ews-store";
import { cn } from "@/lib/utils";

/** Jam WIB terisolasi agar tick tiap detik tidak me-render ulang panel peta atau grafik. */
const LiveClock = memo(function LiveClock() {
  const [now, setNow] = useState<Date | null>(null);

  useEffect(() => {
    setNow(new Date());
    const id = window.setInterval(() => setNow(new Date()), 1000);
    return () => window.clearInterval(id);
  }, []);

  const teksWaktu = now
    ? new Intl.DateTimeFormat("id-ID", {
        timeZone: "Asia/Jakarta",
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
        hour12: false,
      }).format(now)
    : "--:--:--";

  const teksTanggal = now
    ? new Intl.DateTimeFormat("id-ID", {
        timeZone: "Asia/Jakarta",
        weekday: "short",
        day: "numeric",
        month: "short",
        year: "numeric",
      }).format(now)
    : "";

  return (
    <div className="hidden items-center gap-2 rounded-md border border-border bg-card px-2.5 py-1 text-xs sm:flex">
      <span className="num font-semibold text-foreground">{teksWaktu} WIB</span>
      {teksTanggal && (
        <span className="hidden text-muted-foreground xl:inline">
          · {teksTanggal}
        </span>
      )}
    </div>
  );
});

const FreshnessTimer = memo(function FreshnessTimer({
  lastUpdatedAt,
  isFetching,
}: {
  lastUpdatedAt: number;
  isFetching: boolean;
}) {
  const [detik, setDetik] = useState(0);

  useEffect(() => {
    if (!lastUpdatedAt) return;
    const hitung = () =>
      setDetik(Math.max(0, Math.floor((Date.now() - lastUpdatedAt) / 1000)));
    hitung();
    const id = window.setInterval(hitung, 1000);
    return () => window.clearInterval(id);
  }, [lastUpdatedAt]);

  return (
    <span
      className="num hidden text-[11px] text-muted-foreground lg:inline"
      aria-live="polite"
    >
      {isFetching
        ? "Memperbarui…"
        : !lastUpdatedAt || detik < 5
          ? "Baru saja diperbarui"
          : `Diperbarui ${detik} dtk lalu`}
    </span>
  );
});

export interface CommandHeaderBarProps {
  siklus?: SiklusAktif;
  streamStatus: StreamStatus;
  intervalMs: number | false;
  onIntervalChange: (val: number | false) => void;
  lastUpdatedAt: number;
  isFetching: boolean;
  onManualRefresh: () => void;
  isFullscreen: boolean;
  onToggleFullscreen: () => void;
  backHref: string;
}

export const CommandHeaderBar = memo(function CommandHeaderBar({
  siklus,
  streamStatus,
  intervalMs,
  onIntervalChange,
  lastUpdatedAt,
  isFetching,
  onManualRefresh,
  isFullscreen,
  onToggleFullscreen,
  backHref,
}: CommandHeaderBarProps) {
  const { resolvedTheme, setTheme } = useTheme();
  const meta = siklus ? siklusMeta[siklus.siklus] : siklusMeta.normal;
  const isPaused = intervalMs === false;
  const gelap = resolvedTheme === "dark";

  const statusLiveLabel =
    streamStatus === "terhubung"
      ? "SSE Live"
      : streamStatus === "simulasi"
        ? "Mode Simulasi"
        : streamStatus === "menghubungkan"
          ? "Menghubungkan"
          : "Polling Cadangan";

  return (
    <header className="glass shrink-0 rounded-xl px-3 py-2 sm:px-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2.5">
          {!isFullscreen && (
            <Link
              href={backHref}
              className="inline-flex h-8 shrink-0 items-center gap-1.5 rounded-md border border-border bg-card px-2.5 text-xs font-medium text-foreground transition-colors hover:bg-secondary"
            >
              <ArrowLeft className="size-3.5" />
              <span className="hidden sm:inline">Konsol</span>
            </Link>
          )}
          <div className="flex min-w-0 items-center gap-2">
            <span className="grid size-8 shrink-0 place-items-center rounded-lg bg-navy-800 text-white dark:bg-white dark:text-navy-900">
              <Radio className="size-4" />
            </span>
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <h1 className="truncate text-sm font-semibold tracking-tight text-foreground sm:text-base">
                  Command Center Aceh
                </h1>
                <span
                  className={cn(
                    "inline-flex h-5 shrink-0 items-center gap-1.5 rounded-full px-2 text-[11px] font-medium",
                    toneClass[meta.tone].soft,
                    toneClass[meta.tone].text,
                  )}
                >
                  <PulseRing
                    tone={meta.tone}
                    rings={1}
                    active={siklus?.siklus === "tekanan"}
                  />
                  {meta.label}
                </span>
              </div>
              <p className="hidden truncate text-[11px] text-muted-foreground md:block">
                Pusat kendali stabilitas harga pangan &amp; peringatan dini real-time
              </p>
            </div>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-1.5 sm:gap-2">
          <span
            className={cn(
              "inline-flex h-7 items-center gap-1.5 rounded-md border px-2 text-[11px] font-medium",
              streamStatus === "terhubung" || streamStatus === "simulasi"
                ? "border-emerald-200 bg-emerald-50/70 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300"
                : "border-border bg-card text-muted-foreground",
            )}
          >
            <span
              aria-hidden
              className={cn(
                "size-1.5 rounded-full",
                streamStatus === "terputus"
                  ? "bg-amber-500"
                  : "animate-live bg-emerald-500",
              )}
            />
            {statusLiveLabel}
          </span>

          <FreshnessTimer
            lastUpdatedAt={lastUpdatedAt}
            isFetching={isFetching}
          />

          <LiveClock />

          <div className="flex items-center gap-1">
            <Select
              value={isPaused ? "off" : String(intervalMs)}
              onValueChange={(v) =>
                onIntervalChange(v === "off" ? false : Number(v))
              }
            >
              <SelectTrigger
                className="h-8 w-24 text-xs"
                aria-label="Interval pembaruan otomatis"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="15000">15 detik</SelectItem>
                <SelectItem value="30000">30 detik</SelectItem>
                <SelectItem value="60000">60 detik</SelectItem>
                <SelectItem value="off">Jeda</SelectItem>
              </SelectContent>
            </Select>

            <Button
              type="button"
              size="sm"
              variant="outline"
              className="h-8 w-8 p-0"
              onClick={() => onIntervalChange(isPaused ? 15000 : false)}
              aria-label={isPaused ? "Lanjutkan auto-refresh" : "Jeda auto-refresh"}
              title={isPaused ? "Lanjutkan auto-refresh" : "Jeda auto-refresh"}
            >
              {isPaused ? (
                <Play className="size-3.5" />
              ) : (
                <Pause className="size-3.5" />
              )}
            </Button>

            <Button
              type="button"
              size="sm"
              variant="outline"
              className="h-8 w-8 p-0"
              onClick={onManualRefresh}
              aria-label="Segarkan data sekarang"
              title="Segarkan data sekarang"
            >
              <RefreshCw
                className={cn("size-3.5", isFetching && "animate-spin")}
              />
            </Button>

            <Button
              type="button"
              size="sm"
              variant="outline"
              className="h-8 w-8 p-0"
              onClick={() => setTheme(gelap ? "light" : "dark")}
              aria-label={gelap ? "Aktifkan mode terang" : "Aktifkan mode gelap"}
              title={gelap ? "Mode terang" : "Mode gelap"}
            >
              {gelap ? <Sun className="size-3.5" /> : <Moon className="size-3.5" />}
            </Button>

            <Button
              type="button"
              size="sm"
              variant={isFullscreen ? "default" : "outline"}
              className="h-8 gap-1.5 px-2.5 text-xs"
              onClick={onToggleFullscreen}
              aria-pressed={isFullscreen}
              aria-label={isFullscreen ? "Keluar Fullscreen" : "Mode Fullscreen"}
              title="Tekan F untuk toggle Fullscreen"
            >
              {isFullscreen ? (
                <>
                  <Minimize2 className="size-3.5" />
                  <span className="hidden sm:inline">Keluar Fullscreen</span>
                </>
              ) : (
                <>
                  <Maximize2 className="size-3.5" />
                  <span className="hidden sm:inline">Fullscreen</span>
                </>
              )}
            </Button>
          </div>
        </div>
      </div>
    </header>
  );
});
