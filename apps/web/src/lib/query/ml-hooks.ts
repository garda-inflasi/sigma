"use client";

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback } from "react";
import { bacaLampiranMl, cycleIdSah, daftarSiklusMl } from "@/lib/api/ml";
import { useCan } from "@/lib/auth/session-context";
import { useUi, type LingkunganData } from "@/lib/stores/ui-store";

/** Query key hasil training; dipisah dari `qk` bersama supaya modul ini berdiri sendiri. */
export const qkMl = {
  all: () => ["ml"] as const,
  siklus: (env: LingkunganData) => ["ml", "siklus", env] as const,
  lampiran: (path: string, cycleId: string | undefined, env: LingkunganData) =>
    ["ml", "lampiran", path, cycleId ?? "terbaru", env] as const,
};

/**
 * Hasil training hanya dibaca di mode Prod (Real). Mode Staging sengaja memakai data demo PostgreSQL saja (API
 * menjawab lampiran `MODE_STAGING`), jadi bagian ML tidak meminta apa pun di sana.
 */
export function useMlAktif(): boolean {
  return useUi((s) => s.lingkunganData) === "prod";
}

/**
 * Siklus training pilihan di URL (`?cycle_id=`). Kosong berarti siklus final terbaru; memilih yang terbaru menghapus
 * parameternya. Pemakainya harus di dalam `<Suspense>` karena memakai `useSearchParams`.
 */
export function useSiklusMlTerpilih(): readonly [
  string | undefined,
  (cycleId?: string) => void,
] {
  const params = useSearchParams();
  const pathname = usePathname();
  const router = useRouter();
  const aktif = cycleIdSah(params.get("cycle_id"));
  const ubah = useCallback(
    (cycleId?: string) => {
      const q = new URLSearchParams(params.toString());
      const id = cycleIdSah(cycleId);
      if (id) q.set("cycle_id", id);
      else q.delete("cycle_id");
      const s = q.toString();
      router.replace(s ? `${pathname}?${s}` : pathname, { scroll: false });
    },
    [params, pathname, router],
  );
  return [aktif, ubah] as const;
}

/** Daftar siklus training (`GET /v1/ml/cycles`, izin `prediksi.lihat`); tidak meminta apa pun tanpa izin atau di Staging. */
export function useDaftarSiklusMl(enabled = true) {
  const env = useUi((s) => s.lingkunganData);
  const boleh = useCan("prediksi.lihat");
  return useQuery({
    queryKey: qkMl.siklus(env),
    queryFn: () => daftarSiklusMl(24),
    enabled: enabled && env === "prod" && boleh,
    staleTime: 60_000,
    refetchInterval: 5 * 60_000,
    retry: false,
  });
}

/**
 * Lampiran `ml` satu GET endpoint lama untuk siklus pilihan. Halaman tetap memuat `data`-nya lewat hook lama;
 * hook ini hanya menambah blok hasil training. Data siklus sebelumnya dipertahankan selama siklus baru dimuat
 * (`isPlaceholderData`) supaya panel tidak berkedip saat berganti siklus.
 */
export function useLampiranMl(
  path: string,
  opsi: { cycleId?: string; enabled?: boolean } = {},
) {
  const env = useUi((s) => s.lingkunganData);
  const cycleId = cycleIdSah(opsi.cycleId);
  return useQuery({
    queryKey: qkMl.lampiran(path, cycleId, env),
    queryFn: () => bacaLampiranMl(path, cycleId),
    enabled: (opsi.enabled ?? true) && env === "prod",
    staleTime: 60_000,
    refetchInterval: 5 * 60_000,
    placeholderData: keepPreviousData,
    retry: 1,
  });
}
