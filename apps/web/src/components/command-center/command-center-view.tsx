"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useSession } from "@/lib/auth/session-context";
import { softReveal, useGSAP } from "@/lib/motion/gsap";
import {
  useAktivitasLapangan,
  useColdStorage,
  useCuacaTerbaru,
  useEwsList,
  useGeometriWilayah,
  useHeatmap,
  useKpiOperasional,
  useKritisBaru,
  useLogistikOperasional,
  useMarkerWilayah,
  useMasterPasar,
  useRingkasanEksekutif,
  useSiklusAktif,
  useStatusMl,
  useStokOperasional,
  useTitikDesa,
  useTrenHarga,
} from "@/lib/query/hooks";
import { ZOOM_GEOMETRI } from "@/lib/api/spasial";
import { can } from "@/lib/rbac/permissions";
import { useEwsStore } from "@/lib/stores/ews-store";
import { cn } from "@/lib/utils";
import { CommandHeaderBar } from "./header-bar";
import { KpiPulseStrip } from "./kpi-pulse-strip";
import { LiveFeedPanel } from "./live-feed-panel";
import { PetaSentralPanel } from "./peta-sentral-panel";
import { RadarWilayahPanel } from "./radar-wilayah-panel";

const PARAM_KOSONG = {};

export function CommandCenterView() {
  const sesi = useSession();
  const qc = useQueryClient();
  const rootRef = useRef<HTMLDivElement>(null);

  const [selectedKab, setSelectedKab] = useState<string | undefined>(undefined);
  const [komoditasKode, setKomoditasKode] = useState("CBM-KRT");
  const [intervalMs, setIntervalMs] = useState<number | false>(15_000);
  const [isFullscreen, setIsFullscreen] = useState(false);

  const streamStatus = useEwsStore((s) => s.status);
  const newAlertIds = useEwsStore((s) => s.baru);

  // Guard izin secara ketat agar peran terbatas tidak memicu 403.
  const bolehOperasional = can(sesi, "dashboard.operasional");
  const bolehEksekutif = can(sesi, "dashboard.eksekutif");
  const bolehEws = can(sesi, "ews.lihat");
  const bolehCold = can(sesi, "coldstorage.lihat");
  const bolehMaster = can(sesi, "master.lihat");
  const bolehPrediksi = can(sesi, "prediksi.lihat");
  const bolehSpasial = can(sesi, "spasial.lihat");

  const pollOpsi = useMemo(
    () => ({
      refetchInterval: intervalMs,
      refetchIntervalInBackground: isFullscreen,
    }),
    [intervalMs, isFullscreen],
  );

  const siklus = useSiklusAktif(bolehOperasional, {
    ...pollOpsi,
    enabled: bolehOperasional,
  });
  const eksekutif = useRingkasanEksekutif({
    ...pollOpsi,
    enabled: bolehEksekutif,
  });
  const kpi = useKpiOperasional(undefined, {
    ...pollOpsi,
    enabled: bolehOperasional,
  });
  const heatmap = useHeatmap(undefined, {
    ...pollOpsi,
    enabled: bolehOperasional,
  });
  const aktivitas = useAktivitasLapangan({
    ...pollOpsi,
    enabled: bolehOperasional,
  });
  const stok = useStokOperasional(undefined, {
    ...pollOpsi,
    enabled: bolehOperasional,
  });
  const logistik = useLogistikOperasional(undefined, {
    ...pollOpsi,
    enabled: bolehOperasional,
  });
  const cuaca = useCuacaTerbaru({
    ...pollOpsi,
    enabled: bolehOperasional,
  });
  const markers = useMarkerWilayah({
    ...pollOpsi,
    enabled: bolehOperasional || bolehSpasial,
  });
  const geometri = useGeometriWilayah(ZOOM_GEOMETRI);
  const tren = useTrenHarga(komoditasKode, selectedKab ?? "1171", {
    ...pollOpsi,
    enabled: bolehOperasional,
  });
  const ewsList = useEwsList({
    ...pollOpsi,
    enabled: bolehEws,
  });
  const kritisBaru = useKritisBaru(bolehEws);
  const pasar = useMasterPasar(bolehMaster);
  const cold = useColdStorage(PARAM_KOSONG, bolehCold);
  const titikDesa = useTitikDesa(undefined, bolehSpasial);
  const statusMl = useStatusMl(bolehPrediksi);

  // Sinkronisasi state Fullscreen API + fallback pseudo-fullscreen jika browser menolak API.
  useEffect(() => {
    const onFsChange = () => {
      const aktif = Boolean(document.fullscreenElement);
      setIsFullscreen(aktif);
    };
    document.addEventListener("fullscreenchange", onFsChange);
    return () => document.removeEventListener("fullscreenchange", onFsChange);
  }, []);

  const toggleFullscreen = useCallback(async () => {
    const el = rootRef.current;
    if (!el) return;
    if (document.fullscreenElement || isFullscreen) {
      if (document.fullscreenElement && typeof document.exitFullscreen === "function") {
        try {
          await document.exitFullscreen();
          setIsFullscreen(false);
          return;
        } catch {
          // Fallback ke pseudo-fullscreen state
        }
      }
      setIsFullscreen(false);
      return;
    }

    if (typeof el.requestFullscreen === "function") {
      try {
        await el.requestFullscreen();
        setIsFullscreen(true);
        return;
      } catch {
        // Fallback jika environment tidak mengizinkan native Fullscreen
      }
    }
    setIsFullscreen(true);
  }, [isFullscreen]);

  // Shortcut keyboard 'F' untuk toggle Fullscreen dan 'Esc' keluar pseudo-fullscreen.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement | null)?.tagName;
      if (
        tag === "INPUT" ||
        tag === "TEXTAREA" ||
        tag === "SELECT" ||
        (e.target as HTMLElement | null)?.isContentEditable
      ) {
        return;
      }
      if (!e.metaKey && !e.ctrlKey && !e.altKey && e.key.toLowerCase() === "f") {
        e.preventDefault();
        void toggleFullscreen();
      } else if (e.key === "Escape" && isFullscreen && !document.fullscreenElement) {
        setIsFullscreen(false);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [toggleFullscreen, isFullscreen]);

  // Animasi GSAP sekali saat mount, tidak pernah diulang saat data refresh.
  useGSAP(
    () => {
      const items = rootRef.current?.querySelectorAll(".cc-bento");
      if (items?.length) {
        softReveal(items, { y: 10, duration: 0.42, stagger: 0.04 });
      }
    },
    { scope: rootRef },
  );

  const handleManualRefresh = useCallback(() => {
    void qc.invalidateQueries();
  }, [qc]);

  const lastUpdatedAt = Math.max(
    kpi.dataUpdatedAt || 0,
    eksekutif.dataUpdatedAt || 0,
    ewsList.dataUpdatedAt || 0,
    markers.dataUpdatedAt || 0,
  );

  const isFetching =
    kpi.isFetching ||
    eksekutif.isFetching ||
    ewsList.isFetching ||
    markers.isFetching;

  const backHref = bolehEksekutif
    ? "/eksekutif"
    : bolehOperasional
      ? "/operasional"
      : "/ews";

  return (
    <div
      ref={rootRef}
      data-testid="command-center-root"
      data-fullscreen={isFullscreen ? "true" : "false"}
      className={cn(
        "flex w-full min-w-0 flex-col gap-2.5",
        isFullscreen
          ? "fixed inset-0 z-50 h-dvh w-screen overflow-hidden bg-background p-3 sm:p-4"
          : "lg:h-full lg:min-h-0",
      )}
    >
      <CommandHeaderBar
        siklus={siklus.data}
        streamStatus={streamStatus}
        intervalMs={intervalMs}
        onIntervalChange={setIntervalMs}
        lastUpdatedAt={lastUpdatedAt}
        isFetching={isFetching}
        onManualRefresh={handleManualRefresh}
        isFullscreen={isFullscreen}
        onToggleFullscreen={() => void toggleFullscreen()}
        backHref={backHref}
      />

      <KpiPulseStrip
        eksekutif={eksekutif.data}
        kpi={kpi.data}
        alerts={ewsList.data}
        aktivitas={aktivitas.data}
        stok={stok.data}
        bolehEksekutif={bolehEksekutif}
        bolehOperasional={bolehOperasional}
        bolehEws={bolehEws}
        bolehCold={bolehCold}
      />

      {/* Panggung Utama: 3 Zona Bento (Kiri 30% / Tengah 42% / Kanan 28%) */}
      <div className="grid min-h-0 flex-1 grid-cols-1 gap-2.5 lg:grid-cols-[minmax(0,30fr)_minmax(0,42fr)_minmax(0,28fr)]">
        <RadarWilayahPanel
          markers={markers.data}
          heatmap={heatmap.data}
          cuaca={cuaca.data}
          selectedKab={selectedKab}
          onSelectKab={setSelectedKab}
        />

        <PetaSentralPanel
          markers={markers.data}
          geometri={geometri.data}
          pasar={pasar.data}
          coldFasilitas={cold.data?.fasilitas}
          titikDesa={titikDesa.data}
          isLoadingMap={markers.isLoading}
          selectedKab={selectedKab}
          onSelectKab={setSelectedKab}
          komoditasKode={komoditasKode}
          onKomoditasChange={setKomoditasKode}
          trenData={tren.data}
          trenLoading={tren.isLoading}
        />

        <LiveFeedPanel
          alerts={ewsList.data}
          kritisBaru={kritisBaru.data}
          newAlertIds={newAlertIds}
          logistik={logistik.data}
          stok={stok.data}
          statusMl={statusMl.data}
          bolehEws={bolehEws}
        />
      </div>
    </div>
  );
}
